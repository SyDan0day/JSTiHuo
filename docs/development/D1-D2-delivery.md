# D1/D2 鉴真、预约及原子兑换交付记录

2026-10-14，续接 C2。使用既有数据库表，无 DDL 变更。新增的 OpenAPI CSRF 响应头与地区适配理由见 ADR 实施补充。

## 实现

公开 regions 使用版本化数据和六位父子关系。verifyVoucher 执行 PREAUTH/CSRF、VOUCHER 专用一次性挑战、共享数据库 HMAC 桶限流、等成本未知卡哈希校验；密码原样比较，卡号 trim/大写。锁批次及卡券后复核哈希/凭证版本，消费 PREAUTH 并签发 15 分钟 VOUCHER 会话。可兑换卡为 WRITE，已消费/未激活/未封存/未到期/过期卡为 READ；正确凭证可查状态，但 READ 不能核销。

redemptionOptions 读取冻结规则与当前启用/期限有效的权益商品，redemptionCalendar 根据上海默认提前日、17 点截止、30 天窗口、blackout 和数据库余量提供建议。预览不核销、不占库存、不占预约名额。日历缺失行可补建；已有容量不被预览覆盖。

redeem 校验恰选 N 种、quantity=1、去重/不越冻结集合、完整地址/地区父子关系、预约日期和备注。PII 地址 AES-256-GCM 加密、手机号独立 HMAC 查找值和服务器业务号在锁前准备。事务顺序为幂等键、批次/卡券、日历、商品及库存升序；重放先于当前消费/可兑换性检查。最终重新检查启用/期限/禁配/额度/可用库存，然后一次提交核销、订单/冻结商品名编码属性、ACTIVE 预占、reserved 账本、日额度、卡消费时间、审计、Outbox 与幂等结果。死锁/锁等待同键整事务最多重试三次；ORDER WRITE 会话在提交后补发，失败可原键恢复已成交结果。订单号唯一冲突最多三次重新准备服务器业务号和绑定密文；业务请求五秒超时。

customerOrder 只按当前 ORDER 主体读取脱敏详情，恢复内存 CSRF；不返回详细地址或卡密。凭证版本变更使旧卡券/订单会话无效，但消费本身不撤销原 WRITE 卡券会话，保证响应丢失后同键确认。

Vue 接入 `/verify`、`/redeem`、`/redeem/select`、`/redeem/address`、`/redeem/confirm` 和 `/orders/result`。流程保留内存选择、完整级联地址、预约建议与固定幂等请求，提交后清理 PII；密码提交即清空，不放 URL/store/storage。浏览器刷新重新读取授权选品或脱敏订单，不静默重建一份不同命令。

## 已执行验证与边界

- `GOCACHE=/private/tmp/jst-go-cache make test`：Go race 单元、日历/地区边界、OpenAPI lint/类型同步、37 表一致性、Go/Vue 构建通过。
- `cd backend && GOCACHE=/private/tmp/jst-go-cache go vet ./...`：通过。
- 独立 MySQL 子库 `go test -race -v ./tests/integration/... -run TestRedemption -count=1`：通过，补测后 10.819 秒；预期 KILL CONNECTION 场景出现驱动 unexpected EOF，然后用例验证完整回滚。
- 覆盖真实挑战消费与 PREAUTH 轮换、状态读取不占业务资源、地区级联/不设区城市、N_OF 数量和 quantity 错误、禁配、当前/预约日期校验、地址密文与手机号 HMAC、脱敏订单、ORDER CSRF 恢复。
- 100 次同键并发只返回一个订单；100 个不同键竞争一张卡恰一单成功，其他 ALREADY_REDEEMED。两个剩余卡竞争单件库存/最后一个日名额仅一个成功，reserved 与 ACTIVE reservations 一致、available 非负。
- 在 order、reserve:1、reserve:2、calendar、outbox、commit 注入失败均回滚卡、订单、库存、日额度、Outbox 和幂等结果；commit 前使用 MySQL KILL CONNECTION 真正断开事务连接，没有部分订单/库存。
- 单独验证日容量溢出、库存不足、商品预约日期限、预约日晚于卡有效期仍允许，以及已消费卡可鉴真但只获 READ、不能再兑换。
- HTTP 丢失成功响应后原键 POST 恢复、错误 CSRF 拒绝、顾客作用域响应头、后台 order.read 待发列表/排他分页/脱敏详情及非法筛选均通过。

尚未完成 E1 的独立订单查询、编辑、取消和回收，E2 发货/到店/物流，以及全套浏览器和 14 步主链验收。没有真实发货或通知，不能据本包测试宣称完整系统交付。

最终回归：全新根测试库 `jst_d1_d2_final` 上 `go test -race -v ./tests/integration/... -count=1` 全部通过，54.834 秒。再补充强制真实 order_no 唯一冲突与密文重绑定后，兑换专项重跑通过，11.043 秒；`make test` 和 `go vet ./...` 再次通过。后续 E1/E2 将复用新增订单模块和后台列表。

按用户开发规范，本任务新增 Go 与 Vue/TypeScript 命名函数均补齐中文说明，包含用途、参数、返回值及主要错误/副作用；自动生成类型和原始第三方地区文件不改写。未创建 Git commit。
