# H1 一致性、安全及全链路验收

2026-10-15 实施。已补齐只读库存对账、十万订单性能验收、完整浏览器主链、活动路由三档检查和真实 HTTP 契约观察器。原有 A1–G3 的授权、事务、幂等、审计和业务不变量保持有效。

## 实现与验证范围

- `operations/reconciliation.go`：在只读一致快照中比较余额、全部流水与活动预占，最多返回 100 个漂移商品 ID；后台可按权限执行，worker 定期检查。不自动补账。
- `redemption/order_list.go` 与 `order/service.go`：批量读取订单项；兑换使用批次共享锁与目标卡排他锁。性能失败后的原因、优化与受限实测见 [性能报告](H1-performance.md)。
- `tests/contract/generate-cases.mjs` 从 OpenAPI 生成 106 操作的安全边界目录，并用 SHA-256 防止目录过期。`TestContractSecurity` 检查适用的缺身份、CSRF、缺字段与未知列表参数。
- `tests/contract/validate-http.cjs` 复用已锁定依赖中的 Ajv 2020；Go 集成测试在显式 CONTRACT_VALIDATE 模式将真实 HTTP 成功输入与响应经内存管道校验，不把卡密或 PII 写入日志或观测文件。检查类型、必填、额外字段、枚举、数组和数值边界；format 采用 JSON Schema 2020 的注解语义，日期合法性另由真实业务边界测试验证。
- 正向补充测试不再用服务调用替代 HTTP 路由：覆盖卡型/批次发行、元数据编辑、读取、订单状态、私有资料和补发。发现并补齐 `GET /voucher-session`；读取刷新 CSRF，顾客权限返回空数组，不扩大会话范围。上下架返回契约要求的完整 Product，前端按原动作显示结果；退出和已处理回调统一契约为无正文 204。

## 业务与浏览器证据

`tests/browser/fulfillment.cjs` 在真实 Vue/Go/MySQL/worker 上完成：实际登录与强制改密复核→三种商品图片保存上架→入库审批→N_OF=2 两卡生成→卡密只下载到进程内存→封存激活→选 A+B 并限制第三种→完整地址预约→首次 commit 后丢响应→原键原正文恢复→预约日发货→订单号和手机查询→MOCK 运输及签收→第二张卡下单取消→只读对账。

断言 A/B on_hand=9、reserved=0，C=10，取消恢复额度和库存但不返卡，已消费卡异键再次兑换 409。到店与补发分支由对应真实 HTTP/MySQL 集成测试覆盖，并有 G2 售后浏览器专项。

[evidence/h1](evidence/h1) 保留 6 个业务状态各 390/768/1440 的 18 张视口截图；固定业务时钟下避免全页拼接截图的重影，额外断言实际 DOM 无重复公开布局。无真实卡密、完整地址或登录口令截图。

`tests/browser/routes.cjs` 从实际 Router 解析并检查 58 个路由模板，各 390/768/1440，保存 174 张截图及 [routes.json](evidence/h1-routes/routes.json)。无横向溢出、pageerror 报告或服务器 500；动态任务路由由 G1/G2 实际任务专项覆盖。缺少顾客 Cookie 的受保护页面检查真实拒绝/恢复状态，不冒充正向访问。全部路由截图不等同于每个控件的组合状态枚举。

## 实际测试结果

最终 `CONTRACT_VALIDATE=1 CONTRACT_REQUIRE_ALL=1 go test -race -v ./tests/integration -count=1` 在新建隔离空库通过，用时 277.720s；106 个操作均观察到真实成功响应并通过适用 JSON Schema，未缺失正向操作。安全矩阵和业务事务断言同时通过，详见 evidence/h1/verification.json 与 http-contract-coverage.json。`make test`、`go vet -tags browserfixture ./...` 通过。新增隐私保留、PII 恢复校验和售后受影响回归曾以 37.047s 通过，已纳入该全量。性能目标另行验收，不计同步 Schema 观察器开销。

安全、异常与一致性包含：100 同键/异键并发，跨商品不足、逐步事务故障、真实数据库断连接、取消/发货竞争、配置/预约竞争、最后管理员并发、外部 HTTP 500/超时/UNKNOWN、回调重复乱序、六小时拉取/三十天人工异常、私有文件 TTL/路径隔离、恢复整体回滚、180 天匿名化与活动售后暂停。故意断连接用例可能输出 MySQL unexpected EOF，成功标准是对应断言及最终 PASS。

没有真实供应商、公网压力、真实移动设备键盘或正式无障碍认证结论；Chrome 仿真视口与键盘焦点检查不替代真机验收。本地交付范围为 MANUAL/MOCK，生产边界见 H2。

全量后收尾：日历启动/跨日补建及匿名化锁内复查、三 worker 单次提交受影响专项 race 19.122s 通过；随后再次通过 make test 与 Go vet。最终容器中确认 worker 自动补齐至少 31 个日期，HTTPS 浏览器读取通过。

非阻断观察：Vite 在快速视口切换时曾记录 ResizeObserver loop completed with undelivered notifications 警告；没有对应的 pageerror、业务断言失败或横向溢出。此项作为开发组件警告记录，未宣称控制台零警告；生产真机交互仍应复验。
