# 部署、备份恢复与故障处理

适用版本：A1–H2 本地交付（2026-10-15）。本地验收记录见 [H2](../development/H2-delivery.md)。以下命令需在仓库根目录执行；`/secure/...` 表示运维预建的受限绝对路径，不能原样作为实际秘密或生产地址使用。

## 首次部署

需要 Docker Compose v2（支持 `!override`）、Go 1.26、Node 24；构建所用基础镜像 digest 已锁定。生产准备独立域名、可信证书及定期更新机制；本地自签证书只用于隔离演练。

1. `python3 deployments/init-local.py --directory /secure/jst-secrets`。目录必须为空，生成目录 0700、文件 0600，不打印秘密；部分失败保留现场检查，不能覆盖重生成。
2. 设置 `JST_SECRETS_DIR` 为该目录。启动 `docker compose -f deployments/compose.yaml up -d mysql`，等待 healthy。
3. `docker compose -f deployments/compose.yaml --profile tools run --rm migrate`。迁移检查 checksum，不在 API 启动时自动执行。DDL 部分失败先保留现场，不能清库重跑。
4. 创建最小业务账号：`docker compose -f deployments/compose.yaml exec -T mysql sh -c 'MYSQL_PWD=$(cat /run/secrets/db_root) mysql -uroot jstihuo < /run/secrets/app-user.sql'`。仅初次执行；账号只有本库 SELECT/INSERT/UPDATE/DELETE。
5. 构建管理工具后，在可信交互终端执行 `docker compose -f deployments/compose.yaml --profile tools run --rm admin admin --login-name 实际管理员名称`。密码两次无回显输入，禁止通过命令行、脚本输出或截图传递。CLI 只允许空管理员库；首次登录强制改密并撤销旧会话。
6. 本地启动 `docker compose -f deployments/compose.yaml up -d --build api worker proxy`。访问 localhost:8080；先检查 `/readyz`，再按业务指南创建自有内容和商品。地区、权限及默认配置来自版本化代码；worker 启动及上海跨日补建日历，不导入浏览器测试夹具。

## HTTPS 与运行边界

生产叠加 `-f deployments/compose.production.yaml`，配置 `APP_ORIGIN=https://实际域名`、`JST_TLS_DIR=/secure/tls`、`JST_HTTPS_PORT=443`、`JST_HTTPS_BIND=实际监听地址`。默认只绑定 127.0.0.1；有上游入口时按网络拓扑设置。证书文件为 `fullchain.pem` / `privkey.pem`，容器只读挂载 `/etc/nginx/tls`；密钥仅 UID 101 可读，勿改为全员可读。macOS Docker Desktop 可用独立 named volume 准备证书属主；Linux 可直接设置 bind 目录权限。

API/worker 运行 UID 10001、根文件系统只读，临时秘密复制到 tmpfs 后降权；文件卷目录 0700。代理 UID 101、只读根文件系统、无 capabilities。MySQL 不映射宿主机端口。API 只能经可信入口访问；`JST_TRUSTED_PROXY_CIDRS` 必须按实际代理网段收窄，Compose 缺省 Docker 私网 172.16.0.0/12，非此拓扑必须覆盖。入口覆盖 `X-Real-IP`，非信任来源伪造头不会改变限流来源。

检查可信 TLS 的 `/readyz`，登录后 Cookie 应为 `__Host-jst-admin`、Secure/HttpOnly/SameSite=Strict、Path=/。不要用跳过证书检查代替生产 TLS 验证。CSP、HSTS、nosniff 已由生产代理配置。`healthz` 只表示进程存活；`readyz` 检查数据库和迁移，运维需监控两者。

通知缺省 DISABLED，轨迹仅显式 MOCK 模拟；MANUAL 不伪造签收。没有真实供应商契约时，LIVE 面单明确拒绝。可选轨迹回调需单独配置 `TRACKING_WEBHOOK_KEYS_FILE`（provider→Base64 独立密钥），供应商契约和可信来源经验证后方可接入。

