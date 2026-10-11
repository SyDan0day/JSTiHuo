# A2 身份会话与基础 RBAC（2026-10-12）

实现范围：bootstrap、createChallenge、adminLogin、adminSession、adminLogout、adminReauth、changePassword；离线初始管理员 CLI；登录、首次改密、复核页面及路由守卫。

安全实现：Argon2id（64MiB/3次/1并行）、数据库共享限流、图片挑战 HMAC、opaque cookie SHA256、CSRF/Origin 校验、8h绝对及30m闲置有效期、权限逐请求读取、凭证版本及启用状态检查。成功身份变更与脱敏审计同事务；拒绝单独审计。

没有改动领域 DDL 或业务 OpenAPI。复用已有身份、权限、会话、挑战、限流、幂等与审计表。初始化 CLI 只允许空管理员库并锁 identity_guard；SetEnabled 服务要求 admin.write 和近期复核，锁 guard，保护最后同时拥有 admin.write/role.write 的启用管理员。管理员/角色 HTTP 编辑属于 G3，本包未开放对应接口。安全实施补充见12文档。

已执行并通过：make test（Go race 单元、OpenAPI校验及类型同步、37表DDL一致性、Go/Vue构建）；真实MySQL8.4隔离库 TestIdentity + TestMigrations。覆盖 CSRF/错误Origin、挑战一次消费/5次上限、未知账号一致错误与第6次限流、强制改密、权限集合、复核旋转、旧会话失效、账号禁用/版本撤销、最后管理员保护、改密幂等重放/换正文409/仅一次审计、过期PREAUTH。

Compose 应用及管理员 CLI 镜像已构建，登录页面在浏览器中显示正常，验证码来自实际 API。没有预置业务库账号或口令；用户需通过 CLI 选择自己的初始密码。没有推送或生产部署。

最终运行验证：API、proxy、MySQL 正常运行；/readyz 返回 status=ok；匿名 GET /api/v1/admin/session 返回401及统一错误结构。临时 MySQL 13306 测试映射已关闭，正常 Compose 仅暴露 localhost:8080。最终隔离库认证/迁移 race 测试通过（17秒），前后端构建通过。浏览器实际验证了登录表单、标签和验证码显示；未用业务库预置口令执行浏览器改密。
