# 04 可执行业务流程

全节为DESIGN_DECISION；证据对应[基线](01-research-baseline.md)，规则见05、状态见03。F编号贯穿API、工作包与测试。

## F00 管理员认证与权限

运维CLI在本地终端创建初始管理员，密码从终端安全输入，不提供默认口令；后续管理员必须有admin.write+5分钟内reauth。bootstrap预会话→challenge→登录；成功旋转session，失败统一INVALID_CREDENTIALS且不暴露账号存在性。每请求读取enabled/auth_version/权限；禁用用户或角色修改递增关联用户auth_version并撤销session。锁身份控制配置行防止并发移除最后一个拥有admin.write/role.write的启用管理员。初次临时密码登录强制改密，未改密只允许改密/退出；API补充见12。审计不存密码。

## F01 商品创建发布

product.write：输入唯一大写code、name、attributes、清洗HTML、READY图片及可选有效期/区域。事务创建DRAFT产品、媒体关联和零库存余额；重复code409。编辑带version；ENABLED发布要求至少主图、有效期顺序合法、分类存在；零库存允许展示但兑换时拒绝。前台仅ENABLED且未删显示；卡券选品同时要求当前商品启用及有效期。下架立即阻止新兑换，不阻止既有订单履约；订单快照不变。分类为单层可选；属性展示，不拆SKU。组合礼包按单商品扣一份库存。

## F02 批次生成、导出、封存与激活

1. createBatch验证卡型版本已启用、product_ids去重且数量>=selection_count、商品存在、时间合法；DRAFT保存快照，批次数量1..1000，前缀0..4，卡号总长12..24。
2. generateBatch要求version、Idempotency-Key、voucher.generate+recent reauth。短事务DRAFT/FAILED→GENERATING，记录任务状态；返回202。worker只认数据库任务，不信任浏览器生成数。
3. worker在内存用crypto/rand生成凭证，计算Argon2id；先构造加密导出文件写到私有临时文件并fsync/原子rename。文件内容含batch_no、code、secret；只保留AES-GCM密文，随机nonce，key_version和批次ID为AAD，禁止把key写文件旁。
4. 单个数据库事务锁批次、核对lease和generation_epoch；每次领取递增epoch，提交时必须匹配，过期worker即使继续计算也不能提交；插入全部vouchers、voucher_products、文件元数据、成功审计，批次→GENERATED。任何卡号冲突或写失败整批回滚；删除孤儿临时文件。无部分可见批次。重试必须先检查批次状态；GENERATED返回已有结果，不再生成另一组凭证。
5. 卡号随机模式逐字符拒绝采样无偏分布；顺序模式按prefix+左补零序号，每个code全局唯一。随机碰撞批内重采样最多5次，跨库唯一冲突整批重试最多3次；顺序区间冲突明确失败，不跳号冒充请求范围。密码默认12位混合，可按已见选项6/8/9/10/11/12和自主扩展16/24/32，所有模式至少6；短数字密码强限流。生成前检查顺序区间溢出。
6. 24小时内凭voucher.export+recent reauth通过受控下载解密流式输出；每次访问审计，无公共URL、无浏览器缓存。下载断开允许再次授权下载，不用一次读取就销毁导致丢卡；凭证过期后不能重新导出原secret。
7. sealBatch（界面“加密/封存”）先数据库锁批次/文件，标记文件EXPIRED、所有卡SEALED；activate_on_seal时设置active；提交后删除密文文件，删除失败Outbox重试且API立即拒绝下载。封存不可恢复旧导出；24小时到期自动同流程封存。秘密hash生成时就存在，封存不是把数据库明文第一次加密。
8. 卡片发放不通过本系统默认群发；后台仅受控文件交付。原站“生成→导出→加密”流程保留，但内部改为安全存储，标ADR09。

worker崩溃：GENERATING lease10分钟，30秒续租；超时worker先查数据库最终状态再重试。孤儿文件按storage_key引用扫描24小时后清理。批次SEALED不可新增卡。大于1000拆成多个批次，不声称支持原站建议10万单批。

激活/停用：最多500显式ID及对应versions，一次事务全成全败。激活只允许SEALED且未消费未删有效期未结束；停用不取消已有订单。软删仅未消费停用卡，卡号永不复用。有效期修改仅未消费卡，时间变动撤销会话；商品权益禁止批量改写。重置密码仅未消费停用卡，recent reauth，随机新secret，凭证版本递增、旧会话撤销，短期加密单卡导出；封存后再激活。不得提供“清除已提货状态”接口。