## 备份与密钥保管

在 backend 模块目录执行 `go build -o /受控工具目录/recovery ./cmd/recovery`；运行镜像也包含 `/usr/local/bin/recovery`。将校验过的工具和对应镜像 ID 纳入发行清单。

`recovery keygen --key-file /独立密钥目录/backup.key` 创建一次性 0600 AES-256 备份密钥。备份密钥与备份异机、分开保管；PII 全部历史版本、lookup/request/challenge HMAC、必要的 export 密钥环也独立备份，不能放入常规归档。丢失 PII 或 lookup 密钥不能靠重新生成修复。

全备包含数据库和公开图片；IMPORT、CREDENTIAL_EXPORT、REPORT 字节及秘密文件排除。文件元数据仍在数据库中，恢复后必须安全收敛。单归档含 SQL、图片及 tar 头最多 512 MiB，超限失败；当前工具整包内存认证，不适合直接声称支持任意大库。生产超过此规模须先交付分片/物理备份方案并演练。

每日全量、binlog 至少每 15 分钟安全转存一次，保留 binlog 7 天、全备 30 天与异机副本。此频率为运行责任，仓库没有替用户部署外部备份调度。调度必须检查失败和最后成功时间，不能仅看文件存在。停止写入和 worker 后，冻结数据库与文件的同一恢复点；保留原库原卷。

在安全 shell 中启用 `set -euo pipefail` 和 `umask 077`。源库可用 `mysqldump --single-transaction --no-tablespaces --set-gtid-purged=OFF --skip-triggers jstihuo`，凭据通过受限文件或容器 secret，不用命令行明文。将 SQL stdout 直接管道到 `recovery pack --key-file /独立密钥目录/backup.key --files-root /受限图片快照目录`，先写新临时密文文件；仅在全部命令成功后原子更名发布。备份数据库必须同时含 `schema_migrations`；冻结期间记录 `SHOW BINARY LOG STATUS` 的文件和 position。图片快照只复制受控根目录的 `<32位hex>.png`。

binlog 从全备位点至恢复点形成连续窗口。使用相同 MySQL 8.4 版本的官方 `mysqlbinlog --start-position=... --stop-position=...`，输出也经 `recovery pack` 独立加密；不要将原始 binlog/SQL 长期明文保存。跨 binlog 文件按顺序覆盖完整窗口，验证无缺口。库名变化时，ROW 事件可使用 `--rewrite-db=源库->目标库`；本次演练仅有冻结后的受控 ROW 写入，没有 DDL。生产有 DDL/跨库事件时须在隔离 MySQL 同名库重放，不能假设该参数会重写任意 SQL。

## 隔离恢复与切换

1. 维护窗口停止写入和 worker；保存故障现场及原库/卷，记录最后成功备份和目标事务点。创建新数据库和新文件卷，不覆盖原库。
2. `recovery unpack --key-file /独立密钥目录/backup.key --target-dir /受限目录/新恢复目录 < full.jstb`。目标必须不存在；认证失败不发布任何明文。该目录 0700、文件 0600，SQL 导入后及时清理。或用 `unpack-sql` 在完整认证后直接管道导入新库。
3. 导入全备，再按验证过的窗口重放 binlog 增量至目标点。核对全表行数、唯一约束、外键、库存流水和预占；图片按原 storage_key 恢复到新卷并比对 SHA-256，属主 10001、目录 0700、文件 0600。
4. 准备新库专用 DSN 文件 0600。执行 `recovery sanitize --dsn-file /受限目录/target-dsn --restored-database 精确新库名 --pii-keyring-file /独立密钥目录/pii.json`。目标库名不匹配立即拒绝。审计失败整体回滚；再次执行安全收敛可重复。
5. 确认 sessions/challenges 已撤销、所有非图片 STAGED/READY 已过期、GENERATED 批次已封存；未确认面单变 UNKNOWN，潜在已发送通知变 UNKNOWN/DEAD，禁止自动重发。人工按供应商幂等号与证据核对。原订单、库存、账本不由恢复工具补账或回滚。
6. `recovery verify-db --dsn-file /受限目录/target-dsn --pii-keyring-file /独立密钥目录/pii.json`：要求 balanced=true、无漂移且 PII 条数符合预期。错 PII 密钥必须失败；仅迁移 checksum 或库存正确不足以证明资料可解密。
7. 给业务账号授新库最小 DML 权限；将 app_dsn 和文件卷显式切至新目标，重建 api/worker/proxy 使其读入新秘密。检查 readyz、真实登录、商品图片、脱敏订单及反馈、旧会话拒绝和私有文件不可下载；记录 RPO/RTO 与样本规模后再结束维护。
8. 故障时停新 worker，切回保留的原 DSN、原卷和验证过的应用镜像；重复探针和真实读取。发生新业务写入后必须先协调数据，不能直接回切丢失新单。不要自动回滚 DDL 或执行 down -v。

