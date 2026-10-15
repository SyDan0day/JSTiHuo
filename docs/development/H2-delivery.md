# H2 部署备份恢复与交接

2026-10-15 完成隔离本地部署、加密恢复和应用回滚演练。未执行真实生产发布，未连接真实通知/面单供应商。操作步骤见 [运维手册](../runbooks/operations.md)，运营和顾客使用见 [业务指南](../runbooks/user-guide.md)。

## 已实现内容

- Go/Node/Alpine/nginx/MySQL 基础镜像使用明确 digest；API 镜像同时包含 migrate/admin/worker/recovery 工具。发布清单记录本地镜像 ID、架构及源码/锁文件/迁移摘要，见 [release-manifest.json](evidence/h2/release-manifest.json)，没有上传镜像仓库。
- 独占创建秘密目录及文件，分别 0700/0600，不覆盖已有配置、不输出秘密。数据库迁移独立执行，业务账号只有 DML，无 DDL；MySQL 不暴露宿主机端口。
- 显式 HTTPS overlay、生产 Cookie/HSTS/CSP、代理 UID 101、API/worker UID 10001、只读根文件系统和 tmpfs。修正降权后的文件卷权限设置，以及 `/var/run` tmpfs 遮蔽证书目录的问题；证书挂载 `/etc/nginx/tls`。
- 仅信任显式代理 CIDR 的客户端 IP；应用和代理日志不记录 URI、查询、正文、Cookie 或秘密。worker 启动和上海跨日后自动补建配置窗口日历，失败于下次分钟维护重试；输出队列、租约、DB 池、死锁、磁盘与库存差异指标及阈值告警日志。
- 完成订单超过 180 天后按受控保留期匿名化，活动售后暂停，订单锁内重新检查案件，多个 worker 只处理一次；清空收件人/备注和手机索引，撤销本单会话，保留账本与审计。
- `recovery` CLI：keygen、pack、unpack、unpack-sql、sanitize、verify-db。归档采用独立 AES-256-GCM 密钥、随机 nonce 和严格 tar 路径白名单，完整认证后才释放明文；拒绝覆盖恢复目录。最多 512 MiB 的有界内存归档，超限失败。
- 全备包含 DB 和公开图片，排除私有卡密、导入源及报表字节；密钥历史版本分开保管。恢复安全收敛事务撤销会话、过期私有文件、封存生成批次并隔离 UNKNOWN 外部任务；审计失败整体回滚，不修改订单和库存。

## 实际演练

Compose 独立项目 `codex-jst-h2`，MySQL 8.4.11。先从空库迁移，创建最小账号，通过交互无回显 CLI 初始化管理员，再从真实浏览器强制改密、重新登录、复核，创建一个自有商品 PNG、一条加密联系人反馈及一个未激活生成批次。无预置业务账号或测试夹具导入。

本地 HTTP 启动后切到生产 HTTPS 配置，监听 localhost:28443；自签证书由 curl --cacert 验证，浏览器同时检查 Secure/HttpOnly/SameSite=Strict/Path=/ 的 __Host Cookie。浏览器为本地自签证书启用忽略证书错误，单独的 curl 验证不忽略证书。截图见 [evidence/h2](evidence/h2)，最终进程 UID、只读根目录与私有 MySQL 检查见 [runtime-verification.json](evidence/h2/runtime-verification.json)。

停止写入与 worker 后生成认证全备，记录 binlog 位点，再提交一条全备后自有内容记录。使用同版本官方 mysqlbinlog 提取声明的 ROW 窗口，并以独立认证归档保存增量；恢复到随机新库与独立新文件卷。全备单独恢复时该记录为零条，重放增量后为一条，证明实际消费了增量而非仅复制最终库。

|检查|实测结果|
|---|---|
|全备密文|85,030 字节|
|增量密文|5,158 字节|
|binlog 窗口|mysql-bin.000004，114471→114979|
|全表行数|38 张一致（37 业务表 + schema_migrations，安全收敛前）|
|公开图片|1 个，恢复前后 SHA-256 一致，新卷可通过 HTTP 读取|
|必要 PII|1 条反馈认证解密通过，仅输出计数|
|私有文件/批次|旧私有下载过期，GENERATED 批次封存，浏览器验证通过|
|旧会话|安全收敛后零条，必须重新登录|
|库存|balanced=true，无漂移|
|错误密钥/篡改|拒绝，目标目录未创建，无部分明文发布|
|指定点 RPO|0 秒；仅针对声明的冻结窗口|
|服务 RTO|7.526 秒，包含解包、导入、增量、安全收敛、新卷切换及 readyz|
|恢复后浏览器|额外 3.192 秒验证，真实登录、图片、反馈和凭证封存均通过|
|回滚|保留原库原卷，切回基线镜像；readyz 1.836 秒，含浏览器检查 4.062 秒|

完整非敏感记录见 [recovery-report.json](evidence/h2/recovery-report.json)。演练数据仅 1 商品/1 反馈/1 卡，零订单；订单 PII、账本及恢复隔离另由真实订单 `TestRecoverySafety` 覆盖。不能把小样本 RTO 当作十万订单恢复耗时；浏览器检查与 readiness 时间分别记录，没有把人工操作间隔算成连续自动恢复。

## 自动化与交付校验

最终 MySQL race 与 106 操作真实 HTTP 契约全量通过，见 H1。`TestRecoverySafety` 还验证错误目标库名拒绝、审计故障整体回滚、订单及反馈 PII 正确/错误密钥、会话撤销、LABEL UNKNOWN、通知 UNKNOWN/DEAD 和库存不变。归档单元验证随机 nonce、错误密钥、篡改、截断及路径穿越。

`make test`、Go vet、Go/Vue 构建、基础镜像 digest 构建、HTTPS 浏览器与回滚均实际执行。CI 已配置每日、手动、push/PR 的回归，包含恢复安全测试；本轮没有推送或触发远端 CI。外部备份与集中告警尚未部署，由操作手册明确频率和责任，不把静态配置当作正在运行的服务。

## 生产边界与交接

本地开发与文档规定的 MANUAL/MOCK 验收已完成。真实域名/可信证书及续签、真实供应商契约与凭据、异机备份调度、集中日志告警、实际生产数据量恢复与压力复验，需部署环境落实。未向真实收件人发送消息，未自动上线、创建 commit 或 push。

本轮已清理自建容器、临时库、测试进程、解密恢复 SQL 与私有临时秘密；保留源码、非敏感截图、验收报告和本地镜像。未删除用户原有环境或执行 Git 清理，见 [清理记录](evidence/h2/cleanup-report.json)。
