# 15 测试与自动验收

**DESIGN_DECISION**：以下是实施时必须执行的测试，不是本轮已通过的业务测试。仅自有本地数据，APP_ENV=test、MOCK供应商、固定Clock；不使用原站卡密/客户资料。

## 测试层与执行约定

领域单元：Go testing表驱动数量/日期/状态；模糊测试卡号规范化、CSV解析与富文本清洗。集成：真实MySQL8.4独立数据库、真实事务与唯一约束，不用SQLite替代锁语义。并发：barrier同时启动goroutine，go test -race，并检查DB不变量。契约：OpenAPI每操作请求/响应Schema、401/403/字段错误；Playwright完整浏览器UI到Go/MySQL，不用静态Toast替代。供应商测试用httptest可控超时/响应。

建议目录backend/internal/domain/*/*_test.go、backend/tests/integration、tests/e2e、tests/contract；工作包A定义make test/unit/integration/e2e/contract及Compose test profile。CI运行单元+契约+MySQL集成+E2E，夜间100并发/恢复测试。代码尚未存在，命令为实施目标。

## 可自动执行场景矩阵

|ID|领域|输入/操作|必须断言|工作包|
|---|---|---|---|---|
|T01|身份权限|错误密码、CSRF缺失、过期cookie、删最后管理员、临时密码强制修改|401/403/409且无业务修改；成功会话旋转，旧会话失效|A2/G3|
|T02|批次发行|固定随机源生成3张；第2张唯一冲突；worker在文件写后/commit前崩溃；过期lease旧worker提交|成功恰3卡且每张集合一致；任一DB失败0卡；孤儿文件不可下载；epoch不符禁止提交|C1/C2|
|T03|商品|创建/重复编码/无主图上架/下架已有关联订单|409/422；下架阻止新兑但已有订单可发；订单快照不随改名|B1/F1|
|T04|权益数量|ONE_OF 0/1/2，N_OF N-1/N/N+1，重复ID/越集合/qty2|仅合法集合成功；422无卡/库存变化|C1/D1|
|T05|重复并发核销|100协程同卡同键；100协程同卡不同键；同键变地址；提交成功响应丢失|同键同请求均同order_no；不同键恰1成功其余409；变请求409；恢复不重扣|D2|
|T06|跨商品库存|两张卡争最后1库存；多商品第二项不足；事务每步故障注入|无负库存；只有一个消费者成功；失败所有商品预占/卡/订单/日历/Outbox回滚|D2|
|T07|取消|两次同键取消、两个版本并发取消、已发取消、取消后重兑|预占和额度只释放一次；已发409；原卡仍redeemed，不能新兑|E1|
|T08|库存审核|入库/出库批准两次；出库试图占用reserved；拒绝；并发批准|每单一次账本；available不足422；拒绝不动库存；余额等于流水|B2|
|T09|发货|同单多键并发、同单号不同订单、早于日期、已取消发货、重复DELIVERED|仅一主运单；唯一冲突409整事务回滚；早发拒绝；预占转实扣一次；完成只一次|E2|
|T10|预约日期|注入上海16:59:59/17:00/23:59、跨月年、停发、day+30/+31|可选边界严格符合BR05；UTC无偏移错误；无需系统真实等到时间|D1|
|T11|日额度|capacity1两单并发；改期到满日；取消恢复；配置降到used以下|只有1成功；失败改期保留旧slot；取消恢复1；配置409|D2/E1/G3|
|T12|有效期|now=from、until-1微秒、until；鉴真已兑/过期；商品日期限制|from有效until无效；状态可查但不可兑；商品限期按BR07|C1/D2|
|T13|地区|省/市/区黑名单命中；错误父子关系；两个商品不同黑名单|并集拒配；错误地区422；API不能绕过UI禁选|D1/D2|
|T14|修改|READ scope、WRITE成功、version过期、截止前后、发货与修改竞争|READ403；只地址日期备注；截止/已发409；只有一个版本成功|E1|
|T15|凭证保护|下载前复核、他人文件、TTL、封存并发下载、重置后旧secret、扫描日志/备份|越权403/404；过期410；封存阻止新流；旧secret401；无明文卡密|C2/G1|
|T16|售后补发|超原数量、未批准、缺货、重复补发、签收关闭、已发取消|仅批准合法qty可扣available；一案件一运单；原单及卡不逆转|G2|
|T17|外部失败|provider 500/超时未知/重复回调/worker调用后崩溃/通知关闭|可幂等重试；未知不盲发；事件只一次；DISABLED为SUPPRESSED|G2|
|T18|访问隔离|用A订单session访问B物流；卡号+错误手机；表单XSS/SQL注入；伪造captcha|统一拒绝/清洗，无越权PII、无未授权业务请求；服务端验证挑战|A2/F2/G3|
|T19|软删恢复|已取消/完成回收恢复、待发回收、已兑卡删除、审计写失败|恢复不改状态；禁止待发/已兑删除；审计失败业务回滚|E1/G1|
|T20|页面契约|全部活动路由每按钮映射operationId、无权限、空态错误态、390px键盘/底栏|无假Toast；无未定义API；无横向截断或底栏遮挡；延期不显示可执行按钮|F1/F2/G3|
|T21|导入与文件|预检失败、同文件重复commit、运单部分失败重试、恶意路径/CSV公式/超大图|卡券整批回滚，运单逐行结果可追踪；重复不发两次；文件拒绝/转义|G1|
|T22|部署恢复|空库迁移、已迁移checksum变更、加密备份恢复、过期导出清理、账本对账|部署可重复；checksum拒绝；恢复可解密必要PII但不恢复过期导出；零漂移|H1/H2|

## 必须通过的14步E2E

固定时钟2026-10-12 10:00 Asia/Shanghai。使用test-admin（运行时生成随机密码），商品A/B/C各入库批准10，生成N_OF=2、3商品集合、2张卡，有效期覆盖时钟，初始不激活。

1. 管理员登录并创建A/B/C、上传本地fixture主图；断言列表及公开页持久化。
2. 建N_OF批次并请求生成；轮询GENERATED，DB恰2张卡。
3. 授权下载测试凭证到测试进程内存，不输出到录像/trace；封存，后续下载410。
4. 激活卡1、卡2，验证未激活前不可兑换。
5. 客户卡1正常挑战及鉴真；断言未核销、未预占。
6. 选择A+B（C未选）；验证不能选择3个。
7. 输入虚构收货资料及明天日期，读取可用日历。
8. 点击确认；模拟重复点击同键，均同一订单。
9. 后台出现PENDING；卡已兑；A/B reserved=1,C=0，日额度+1。
10. Clock前进到预约日，管理员人工MOCK单号发货；A/B on_hand=9,reserved=0。
11. 用户用单号+测试手机查询，看到脱敏订单SHIPPED。
12. MOCK回调IN_TRANSIT，页面显示测试轨迹；其他订单session访问404。
13. MOCK DELIVERED完成；后台和前台COMPLETED，重复回调无重复Outbox。
14. 卡1不同键再次兑换409；卡2仍可用；全局账本/预占/消费唯一约束核对通过。

另跑取消分支：卡2下单→取消→reserved/slot恢复，卡2仍已兑；跑到店分支用新卡→COMPLETED/PICKUP、无日历/运单；补发分支从完成订单发起案件，不恢复原卡。

## 全局断言与质量门禁

每个测试事务后核对inventory不变量、每卡最多1redemption、每order仅1主shipment、每event_key唯一。重要服务覆盖每一状态转换，不以总体行覆盖百分比代替规则覆盖。API的x-tests约3项/操作必须由contract runner生成并根据操作主体约束补数据。弱网重试必须保持幂等键。

性能指标按06在记录硬件、数据规模、并发数后验收；不达到则定位锁/索引，禁止移除事务保证。EXPLAIN列表和热点锁查询使用预期索引；10万订单列表游标查询不做无界offset。审计/导出扫描不得含TEST凭证明文以外任何敏感日志，凭证明文fixture也应从日志排除。

## 交叉审核新增契约验收

T01校验Session.force_password_change控制临时口令权限；T05包含提交成功后卡已消费但原有效会话仍可同键恢复订单；T08包含setInventoryWarning只改阈值；T17包含resolveJob/resolveNotification的复核、版本和外部核对凭据；T20包括后台adminOrderShipments/adminShipmentEvents/getAfterSale读取，禁止借用客户会话；T21包含PREVIEW→APPLY的并发确认与分阶段UI。

列表契约特别测试orders.deleted默认false、true仅回收站，stockDocuments.direction、auditLogs.entity_type/entity_id、vouchers.batch_id/active/redeemed及inventoryLedger.product_id的白名单校验。OpenAPI中的q/status通用参数对不支持资源必须明确拒绝，不能拼任意SQL。
