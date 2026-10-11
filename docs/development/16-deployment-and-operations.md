# 16 部署、运维与恢复

**DESIGN_DECISION**：本轮只交付规格；没有创建运行Compose或业务代码。A1实施docker-compose.yml包含proxy、api、worker、mysql、migrate（profile tools），私有mysql_data/files卷，secret文件外部只读挂载；仅proxy暴露HTTPS，MySQL不映射公网。API/worker同镜像，非root、只读根FS、tmpfs临时空间；files独立最小权限。开发模式localhost端口可独立例外，生产必须TLS。

Go多阶段构建、前端npm ci锁依赖构建静态资源；Vite开发代理/api仅指本地Go；生产proxy先/api到Go，再/admin和公开路由fallback index.html。/media仅READY IMAGE受控文件；IMPORT/CREDENTIAL_EXPORT不能被静态服务器暴露。UI清缓存仅前端版本刷新，不提供任意服务器命令。

## 首次部署顺序

1. 生成独立DB账号和密钥（PII/导出/HMAC/挑战），不使用文档示例作为秘密；配置APP_ENV、APP_ORIGIN、时区、通知DISABLED。
2. 启动MySQL8.4并healthcheck；运维迁移账号执行版本化迁移；检查checksum。
3. CLI建立bootstrap管理员，安全终端读密码；CLI导入版本化地区静态数据与权限目录、默认配置、未来31天日历。
4. 启动api/worker/proxy；readyz通过后运行本地种子E2E；禁止把测试fixture导入生产。
5. 验证未登录拒绝、无公开导出、密钥可读权限、备份解密恢复；随后发布。

## 配置默认值

TZ=UTC（应用业务日显式Asia/Shanghai）；DB_POOL_MAX=25，APP_HTTP_TIMEOUT=10s；WORKER_BATCH=20；GENERATION_LEASE=10m；NOTIFICATION_MODE=DISABLED；FILE_ROOT=/var/lib/jst/files；CREDENTIAL_EXPORT_TTL=24h；SESSION_ADMIN_TTL=8h、idle30m。每天午夜生成未来31日历，服务读日历缺失可惰性补齐；配置黑名单/日容量的变更用T-CONFIG。

## 备份恢复

每日物理/逻辑全量加密备份，binlog启用且保留7天；全备保留30天，异机副本；目标RPO<=15分钟、RTO<=2小时，属待演练目标。备份包含DB、私有文件元数据/必要文件、密钥版本备份（分开保管），导出凭证文件不纳入常规长期备份。每月在隔离环境恢复，校验行数/账本/外键/应用版本，再跑只读和自有E2E。

恢复仅运维CLI/runbook：维护模式、停止写入/worker、保存现场、还原到新库和目录、重放binlog到指定点、清理所有过期导出并撤销session、校验inventory/idempotency/outbox/供应商已处理事件、再切换。禁止后台一键整库恢复/初始化，避免绕过审计与密钥保护。Outbox恢复可能重复，必须靠供应商幂等/UNKNOWN核对。

## 日志与监控

结构slog JSON；指标请求时延/错误码、DB池/死锁、兑换冲突、余额漂移、Outbox积压/DEAD、generation lease过期、磁盘容量及导出未删除。告警：库存差异任一立即、Outbox最老>15分钟、死锁持续>1%、磁盘>80%。审计至少365天；业务订单PII默认完成180天后匿名化（保留状态、账本和不可逆索引删除），开放售后暂停清理；具体生产保留政策可配置但不降低审计可追溯性，法律要求需业务另评估。

升级先兼容迁移后滚动api/worker；有状态规则变更新版本对新批次生效，历史快照不被全局覆盖。回滚应用不自动回滚DDL。性能和恢复演练报告是上线门禁，不以文档存在代替验证。
