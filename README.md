# JSTiHuo

根据 [开发规格](docs/development/00-README.md) 实施的独立提货系统。已实现 A1–H2 的本地交付：商品库存、卡券发行、兑换订单、履约售后、导入导出、门户内容、反馈、权限配置及可靠异步任务。MANUAL/MOCK 完成闭环；真实供应商和生产发布须另行配置与验收。交付证据见 [开发状态](docs/development/IMPLEMENTATION-STATUS.md)。

## 本地开发

Go 1.26.1、Node 24+、Docker Compose 与 MySQL 8.4。依赖已锁定于 go.sum 与 package-lock.json。

```sh
npm ci --prefix frontend
make test
python3 deployments/init-local.py
docker compose -f deployments/compose.yaml up -d mysql
docker compose -f deployments/compose.yaml --profile tools run --rm migrate
docker compose -f deployments/compose.yaml exec -T mysql sh -c 'MYSQL_PWD=$(cat /run/secrets/db_root) mysql -uroot jstihuo < /run/secrets/app-user.sql'
docker compose -f deployments/compose.yaml up -d --build api worker proxy
```

打开 http://localhost:8080，后台工程入口为 /admin。本地密钥只生成一次并忽略提交；重复执行生成命令会拒绝覆盖。数据库账号创建只需初次执行；应用账号无 DDL 权限。迁移与 API 启动相互独立，API 不自动建表。

`GET /healthz` 检查进程，`GET /readyz` 检查数据库和全部迁移 checksum。启动校验全部密钥；探针不返回数据库错误详情。探针为运维接口，不属于 /api/v1 业务操作。

前端开发可执行 `npm --prefix frontend run dev`，API 代理指向本地 8080。业务请求客户端支持同源 cookie、内存 CSRF 与用户动作的固定幂等键；没有自动重试。

## 验证

`make test` 执行 Go race 单元测试、OpenAPI 规范验证及类型生成、37 表迁移一致性检查、Go/Vue 构建。`TEST_DB_DSN='...' make integration` 在**隔离空库**运行真实 MySQL 迁移、重复执行及 checksum 篡改拒绝测试，该测试会篡改测试库迁移记录，不可使用生产库。CI 提供 MySQL 8.4 服务运行该测试。

MySQL DDL 不能整体回滚。部分失败会停止并保留现场，需运维检查修复，禁止盲目删除数据重跑。已完成隔离环境 HTTPS、镜像 digest 锁定、加密备份、binlog 增量恢复和回滚演练，见 [部署与恢复手册](docs/runbooks/operations.md)；实际生产域名、证书、异机备份及供应商尚未验证。

worker 已接入生成、文件任务、TTL、轨迹调度、Outbox、库存对账、指标日志与订单隐私保留。`TEST_DB_DSN='隔离测试库' make e2e` 执行协议主链；真实浏览器 14 步、取消和三档路由检查已通过，见 [H1 验收](docs/development/H1-delivery.md)。

## A2 管理员认证

打开 `/admin/login` 登录；首次登录必须改密并重新登录。`/admin/security` 提供敏感操作密码复核。首次初始化在本地终端运行，密码无回显：

```sh
docker compose -f deployments/compose.yaml --profile tools build admin
docker compose -f deployments/compose.yaml --profile tools run --rm admin admin --login-name your-admin
```

CLI 只允许空管理员库，重复执行不会覆盖账号。运行业务库没有预置账号。生产仅允许 HTTPS；本地 HTTP 仅 development 可用。改密撤销所有会话，退出、复核旋转与禁用账号检查均由服务端执行。

Vite 调试时应将 API 的 APP_ORIGIN 设置为实际浏览器 Origin（如 http://localhost:5173）；Compose 默认 http://localhost:8080，可由 APP_ORIGIN 配置。管理员、角色与权限在 `/admin/admins` 与 `/admin/roles` 维护；服务端阻止提权和移除最后一个完整管理员。

## B1 商品管理

管理员登录后打开 /admin/products。新增时先上传 PNG/JPEG 图片，填写编码、名称和属性，保存为草稿，再单独上架。上架后在 /products 及商品详情公开展示。最多9张图片，首张为主图；单张输入最多5 MiB。编辑提交完整快照与版本，冲突后重新读取核对。

保存商品与零库存余额创建同事务。库存入库及审核已在B2实现，公开展示不代表兑换已经开放。

集成测试需用具有 CREATE/DROP DATABASE 权限的隔离 MySQL 测试账号。商品测试创建临时子库并在完成后删除；认证及迁移测试仍要求给定空测试库。不要使用业务或生产库作为 TEST_DB_DSN。

## B2 库存管理

打开 `/admin/inventory` 查看实际库存、预占、可用量及流水，设置警戒值。`/admin/inventory/documents` 创建入出库单并审核。建单引用商品列表中的商品 ID；单据批准后才变更库存。出库不能动用已预占数量。拒绝需填写原因。库存写权限与审核权限分开控制。

审核和警戒值编辑均比较版本，并支持同键重试；冲突后刷新核对。详细事务与测试结果见 [B2交付记录](docs/development/B2-delivery.md)。