每月实际演练完整恢复；CI 的每日单元/恢复安全回归不替代全备、异机副本和真实恢复点演练。

## 监控与故障处理

API slog 记录 request_id、固定路由类别、状态、耗时及字节数；代理只记录固定方法、状态、字节和耗时。均不记录 URI、查询、cookie、正文、原供应商响应或秘密。代理 error_log 为 crit；使用请求标识和应用固定错误排查。集中日志保留至少 365 天，限制读取并验证外部采集与告警链。

worker 启动和每 15 分钟记录 Outbox 数量/DEAD/最老待执行年龄、过期任务租约、DB 池、磁盘、事务与死锁计数；磁盘 >80%、Outbox >15 分钟、窗口内至少 100 次事务且死锁占比 >1% 输出 WARN。定时库存对账差异输出 ERROR，后台库存页面可即时只读对账。告警输出已实现，外部值班渠道需部署方接通。

- 库存差异：停止相关写入，保存对账商品 ID、账本和业务证据；只读定位原因，禁止直接改余额“对平”。定时对账最坏 15 分钟发现，不能宣称实时逐事务告警。
- Outbox/租约：先看配置、DB 与 worker 状态。只有明确未执行的失败可重试；UNKNOWN 必须近期复核并提交外部证据，不能手动改表为 PENDING。
- 下载过期：重新验证发行流程；封存后卡密不可从备份恢复下载。凭证重置会撤销旧卡会话，必须安全重新交付。
- 磁盘与孤儿文件：先检查 TTL worker；清理仅按受控文件类型/数据库引用，不通过 Web 任意删除或执行命令。
- 数据库不可用：readiness 失败，先恢复数据库并检查迁移；不要关闭 checksum 检查绕过。

订单收件资料默认完成超过 180 天后每小时按最多 1000 单匿名化，OPEN/APPROVED 售后暂停。`ORDER_PII_RETENTION_DAYS` 可设 180–36500 天；更短配置拒启动。清空收件人/备注和手机索引，撤销本单会话，保留订单状态、商品与账本、匿名化审计；不再允许对已匿名化订单新开售后。需要法律保留时应提前增加期限并限制访问；已匿名化资料不可恢复为可查询联系人。

## 升级与密钥轮换

发布前记录应用镜像 ID/digest、源码/迁移校验和、依赖锁文件与配置版本。先验证兼容迁移，再切 api/worker，proxy 静态资源同步发布；历史已发行权益快照保持不变。回滚只切验证过的镜像及兼容数据，不能自动反向 DDL。

PII/export 轮换：保留旧版本，添加新随机版本并设置 active，api/worker 同步重启；验证新写版本和旧数据读取，再按完整备份/数据生命周期决定能否退役旧版本。lookup/request/challenge HMAC 当前为单密钥，不能无计划在线替换；须维护窗口撤会话并评估已有手机号索引、幂等和验证码。生产使用独立秘密存储和安全轮换流程，避免旧备份随密钥丢失而不可恢复。
