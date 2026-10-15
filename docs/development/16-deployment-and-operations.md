# 16 部署、运维与恢复

**DESIGN_DECISION**：已实现并在隔离环境验收 deployments/compose.yaml 及生产 HTTPS overlay，包含 proxy、api、worker、mysql 与 migrate/admin 工具 profile。私有 mysql_data/files 卷，秘密只读挂载；MySQL 不映射宿主机端口。API/worker 从同一 Dockerfile 构建，最终 UID 10001、只读根 FS、tmpfs 临时空间；代理 UID 101。开发默认 localhost HTTP，生产必须 TLS。具体命令与边界见 [运维手册](../runbooks/operations.md)，实测见 [H2](H2-delivery.md)。

Go多阶段构建、前端npm ci锁依赖构建静态资源；Vite开发代理/api仅指本地Go；生产proxy先/api到Go，再/admin和公开路由fallback index.html。/media仅READY IMAGE受控文件；IMPORT/CREDENTIAL_EXPORT不能被静态服务器暴露。UI清缓存仅前端版本刷新，不提供任意服务器命令。

## 首次部署顺序

1. 生成独立DB账号和密钥（PII/导出/HMAC/挑战），不使用文档示例作为秘密；配置APP_ENV、APP_ORIGIN、时区、通知DISABLED。
2. 启动MySQL8.4并healthcheck；运维迁移账号执行版本化迁移；检查checksum。
3. CLI 建立 bootstrap 管理员并写入权限目录，安全终端读密码；地区静态数据随版本化代码加载，默认配置与未来日历按业务读取或配置更新惰性创建。
4. 启动api/worker/proxy；readyz通过后运行本地种子E2E；禁止把测试fixture导入生产。
5. 验证未登录拒绝、无公开导出、密钥可读权限、备份解密恢复；随后发布。

## 配置默认值

TZ=UTC（应用业务日显式Asia/Shanghai）；数据库池最多 25 个连接，数据库读写超时 10s；worker 逐任务领取并受租约保护；GENERATION_LEASE=10m；NOTIFICATION_MODE=DISABLED；FILE_ROOT=/var/lib/jst/files；CREDENTIAL_EXPORT_TTL=24h；SESSION_ADMIN_TTL=8h、idle30m。worker 启动及上海跨日后的首次分钟维护补建配置窗口日历（缺省 31 个日期），服务读日历缺失可惰性补齐；配置黑名单/日容量的变更用T-CONFIG。

## 备份恢复

每日全量加密备份，binlog 启用且保留 7 天；全备保留 30 天并有异机副本；目标 RPO<=15 分钟、RTO<=2 小时。隔离小样本已达到目标，生产规模和异机副本仍须复验。认证归档包含 DB、文件元数据与公开图片，排除卡密导出、导入源和报表字节；密钥历史版本分开保管。当前 recovery CLI 单包最多 512 MiB，整包认证后才释放明文，更大规模须交付分片或已验证的物理备份。每月在隔离环境恢复，校验行数/账本/外键/应用版本，再跑只读和自有 E2E。

恢复仅运维CLI/runbook：维护模式、停止写入/worker、保存现场、还原到新库和目录、重放binlog到指定点、清理所有过期导出并撤销session、校验inventory/idempotency/outbox/供应商已处理事件、再切换。禁止后台一键整库恢复/初始化，避免绕过审计与密钥保护。Outbox恢复可能重复，必须靠供应商幂等/UNKNOWN核对。

## 日志与监控

结构slog JSON；指标请求时延/错误码、DB池/死锁、兑换冲突、余额漂移、Outbox积压/DEAD、generation lease过期、磁盘容量及导出未删除。告警：库存差异任一立即、Outbox最老>15分钟、死锁持续>1%、磁盘>80%。审计至少365天；业务订单PII默认完成180天后匿名化（保留状态、账本和不可逆索引删除），开放售后暂停清理；具体生产保留政策可配置但不降低审计可追溯性，法律要求需业务另评估。

升级先兼容迁移后滚动api/worker；有状态规则变更新版本对新批次生效，历史快照不被全局覆盖。回滚应用不自动回滚DDL。性能和恢复演练报告是上线门禁，不以文档存在代替验证。

实际运行：API/代理输出不含 URI、查询、Cookie 和正文的结构日志；worker 每 15 分钟只读对账并记录队列、租约、磁盘和数据库指标。完成订单默认超过 180 天后每小时最多匿名化 1000 单，活动售后暂停，清空手机索引、收件资料和备注、撤销本单会话，保留订单、账本及审计；保留期可用 ORDER_PII_RETENTION_DAYS 增加。外部告警采集、集中日志与异机备份调度为部署责任，本地未连接真实值班渠道。
