# C2 安全发行与凭证生命周期

2026-10-14，续接 C1。复用现有表及 OpenAPI，无新增 DDL。

## 实现

实现 downloadFile、sealBatch、sealVoucher、resetSecret、bulkVouchers、editVoucher，并接入后台批次和卡券页面。

凭证下载为带 CSRF 的 POST 二进制响应，要求 file.read、voucher.export、voucher.generate、5 分钟内数据库会话复核及文件发行者归属。下载在批次/单卡/文件授权锁内读取私有密文，检查长度和 SHA256，再按文件 key_version 和绑定主体的 AAD 验证 AES-GCM；验证失败不输出凭证。明文仅驻留内存，不写临时盘或 JSON。文件访问审计在第一个凭证字节发送前独立持久化。下载持锁直到流结束，最长 60 秒；封存与重置等待同一授权锁。返回 no-store、附件头和 application/octet-stream。

批次封存将卡券设为 SEALED，立即撤销旧文件下载权限；activate_on_seal 在封存事务中生效，已删除/已消费/已过期卡不激活。单卡封存仅针对已封存批次中的重置凭证。密文物理删除由持久化 file.destroy 任务执行；删除失败不会恢复下载权限，并在后续 worker 重试。worker 每分钟检查 TTL，批次或单卡到期自动封存，单卡重置凭证到期封存保持停用。

密码重置要求未消费的停用卡、已封存批次、对应权限和最近身份复核。生成新随机密码/Argon2id 哈希，增加凭证版本并撤销对应 VOUCHER/ORDER 会话及关联挑战；旧文件失效，新文件与新哈希在同一事务关联。重试返回原 CredentialDelivery，不保存密码到幂等结果。密码哈希在锁卡前计算，文件失败后仅删除数据库确认未引用的孤儿密文。

批量操作最多 500 张，显式 ID/version 去重并成对排序，统一锁批次再锁卡；激活、停用、软删除或修改有效期一次全成全败。停用已消费卡不撤销已成交订单身份或取消订单。有效期修改撤销相关会话。软删保留唯一卡号，禁止重新使用。审计保留对象 ID 集合、版本、动作和原因，不含秘密。

导出 keyring 保留旧版本读密钥，新写使用 active 版本。补齐 C1 随机字母表：卡号使用全部大写字母数字；DIGITS 使用 0..9；MIXED 使用规格的无易混字母和 2..9。

## 已执行验证

- `GOCACHE=/private/tmp/jst-go-cache make test`：race 单元测试、契约 lint/类型生成、37 表 DDL 一致性、Go/Vue 构建通过。
- `cd backend && GOCACHE=/private/tmp/jst-go-cache go vet ./...`：通过。
- 独立 localhost MySQL 8.4 空库 `go test -race -v ./tests/integration/... -count=1`：全部通过，43.067 秒，包括商品、认证、库存、迁移及 C1/C2 卡券测试。
- 补充密文损坏拒绝、旧版本读密钥与批量审计追踪后，再运行 `go test -race -v ./tests/integration/... -run TestVoucherGeneration -count=1`：通过，23.071 秒。
- 覆盖：未封存拒激活、真实授权 CSV/响应头、发行者归属拒绝、封存后 410、封存自动激活、激活卡拒重置、第二卡过期版本整批回滚、重置同键重放、真实新哈希验密/旧密码失效、会话挑战撤销、单卡封存、密文删除、下载流阻塞封存、TTL 拒绝/自动封存、密文校验失败不输出、旧密钥兼容、有效期编辑、重复 ID 拒绝及下载审计。

## 运行与后续

`docker compose -f deployments/compose.yaml up -d --build api worker proxy`。在批次详情导出并封存，在卡券页执行发行状态与凭证操作；身份复核通过 `/admin/security`。没有公共密钥下载 URL。

D1/D2 尚待接入鉴真和最终兑换事务。完整浏览器验收、物理磁盘与日志全链扫描、备份恢复后的 EXPIRED 重放属于后续 H1/H2，不能据上述 Go/SQL 测试宣称已经完成。真实生产发布仍未执行。
