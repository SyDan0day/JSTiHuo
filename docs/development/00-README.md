# JSTiHuo 全栈开发规格 v1.0

**用途：可以交给编程Agent按工作包实施的独立系统规格。** 本轮仅新增开发文档，没有业务代码、目标站写操作或原站服务器源码。审阅日期2026-10-11。未知原站规则用明确DESIGN_DECISION补足，不要求逐条人工批准。

## 阅读顺序与权威性

先读01基线→02范围→03领域→04流程→05规则→18 ADR，再读06–14实施细节；执行时从17选择一个包，按20模板交付。机器接口以openapi.yaml为字段权威，数据库以schema.sql为迁移起点，业务状态与事务以03–05/13为权威；若实施发现矛盾，先同步修正文档/契约再编码，不默默忽略。

旧研究保留在docs/research；新接口全部位于/api/v1且不调用目标网站。原站事实不能由自主设计覆盖。原型52页只提供页面和菜单参照，真实证据优先。

## 文件目录

|文件|内容|
|---|---|
|[01-research-baseline.md](01-research-baseline.md)|01 统一调查基线|
|[02-requirements-and-scope.md](02-requirements-and-scope.md)|02 需求与范围|
|[03-business-domain.md](03-business-domain.md)|03 业务领域与状态机|
|[04-business-workflows.md](04-business-workflows.md)|04 可执行业务流程|
|[05-business-rules.md](05-business-rules.md)|05 规则常量、字段与异常|
|[06-system-architecture.md](06-system-architecture.md)|06 系统架构|
|[07-backend-design.md](07-backend-design.md)|07 Go后端实施设计|
|[08-database-design.md](08-database-design.md)|08 数据库详细设计|
|[09-api-specification.md](09-api-specification.md)|09 API契约|
|[10-frontend-design.md](10-frontend-design.md)|10 Vue用户端设计|
|[11-admin-design.md](11-admin-design.md)|11 管理后台设计|
|[12-authentication-and-security.md](12-authentication-and-security.md)|12 认证、凭证与安全|
|[13-inventory-and-consistency.md](13-inventory-and-consistency.md)|13 库存与事务一致性|
|[14-shipping-and-integrations.md](14-shipping-and-integrations.md)|14 物流、通知与文件集成|
|[15-testing-and-acceptance.md](15-testing-and-acceptance.md)|15 测试与自动验收|
|[16-deployment-and-operations.md](16-deployment-and-operations.md)|16 部署、运维与恢复|
|[17-implementation-work-packages.md](17-implementation-work-packages.md)|17 实施工作包|
|[18-design-decisions.md](18-design-decisions.md)|18 设计决策记录（ADR）|
|[19-requirements-traceability.md](19-requirements-traceability.md)|19 需求—证据—实现追踪|
|[20-development-execution-guide.md](20-development-execution-guide.md)|20 编程Agent执行指南|
|[00-README.md](00-README.md)|总入口、范围与校验边界|
|[openapi.yaml](openapi.yaml)|OpenAPI 3.1.1：106操作、108 Schema，权限/事务/幂等/测试扩展|
|[schema.sql](schema.sql)|MySQL 8.4：37张领域与支撑表、索引、CHECK、外键|
|[reviewed-sources.json](reviewed-sources.json)|研究来源及SHA-256，区分阅读与仅清点|
|[quality-review.md](quality-review.md)|交叉审核、已执行静态检查与尚未执行运行验证|

## 最终方案

12模块、37表、106接口操作、16 ADR、17工作包、22流程验收矩阵及每接口契约用例。Go net/http模块化单体 + MySQL 8.4 + Vue3/TypeScript/Vite + Docker Compose；后台Element Plus；Go testing/Playwright；slog。API/worker共享MySQL事务与Outbox，不引入Redis或独立消息队列。技术与表结构全部DESIGN_DECISION。

核心闭环：商品发布/入库→批次生成→短期受控导出→封存/激活→鉴真→选品预约→原子核销/预占/订单→发货实扣→轨迹签收完成。取消释放库存和日额度，但不还原已消费卡券；补偿另发卡。ONE_OF选1种，N_OF恰N种，每种1件，一卡一次一订单。默认无支付、无分次、多仓或SKU。

先执行A1基础工程、A2身份安全、B1商品；然后B2库存→C1/C2卡券→D1/D2兑换→E1/E2订单物流，F/G补齐门户管理，H完成验证上线。真实供应商默认禁用，MANUAL/MOCK保证本地闭环；延期功能不显示假成功按钮。

## 就绪与风险

规格可直接启动开发。不是“系统已开发/已上线/与原站所有规则等同”。OpenAPI、Schema示例、SQL语法与引用进行静态校验，见quality-review；Go/MySQL事务、浏览器E2E、真实供应商及备份恢复须在对应工作包实际执行。当前没有可运行代码或Git仓库；A1先建立工程与版本控制，再生成依赖锁定文件。

仍有兼容风险：原站多选多数量含义、内部加密、取消后的卡券处理、库存扣减时点未被成功交易实测。新系统已有确定默认与回归用例；后续出现新证据采用追加ADR、版本化规则及迁移，禁止重写已发行卡的权益快照。生产密钥、域名TLS、供应商凭据和隐私保留政策是上线配置门槛，不是开始编码的阻塞。
