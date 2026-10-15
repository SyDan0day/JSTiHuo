# 14 物流、通知与文件集成

**DESIGN_DECISION**：CarrierProvider接口CreateLabel(ctx,requestKey,orderSnapshot)、GetLabel(ctx,requestKey)、Track(ctx,carrier,tracking)、VerifyEvent(rawBody,headers)。Notifier.Send(ctx,eventKey,template,recipient,data)、Lookup(ctx,eventKey)。没有供应商账号时MANUAL/MOCK完整可运行；LIVE适配器未配置返回503 INTEGRATION_DISABLED，不假成功。

## Outbox worker

订单事务写event_key唯一事件，仅对象ID/版本/模板。worker小事务FOR UPDATE SKIP LOCKED领取最多20条，state=PROCESSING、lease60秒；提交后调用外部；超时10秒（连接3秒），不持DB事务等待外网。成功短事务记DONE/notification SENT；每次调用计入 attempts，前四次确定失败分别退避 1m/5m/30m/2h，包含首次在内最多五次，第五次失败进入 DEAD，人工retry只能FAILED/DEAD且查权限。lease过期可再领，因此至少一次交付，不宣称Exactly Once。

支持供应商幂等键时沿用event_key；不支持且调用超时结果未知，记UNKNOWN并Lookup/人工核对，禁止盲目重发短信/面单。内部事件如图片清理可安全重复。通知关闭写SUPPRESSED且Outbox DONE保留审计；MOCK记录模拟结果不发外网，公开界面标“测试轨迹”。管理员通知没有配置收件对象则SUPPRESSED，不发送任意地址。

## 运单与轨迹

原发全单，禁止部分发货，carrier+tracking唯一且shipment_key=primary:{id}。tracking只允许ASCII字母数字连字符3..80；provider规则可更严格但不可静默截断。自动运单job创建和version绑定，运行时订单锁检查PENDING；READY单号仍需人工确认发货，确认必须与该job返回单号一致；取消/改址遇到RUNNING/READY/UNKNOWN job返回409，需供应商确认撤销后才能继续（v1不提供自动撤销，客服用人工流程标记已核对）。未配置供应商不得产生真实运单。

跟踪回调HMAC SHA256(timestamp+'.'+rawBody)、独立provider密钥；timestamp允许±5分钟，event ID持久去重，恒时比较；拒绝过大body>256KiB。发生时间只影响时间线，不让旧事件回退DELIVERED；description 不保存自由文本，按事件类型映射固定公开文案。调度拉取每6小时，签收后停止，30天未签收改为人工异常，不自动完成；管理员可有理由确认完成。

## 卡券导入

UTF-8 CSV：code,secret（不回显）,valid_from,valid_until；batch_id在请求给定，批次必须DRAFT且设置已有权益快照，最多1000行。预检不创建卡、不更改批次；错误只返回行号/字段/码，禁止secret。提交整体原子，规范卡号唯一、密码长度格式符合规则；导入产生hash并直接SEALED（外部发行方已持秘密），批次SEALED、active按activate_on_seal。不再生成/重新导出原秘密；原始导入文件AEAD加密临时保存，commit后销毁。生成/导入互斥，锁批次。重复文件SHA+kind+batch唯一job_key防重复。

## 运单导入

UTF-8 CSV：order_no,carrier_code,tracking_no,expected_version。预检全表不写业务，每行验证订单待发/版本、carrier、重复单号。确认commit按订单ID升序逐行调用T-SHIP，允许行级成功，job结果明确成功行/失败行及错误码；重试仅失败行，idempotency_key=jobID+rowIndex+rowHash。不同文件重复订单受主运单唯一键保护。无“整批成功”假提示。

## 报表/文件管理

普通导出只VOUCHER_METADATA、ORDERS，默认字段白名单：卡号掩码/批次/状态/有效期，订单号/状态/日期/脱敏收货摘要。order.pii权限+reauth才可选择PII扩展（首版不开放扩展参数，故仅脱敏）。worker流式CSV最多10万行、10MiB；超出明确失败需收窄筛选。避免Excel公式注入：以=+-@、tab/CR开头单元格前加单引号并RFC4180引号转义。下载校验owner或file.export管理权限，24小时过期。

供应商密钥不通过config API返回或明文录入；部署引用secret_ref，启动解析，后台仅可选择已配置引用白名单。微信/小程序/旺店通完整集成列V2，接口适配层预留不实现虚假同步。

## 人工核对与任务阶段的闭合契约

DESIGN_DECISION：`resolveJob`要求integration.manage、5分钟内复核、version、reason和供应商工单引用（不含客户PII）。只处理LABEL的UNKNOWN，或尚未确认发货的READY且供应商已确认作废。CONFIRMED_CREATED必须附carrier_id/tracking_no，经校验写结果并转READY；CONFIRMED_VOID/CONFIRMED_FAILED转FAILED，解除订单编辑/取消的外部任务锁。已经SHIPPED的订单禁止此操作；不能通过声明失败撤销已发货或删除运单。核对证据摘要记audit，供应商凭证不写入。若结果实际尚不确定，应保留UNKNOWN；无可信证据不能调用“核对完成”。核对与shipOrder并发锁同一订单，失败则全回滚。

`resolveNotification`只处理UNKNOWN；CONFIRMED_SENT→SENT并结束Outbox；CONFIRMED_NOT_SENT→FAILED，才可调用retryNotification。两者均需reauth、version及核对引用。Outbox已完成不可再重发，人工判断错误的风险计入审计；生产供应商接入须提供Lookup能力或明确人工工单流程。

Job.phase存integration_jobs.options.phase，API枚举PREVIEW/APPLY。导入创建为PREVIEW/PENDING，预检完成为PREVIEW/READY；commitImport检查原文件hash、对象权限、预检零致命错误，原子转APPLY/PENDING并入Outbox，再次提交返回同job。执行完为APPLY/READY（运单部分成功用failed计数和结果文件明确展示），整体失败为FAILED。导入完整校验不能仅依赖旧预检结果。LABEL/TRACKING/EXPORT_REPORT始终APPLY。result_summary固定字段processed、failed、row_errors[{row,code,field}]及可选carrier_id/tracking_no，无secret；大结果仅私有脱敏文件。输入/输出文件和创建者由审计主体及options.owner_admin_id关联，缺失owner拒绝访问；options还保存业务输入快照与expected_order_version。客户端不能提交任意options。

验收补充T17：UNKNOWN禁止直接retry；无权限/无reauth不能resolve；旧version409；确认已创建不能再次创建面单；确认作废后才可改址；已发订单resolve409。T21：同job并发commit只产生一次APPLY任务。
