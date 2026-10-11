# 07 Go后端实施设计

**DESIGN_DECISION**：net/http.ServeMux方法路由、database/sql、go-sql-driver/mysql、slog；Argon2id用golang.org/x/crypto，图片/CSV/JSON优先标准库。HTTP JSON错误与Schema校验在transport，不能用反射任意写数据库。业务不依赖HTTP包。

```text
backend/
  cmd/api/main.go
  cmd/worker/main.go
  cmd/admin/main.go                 # 初始账号、改密、运维
  internal/bootstrap/{config,wire}.go
  internal/transport/http/{router,middleware,error,decode}.go
  internal/transport/http/handlers/{catalog,voucher,redemption,order,inventory,shipping,identity,operations}.go
  internal/app/{catalog,voucher,redemption,order,inventory,shipping,identity,operations}/service.go
  internal/domain/{voucher,order,inventory,calendar}/rules.go
  internal/ports/{repository,transaction,clock,random,files,carrier,notifier}.go
  internal/infra/mysql/{tx,products,vouchers,orders,inventory,sessions,outbox}.go
  internal/infra/{crypto,files,integrations}/
  internal/worker/{outbox,generation,imports,reconciliation}.go
  migrations/0001_*.sql
  tests/integration/
frontend/                           # 见10
```

|层|职责|禁止事项|
|---|---|---|
|Router|路径、方法、中间件、handler装配|无业务分支|
|Middleware|request_id、recover、body限额、cookie认证、CSRF、权限、超时|不记录body/凭证|
|Handler|解码一次JSON、DisallowUnknownFields、Schema约束、调用服务、序列化|不直接SQL/开事务/算库存|
|Service|用例、权限细化、状态、幂等与事务、审计/Outbox|不直接外部通知，勿事务内网络|
|Domain|纯数量/日期/状态规则，稳定错误码|不接数据库/HTTP|
|Repository|参数化SQL、映射、显式行锁方法|不做业务裁决、不隐式commit|
|UnitOfWork|BeginTx/rollback/commit，传共享tx到repos|禁止混用db和tx连接执行同用例|
|Infra|MySQL、加密、文件、外部协议适配|不绕过领域状态|

Service签名规范：Execute(ctx,Actor,Command)→Result,error；Actor含subject ID/scope/permission，不接收客户端伪造admin_id。Clock/Random注入用于确定性测试，生产crypto/rand。Repository方法按用例小接口，如LockVoucher、LockInventoryOrdered，不给每个表设计万能CRUD。

事务统一ReadCommitted，SET session time_zone UTC；DSN parseTime=true/loc=UTC；连接池默认25/idle10/lifetime5m。普通请求10秒deadline、兑换5秒、事务lock_wait_timeout=3秒（会话设置需连接池一致）；deadlock/锁等待上层最多重试3次+jitter且整事务重开。同幂等键/无外部网络操作才能自动重试。Commit返回不确定时返回503/request_id，客户端原键重试读取已提交结果。

slog结构字段request_id、operation、entity_id、duration_ms、error_code；不包含输入body、卡号完整值、secret、手机号/地址、Cookie或SQL参数。可访问日志路径模板代替原URL参数。审计成功与业务同事务；失败/鉴权拒绝独立短事务，不写敏感diff。

配置全部env或只读secret文件：DB_DSN_FILE、SESSION_...、PII_KEYRING_FILE、EXPORT_KEYRING_FILE、LOOKUP_HMAC_KEY_FILE、REQUEST_HMAC_KEY_FILE、CHALLENGE_HMAC_KEY_FILE。keyring解析失败拒启动。/healthz仅进程，/readyz检查DB/schema/keyring；仅运维网暴露指标，不泄露数据库错误给用户。

错误映射：领域typed error→统一HTTP码；mysql唯一约束冲突转换409，FK错误输入转换422，其余内部错误500且只给request_id。500也用Error schema。所有success返回权威DB结果，禁止“先Toast成功后异步保存”。
