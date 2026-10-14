# E1 订单查询、修改、取消与回收

2026-10-14，续接 D1/D2，复用现有表，无 DDL 变更。

## 已实现

- queryOrder：PREAUTH 与 ORDER_QUERY 挑战、标识/IP 的共享 HMAC 桶限流、卡号或订单号加完整手机匹配。错误统一 INVALID_CREDENTIALS；成功只签发 30 分钟 ORDER READ 会话，手机不进入 URL、审计或幂等结果。
- orderStepUp：保留 ORDER 会话并建立新的 VOUCHER 专用预认证挑战，使用对应原卡号/密码升级 WRITE。已消费、停用或过期不阻断既有订单本人复核；锁内重新检查凭证版本和原哈希，消费挑战并撤销旧 ORDER/PREAUTH 会话。
- customerOrderEdit/adminOrderEdit：完整新地址、日期和备注带 version 提交，禁止商品替换。只允许未回收的待发配送订单，在原预约日前一天上海 17 点前。先准备绑定订单号的 AES-GCM 密文及手机 HMAC，再按旧、新日期升序锁日历，复查容量、禁配和新预约商品期限；名额迁移与地址更新一次提交，不改变预占库存。同键成功恢复优先于状态/版本检查。
- privateOrder：order.pii 权限、版本和非空原因检查，完整地址读取前持久化审计；每次成功访问包含同键重放都审计，幂等记录只保存对象引用，读取和版本核对在同一授权事务内。
- orderCancel：order.cancel 权限及非空原因；只有待发配送、无未决 LABEL 作业才允许。按日历、商品及余额固定顺序锁资源；ACTIVE 预占转 RELEASED、reserved 减少、唯一 release 账本、日额度释放、CANCELLED 状态、Outbox 与审计同事务。卡券消费时间永久保留。
- orderTrash/orderRestore：只处理 CANCELLED/COMPLETED 终态，版本比较，只变 deleted_at 和版本，恢复不会改变状态或返还卡券、库存。
- 后台 `/admin/orders`、`/admin/orders/:id`、`/admin/orders/:id/edit` 支持待发/终态/回收站、详情、完整替换地址及有权限的取消/回收/恢复/审计 PII 操作。用户 `/orders/query`、`/orders/result`、`/orders/edit` 支持只读查询、原卡密提权、级联地址和日期建议。密码提交后立即清空；地址只在页面内存存在，不填充原完整 PII。

编辑和取消按校验、准备密文、固定资源锁、原子变化提取有业务职责的辅助函数；不以缩行删除安全检查。新增 Vue/TypeScript 已用临时官方 Prettier 3.9.10 展开多行，未增加生产依赖。函数注释按用途、完整参数、返回值分项，事务和并发约束独立说明，不做字符截断或 GoDoc 代码块续行。

## 契约修订

现有 redemptionCalendar 原来只允许卡券会话，无法支持已消费卡对应订单改期。补充 ORDER 授权：只读取自身未回收、未过修改截止的待发配送订单日期建议，不授予再次兑换或修改能力。修改仍需要 ORDER WRITE。属于同一路由向后兼容授权补充，不添加未定义新 API。

订单 JSON 无 scope 字段，新增 X-Order-Scope 成功响应头（READ/WRITE），用于界面提示；服务端始终从数据库会话授权。保留此前 CSRF 分作用域内存策略。配置停止新预约不能撤销既有日期；保持原日修改地址不受新 blackout/closed 影响，自身已占日期即使满额仍可显示。

## 实际验证

- `GOCACHE=/private/tmp/jst-go-cache make test`：Go race 单元、OpenAPI 校验与类型生成、37 表一致性、Go/Vue 构建通过。
- `cd backend && GOCACHE=/private/tmp/jst-go-cache go vet ./...`：通过。
- 独立 MySQL 子库 `go test -race -v ./tests/integration/... -run TestRedemption -count=1`：订单/兑换联合验证通过；真实 HTTP 追加后耗时 13.621 秒。追加同键取消与双向并发改期后，再次通过，13.484 秒。
- 用例覆盖 READ 拒写、原卡密升级/旧会话撤销、跨订单写拒绝、满日改期保留旧名额、迁移中故障全回滚、成功同键恢复、旧版本拒绝、不改变预占、PII 每次读取审计、取消审计失败回滚、两个版本竞争仅一次取消、卡仍消费、回收恢复不改状态、待发拒回收和截止时间拒绝。
- 真实 HTTP Handler 验证查询 READ、PATCH 403、带独立 PREAUTH 挑战提权、ORDER 预约建议、PATCH 成功和原键恢复；全部请求均带实际会话 Cookie/CSRF。
- 回归新配置停止预约时既有原日地址仍能更新，自身已占日期保持可选。

## 尚未完成

E2 人工发货、到店、承运商和轨迹已在后续本轮接入，取消与发货同版本争用和协议主链验证见 [E2 交付记录](E2-delivery.md)。全套浏览器验收及 390/768/1440 截图仍属于 H1。未发送真实通知或执行真实发货；累计 C1–E2 随本轮统一提交。

最终全套回归：新建隔离空根库 `jst_e1_final`，执行 `go test -race -v ./tests/integration/... -count=1` 全部通过，56.260 秒。`make test`、`go vet ./...`、gofmt、`git diff --check` 通过。源码检查未发现普通函数说明被格式化为 GoDoc 代码块或缺少用途/参数/返回值分项。
