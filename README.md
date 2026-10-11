# JSTiHuo

根据 [开发规格](docs/development/00-README.md) 实施的独立提货系统。已实现 A1 基础工程与 A2 管理员认证与 B1 商品管理及公开目录、B2 库存单据与审核。

## 本地开发

Go 1.26.1、Node 24+、Docker Compose 与 MySQL 8.4。依赖已锁定于 go.sum 与 package-lock.json。

```sh
npm ci --prefix frontend
make test
python3 deployments/init-local.py
docker compose -f deployments/compose.yaml up -d mysql
docker compose -f deployments/compose.yaml --profile tools run --rm migrate
docker compose -f deployments/compose.yaml exec -T mysql sh -c 'MYSQL_PWD=$(cat /run/secrets/db_root) mysql -uroot jstihuo < /run/secrets/app-user.sql'
docker compose -f deployments/compose.yaml up -d --build api proxy
```

打开 http://localhost:8080，后台工程入口为 /admin。本地密钥只生成一次并忽略提交；重复执行生成命令会拒绝覆盖。数据库账号创建只需初次执行；应用账号无 DDL 权限。迁移与 API 启动相互独立，API 不自动建表。

`GET /healthz` 检查进程，`GET /readyz` 检查数据库和全部迁移 checksum。启动校验全部密钥；探针不返回数据库错误详情。探针为运维接口，不属于 /api/v1 业务操作。

前端开发可执行 `npm --prefix frontend run dev`，API 代理指向本地 8080。业务请求客户端支持同源 cookie、内存 CSRF 与用户动作的固定幂等键；没有自动重试。

## 验证

`make test` 执行 Go race 单元测试、OpenAPI 规范验证及类型生成、37 表迁移一致性检查、Go/Vue 构建。`TEST_DB_DSN='...' make integration` 在**隔离空库**运行真实 MySQL 迁移、重复执行及 checksum 篡改拒绝测试，该测试会篡改测试库迁移记录，不可使用生产库。CI 提供 MySQL 8.4 服务运行该测试。

MySQL DDL 不能整体回滚。部分失败会停止并保留现场，需运维检查修复，禁止盲目删除数据重跑。现阶段没有生产发布或备份恢复验收；Compose 仅供 localhost 开发，生产 TLS、镜像 digest 锁定与恢复演练属于后续交付门槛。

后续工作包为 C1 卡券批次。worker 的真实任务执行在 C1/G2 接入，尚未创建空转 worker。14 步业务 E2E 于 D2/E2 后实施。

## A2 管理员认证

打开 `/admin/login` 登录；首次登录必须改密并重新登录。`/admin/security` 提供敏感操作密码复核。首次初始化在本地终端运行，密码无回显：

```sh
docker compose -f deployments/compose.yaml --profile tools build admin
docker compose -f deployments/compose.yaml --profile tools run --rm admin admin --login-name your-admin
```

CLI 只允许空管理员库，重复执行不会覆盖账号。运行业务库没有预置账号。生产仅允许 HTTPS；本地 HTTP 仅 development 可用。改密撤销所有会话，退出、复核旋转与禁用账号检查均由服务端执行。

Vite 调试时应将 API 的 APP_ORIGIN 设置为实际浏览器 Origin（如 http://localhost:5173）；Compose 固定为 http://localhost:8080。管理员/角色编辑接口及页面于 G3 接入。

## B1 商品管理

管理员登录后打开 /admin/products。新增时先上传 PNG/JPEG 图片，填写编码、名称和属性，保存为草稿，再单独上架。上架后在 /products 及商品详情公开展示。最多9张图片，首张为主图；单张输入最多5 MiB。编辑提交完整快照与版本，冲突后重新读取核对。

保存商品与零库存余额创建同事务。库存入库及审核已在B2实现，公开展示不代表兑换已经开放。

集成测试需用具有 CREATE/DROP DATABASE 权限的隔离 MySQL 测试账号。商品测试创建临时子库并在完成后删除；认证及迁移测试仍要求给定空测试库。不要使用业务或生产库作为 TEST_DB_DSN。

## B2 库存管理

打开 `/admin/inventory` 查看实际库存、预占、可用量及流水，设置警戒值。`/admin/inventory/documents` 创建入出库单并审核。建单引用商品列表中的商品 ID；单据批准后才变更库存。出库不能动用已预占数量。拒绝需填写原因。库存写权限与审核权限分开控制。

审核和警戒值编辑均比较版本，并支持同键重试；冲突后刷新核对。详细事务与测试结果见 [B2交付记录](docs/development/B2-delivery.md)。
