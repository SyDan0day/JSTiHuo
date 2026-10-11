# A1 开发交付记录（2026-10-12）

范围：Go API/迁移 CLI，Vue 两套入口布局及懒加载路由，OpenAPI 生成类型/客户端，分依赖迁移，Compose 与 CI。未实现领域业务，不修改研究原件。首次初始化本地 Git，无自动提交/推送。

数据库：权威 schema.sql 的 37 表及约束拆分为六份 SQL，最后一份全部外键；独立 schema_migrations 与 checksum/GET_LOCK 保护，禁止启动自动迁移。无领域字段变更。OpenAPI 业务契约无改动；运维探针在业务契约之外。

已执行并通过：Go race 单元测试（配置缺失、坏 keyring、health/readiness、错误脱敏、405/404）；Go 全工程构建；OpenAPI 规范验证及 TS 类型生成；vue-tsc/Vite 生产构建；DDL 全语句一致性（37 表）；Compose 配置解析。

尚未通过：本机 Docker daemon 未运行，无法执行真实 MySQL 空库迁移/集成测试及 Compose 启动；已提供隔离测试与 CI。CI 未在远程执行。镜像 digest、运行 UI 截图与生产恢复演练未完成。A1 尚不满足完整运行验收，A2 前应先跑 MySQL 测试。worker 随实际异步任务接入，不放置假成功任务实现。

运行说明与命令见项目根 README.md。

## Docker 复测（2026-10-12）

Docker 29.4.0 可用；MySQL 8.4 健康，完整迁移容器执行成功。真实隔离库 Go race 集成测试通过：空库迁移、重复执行、checksum 篡改拒绝。修复非 root 应用读取 0600 密钥失败：启动时仅复制当前命令所需密钥至 tmpfs 后降权运行，不复制 root 密码或账号 SQL。前述 Docker 不可用记录为首次测试状态。

整套 Compose 已构建并启动：mysql healthy，api/proxy Running；API 主进程 UID=10001。通过 localhost:8080 验证 /healthz、/readyz 返回 200/status=ok，首页与 /admin 返回 HTTP 200。数据库 37 张领域表 + schema_migrations，共 38 表，6 条迁移记录。初次镜像认证超时经显式 docker pull 重试解决。测试 loopback 数据库端口已随正常 Compose 启动关闭。基础运行验收已补齐；浏览器视觉 QA、镜像 digest 固定及生产恢复仍未验收。