## F03 鉴真

POST voucher-sessions：preauth+服务端challenge，规范大写卡号、密码原字节不trim。先按IP及HMAC(card_code)计数限流，再查卡/等成本dummy hash校验；错误卡号/密码统一401。正确凭证才返回active/redeemed/有效期及商品公开摘要。即使卡已兑/过期也可鉴真状态，但不签发可兑换权限；会话只能读取自身，不因一次鉴真锁库存或核销。secret校验不把本地拼图标记当凭据。

## F04 选品、预约与兑换

有效卡会话→GET options（集合/规则版本）→恰选N种，每种1件→地址/日期→GET calendar→确认POST redemptions。UI保留选择不保留secret，页面预览不预占库存。最终事务重新校验全部条件：卡状态、版本、商品启用/有效期、地区、日历关闭/额度、库存。完整锁与幂等顺序见13。成功返回唯一PENDING订单，签发ORDER WRITE会话；卡立即redeemed，库存reserved增加。失败不产生订单、不消耗卡/额度/库存；提交超时用原幂等键重试，不能自动换键。

## F05 查询、修改、取消、回收

查询接受卡号或订单号+完整收货手机+挑战，服务端匹配phone_lookup；失败统一401，成功只返回该订单READ会话及脱敏内容。需要修改时用原卡密再次校验升级WRITE，已消费不阻止校验本人，但不赋予再次兑换权。编辑页不预填完整旧PII，用户输入完整替换地址，后台order.pii可经审计读取。仅PENDING且发货日前一日17:00之前允许修改地址/日期/备注；商品集合永不修改。改期锁旧/新日历，新额度通过才原子迁移；不减少原库存预占。

取消由后台order.cancel执行，有reason和version；无运行中/READY运单作业才允许，释放库存预占及一个预约名额；卡保持已消费。客服补偿另发新卡并审计关联reason，不复用卡。回收仅CANCELLED/COMPLETED；恢复只解除隐藏。订单日志为audit筛选，不另做可删日志表。

## F06 库存入出库审核

提交单据不改变库存。审核人在独立请求按version锁单据，锁商品余额升序；入库on_hand+=q；出库必须available>=q才on_hand-=q，不能动reserved。PHYSICAL/VIRTUAL都记同一数量账，只标记来源类型，不绕过库存约束。审核幂等event_key=document:{id}:product:{id}；拒绝无账变动；APPROVED不可再次编辑/撤回，纠错新反向单据并写reason。

## F07 发货物流

到预约日后人工填写carrier/tracking_no；全单T-SHIP将预占转实扣、原发唯一shipment、订单SHIPPED、Outbox与审计一起提交。单号冲突409，全回滚。自动运单先创建异步LABEL任务，不修改库存；成功仅得到待确认单号，管理员确认人工发货接口后完成出库。未知外部超时不重复创建运单，先查询供应商幂等请求号。物流拉取/回调事件去重；原发DELIVERED且订单SHIPPED→COMPLETED，历史事件不倒退；人工完成要求已发、reason、权限，不伪造物流签收。

## F08 售后及补发

ORDER会话用户对SHIPPED/COMPLETED订单申请COMPLAINT或REPLACEMENT；每订单最多一个OPEN/APPROVED案件，由锁订单防并发。补发items来自原订单且数量不超过原项数量。客服批准后人工补发，T-REPLACE锁售后→库存，available>=补发数，直接扣实物（即时发货不长期预占），生成REPLACEMENT运单、唯一after_sale键和账本，原单保持原状态、卡仍已兑。该案件补发只允许一次；送达后关闭。默认不接受自动退货入库或现金退款；退回实物经人工验收另建入库单引用售后ID备注。投诉可直接关闭，拒绝需理由。

## F09 反馈

公开入口姓名/手机/内容必填、订单号/邮箱选填；挑战+限流后加密联系人入feedback。用户填写order_no只能作为参考，不授予订单访问；若当前ORDER会话匹配才设置order_id。后台标记RESOLVED并记录resolution，默认不自动向用户发消息。

## F10 设置通知及F11文件任务

设置采用固定Schema及version，不接受任意键/脚本。日历重建只改未过去日，capacity不得小于used；现存订单的发货日期不因配置改变自动取消。通知DISABLED默认，MOCK仅测试，LIVE须部署密钥及模板；事务Outbox只保存ID。文件先上传→校验→STAGED/READY；CSV导入先预检结果、用户确认commit再写。凭证导出与普通报表不能复用无鉴权下载路径。细节见12/14/16。