## C1 卡型与批次生成

打开 `/admin/voucher-batches` 创建不可变卡型版本、选择权益商品并保存批次草稿；详情页完成身份复核后提交生成，后台自动轮询任务状态。`/admin/vouchers` 查询卡券，不显示密码。生成权限为 `voucher.generate`，查询为 `voucher.read`，商品选择还需要 `product.read`。

必须同时启动 `api` 和 `worker`。单批最多 1000 张，任务支持租约到期后恢复；卡号冲突整批失败，无部分可用卡。卡券默认未激活，只有短期 AES-GCM 导出密文持久化；受控下载、封存、激活停用、软删除、有效期编辑和凭证重置已在 C2 实现。详细实现和验证边界见 [C1 交付记录](docs/development/C1-delivery.md)。

C2 操作及验证见 [C2 交付记录](docs/development/C2-delivery.md)。全部剩余工作包进度见 [持续开发状态](docs/development/IMPLEMENTATION-STATUS.md)。

## D1/D2 鉴真与兑换

打开 `/verify` 以卡号、密码和服务端图片挑战鉴真；可兑换卡进入选品、地区地址和预约日确认流程。提交一次消费卡券并原子创建待发订单、预占库存与预约名额。网络结果不确定时使用原请求恢复。顾客订单在 `/orders/result` 查看脱敏信息，管理员通过 `/admin/orders` 查看待发订单。

地区数据锁定为 2023 年省市区版本并随代码交付；不设区城市使用“全市”第三层，镇街填详细地址。无需新增迁移。订单操作和履约已在 E1/E2 接通；各阶段验证范围见 [D1/D2 交付记录](docs/development/D1-D2-delivery.md)。

## E1 订单操作

`/orders/query` 用订单号或卡号与完整收货手机获取只读会话；修改需在 `/orders/edit` 用对应原卡密与新挑战复核。更新必须填写完整替换地址，只有原预约日前一天上海 17 点前的待发配送订单可以修改。

管理员在 `/admin/orders/:id` 按权限执行修改、取消、回收、恢复和有原因的完整地址查看。取消只释放预占和预约名额，不恢复卡券；回收/恢复只改变显示。详见 [E1 交付记录](docs/development/E1-delivery.md)。

## E2 发货、到店与物流

`/admin/carriers` 维护承运商目录。`/admin/orders/:id/ship` 在上海预约日到达后确认全单发货，原预占转实扣；同单只创建一个主运单，同承运商单号不能重复。订单详情可有原因地人工完成，实际轨迹单独展示。`/admin/pickups` 现场核验原卡密和商品 ID 集合，直接完成到店订单，不占配送名额或其他订单预占。

只有显式 MOCK 承运商会由 worker 在发货后三十秒/六十秒生成标注模拟的运输/签收事件；MANUAL 不自动生成轨迹。KDNIAO/KUAIDI100 当前仅保存目录映射，尚未调用真实服务。顾客在 `/orders/tracking` 读取自己订单的物流。权限、事务边界与测试详情见 [E2 交付记录](docs/development/E2-delivery.md)。

## F1 公开内容与响应式页面

首页 `/` 展示真实商品、已发布推荐和公告；`/products` 与详情页使用统一公开布局。`/pages/:slug` 读取已发布内容，未发布或撤下统一提示不可用。移动底栏提供首页、鉴真、提货、进度和联系入口。

管理员从 `/admin/content` 创建、编辑、发布和撤下自有内容，需要 `content.read` / `content.write`。固定信息入口使用 `guide`、`about`、`contact`、`notice` 四个 slug；新安装不预置商家身份或联系方式，须由运营核实发布。正文服务端清洗，编辑要求完整快照及版本；网络结果不确定时表单锁定，使用原请求确认。无需新增迁移。

真实 MySQL、浏览器发布闭环与 390/768/1440 截图见 [F1 交付记录](docs/development/F1-delivery.md)。浏览器夹具通过 `browserfixture` 标签单独启用，不随普通测试启动服务。

## F2–G3 用户与运营功能

顾客可提交真实持久化反馈，在本单写授权下发起投诉或补发。后台支持反馈处理、售后决策、补发履约、CSV 预检与确认、脱敏报表、文件到期管理、通知及面单结果核对。外部调用结果未知时必须先复核，不能直接重发。

后台提供统计、账户角色、预约规则、审计、只读地区与本地公开二维码。使用步骤见 [业务操作指南](docs/runbooks/user-guide.md)；事务与边界见各 F/G 交付记录。

## 部署与恢复

[运维手册](docs/runbooks/operations.md) 包含首次部署、HTTPS、秘密与密钥轮换、备份恢复、故障处理和回滚。`recovery` CLI 支持独立密钥的认证加密归档，仅包含 SQL 与公开图片，排除卡密导出和导入源；恢复后必须安全收敛并检查账本、PII 和外部任务。

本地交付使用自有合成资料，未向真实收件人发送消息。生产运行需真实域名证书、内容确认、异机备份与规模演练，LIVE 供应商默认不可用。没有自动创建 Git 提交或推送。
