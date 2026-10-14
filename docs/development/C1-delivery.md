# C1 卡型、批次与原子生成交付记录

2026-10-14，基于 `746716e`（A1–B2）。新增 C1 实现，使用现有迁移中的卡券与文件表，无 DDL 或 OpenAPI 形状变更。

## 实现范围

- 不可变卡型版本，同名创建递增 revision；批次冻结卡型规则、版本和商品集合。
- 草稿校验：1..1000 张、商品去重且满足 ONE_OF/N_OF、有效期顺序、密码长度白名单、随机尾段至少 8 位、顺序区间溢出检查。数组规范化后计算请求 HMAC。
- 八个真实 API：listVoucherTypes/createVoucherType/listBatches/createBatch/getBatch/generateBatch/listVouchers/getVoucher。读权限 voucher.read，三项写权限 voucher.generate；写入检查 Origin/CSRF、持久化权限和幂等键，生成再次核对数据库会话中的 5 分钟身份复核。
- 独立 `cmd/worker`：只领取数据库 GENERATING 任务，10 分钟租约、30 秒续租、每次领取递增 epoch、崩溃后租约到期可恢复。旧 epoch 或过期租约不能发布。
- 随机模式用密码学随机拒绝采样；批内碰撞最多 5 次，跨库唯一冲突最多 3 次整批尝试；顺序冲突直接失败，不跳号。
- 生产 Argon2id 哈希；CSV 在内存生成，AES-256-GCM 密文写入私有目录、fsync 和原子 rename；AAD 绑定格式、批次和 key version。元数据 TTL 24 小时，卡券默认停用。
- 文件元数据、全部卡券、权益关联、审计和 GENERATED 状态一次事务发布；失败不留下部分卡。提交响应不确定时查询文件引用，再决定是否清理，避免误删已提交材料。进程崩溃孤儿密文在 24 小时宽限后清理，数据库已引用文件保留。
- 后台 `/admin/voucher-types`、`/admin/voucher-batches`、`/admin/voucher-batches/new`、`/admin/voucher-batches/:id`、`/admin/vouchers`。商品选择、卡型版本、草稿生成、失败重试、任务轮询、过滤和分页均使用真实 API，卡密不回显。

## 实际验证

- `GOCACHE=/private/tmp/jst-go-cache make test`：Go race 单元测试、OpenAPI lint/类型生成、37 表一致性、Go 构建、Vue 类型检查和 Vite 构建通过。
- `cd backend && GOCACHE=/private/tmp/jst-go-cache go vet ./...`：通过。
- MySQL 8.4.11 独立 localhost 容器，空测试库上 `go test -race -v ./tests/integration/... -count=1`：全部通过，耗时 38.850 秒（TestCatalog、TestIdentity、TestInventory、TestMigrations、TestVoucherGeneration）。
- 卡券集成：同键重放/摘要冲突、卡型版本及冻结快照、服务端认证与身份复核、撤销写权限、字段缺失/非法筛选、N_OF 集合不足和重复商品拒绝；并发 worker 恰一个领取；三卡真实 Argon2 验密、每卡三项权益、AES-GCM 解密及磁盘密文检查；第二张顺序卡冲突整批回滚及孤儿文件删除；过期租约重领后旧 worker 无法提交、新 worker 成功；已生成批次不重生成；1000 张/3000 权益关联一次原子提交；24 小时孤儿清理不删已引用文件。
- 1000 张事务测试使用低成本 fixture hasher，验证数据库容量和原子发布；生产 Argon2 参数在三卡用例中实际执行。尚未测量生产哈希下 1000 张的耗时。
- T12 的 `[from,until)` 边界由领域单元测试验证；鉴真与兑换有效期执行属于 D1/D2。未进行浏览器截图验收或完整业务 E2E。

## 使用与边界

重新构建并启动：`docker compose -f deployments/compose.yaml up -d --build api worker proxy`。worker 与 API 共用私有 files 卷及独立导出 keyring；不需要新增迁移。任务失败在批次页刷新后重试，重复或网络不确定的请求保留原幂等键。停机中断生成后，租约到期重新领取。

C2 负责受控下载、封存/激活、凭证重置和导出 TTL 自动封存；当前没有下载按钮，不开放卡券兑换。GENERATED 卡券仍为 EXPORTABLE/active=false；activate_on_seal 只是冻结配置，不提前生效。供应商通知仍禁用。
