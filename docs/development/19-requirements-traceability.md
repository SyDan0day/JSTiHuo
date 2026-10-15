# 19 需求—证据—实现追踪

全表实施映射为 **DESIGN_DECISION**。E01–E12见[调查基线](01-research-baseline.md)；M01–M52见[后台矩阵](11-admin-design.md)；T01–T22见[测试](15-testing-and-acceptance.md)。A1–H2 实际业务、真实 MySQL 与浏览器结果见各包 delivery.md，最终验收见 H1/H2。

每个API路径/页面/权限/输入输出的权威字段在[OpenAPI](openapi.yaml)，本表连接Service、领域表与验收。表集合表示该流程可能参与的表，不表示每个GET都会写所有表；具体事务边界见04/13及x-transaction。所有写操作还涉及audit_logs；幂等业务操作涉及idempotency_keys，异步业务涉及outbox_events。

## 领域覆盖

|流程|需求/证据|Service|领域表|测试|包|
|---|---|---|---|---|---|
|F00|R10；E04/E10|identity|admin_users, roles, permissions, admin_user_roles, role_permissions, sessions, challenges, system_configs|T01/T18|A2/G3|
|F01|R01/R02；E01/E05|catalog|products, categories, product_media, files, inventory_balances, content_pages|T03/T20|B1/F1|
|F02|R03；E06|voucher|voucher_types, voucher_batches, vouchers, voucher_products, files, sessions|T02/T04/T12/T15|C1/C2|
|F03|R03/R10；E02/E06|identity/voucher|vouchers, voucher_products, products, sessions, challenges, rate_limits|T01/T12/T15/T18|A2/C2|
|F04|R04；E02/E06/E08|redemption|vouchers, voucher_products, voucher_redemptions, orders, order_items, shipping_calendars, inventory_balances, inventory_reservations, inventory_transactions, sessions|T04/T05/T06/T10/T11/T12/T13|D1/D2|
|F05|R05；E07|order|orders, order_items, voucher_redemptions, vouchers, shipping_calendars, inventory_balances, inventory_reservations, inventory_transactions, sessions|T07/T11/T14/T19|E1|
|F06|R06；E08|inventory|inventory_balances, inventory_documents, inventory_document_items, inventory_transactions, inventory_reservations|T06/T08/T22|B2/H1|
|F07|R07/R04；E08/E07|shipping/redemption|orders, order_items, vouchers, voucher_redemptions, inventory_balances, inventory_reservations, inventory_transactions, shipments, shipment_events, carriers, integration_jobs|T09/T17|E2/G2|
|F08|R08；E07|order/shipping|orders, order_items, after_sales, shipments, inventory_balances, inventory_transactions|T16/T17|G2|
|F09|R09；E01/E04|operations|feedback, orders|T18/T20|F2/G2|
|F10|R12/R10；E04/E10|operations/identity|system_configs, shipping_calendars, notifications, outbox_events, audit_logs, admin_users, roles, permissions, admin_user_roles, role_permissions|T01/T11/T17/T22|G3/H2|
|F11|R11；E06/E08/E10|operations/voucher/shipping|files, integration_jobs, voucher_batches, vouchers, voucher_products, orders, shipments, inventory_balances, inventory_reservations, inventory_transactions|T15/T21|G1|

## API逐操作落点

Handler 实际位于 backend/internal/transport/http，Service 位于 backend/internal/app 各领域模块；以下模块中的斜线代表跨服务协调，使用同一个事务，不能分别提交。每项须执行 OpenAPI x-tests 的 OK/DENY/BOUNDARY 契约用例；H1 已记录 106 操作真实成功响应与适用安全边界。

|operationId|前端页面|Service/流程|事务|测试及实施包|
|---|---|---|---|---|
|bootstrap|`/verify`|identity/voucher / F03|T-AUTH|T01/T12/T15/T18；A2/C2|
|createChallenge|`/verify`|identity/voucher / F03|T-AUTH|T01/T12/T15/T18；A2/C2|
|adminLogin|`/admin/login`|identity / F00|T-AUTH|T01/T18；A2/G3|
|adminSession|`/admin`|identity / F00|T-READ|T01/T18；A2/G3|
|adminLogout|`/admin`|identity / F00|T-AUTH|T01/T18；A2/G3|
|adminReauth|`/admin/security`|identity / F00|T-AUTH|T01/T18；A2/G3|
|publicProducts|`/products`|catalog / F01|T-READ|T03/T20；B1/F1|
|publicContent|`/content`|catalog / F01|T-READ|T03/T20；B1/F1|
|regions|`/regions`|catalog / F01|T-READ|T03/T20；B1/F1|
|publicProduct|`/products/:id`|catalog / F01|T-READ|T03/T20；B1/F1|
|publicContentPage|`/pages/:slug`|catalog / F01|T-READ|T03/T20；B1/F1|
|verifyVoucher|`/verify,/redeem`|identity/voucher / F03|T-AUTH|T01/T12/T15/T18；A2/C2|
|redemptionOptions|`/redeem/select`|redemption / F04|T-READ|T04/T05/T06/T10/T11/T12/T13；D1/D2|
|redemptionCalendar|`/redeem/address`|redemption / F04|T-READ|T04/T05/T06/T10/T11/T12/T13；D1/D2|
|redeem|`/redeem/confirm`|redemption / F04|T-REDEEM|T04/T05/T06/T10/T11/T12/T13；D1/D2|
|queryOrder|`/orders/query`|order / F05|T-AUTH|T07/T11/T14/T19；E1|
|orderStepUp|`/orders/edit`|order / F05|T-AUTH|T07/T11/T14/T19；E1|
|customerOrder|`/orders/result`|order / F05|T-READ|T07/T11/T14/T19；E1|
|customerOrderEdit|`/orders/edit`|order / F05|T-EDIT|T07/T11/T14/T19；E1|
|customerShipments|`/orders/tracking`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|customerTracking|`/orders/tracking`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|customerAfterSale|`/orders/after-sales`|order/shipping / F08|T-STATE|T16/T17；G2|
|customerAfterSales|`/orders/after-sales`|order/shipping / F08|T-READ|T16/T17；G2|
|createFeedback|`/feedback`|operations / F09|T-STATE|T18/T20；F2/G2|
|listProduct|`/admin/products`|catalog / F01|T-READ|T03/T20；B1/F1|
|getProduct|`/admin/products/:id`|catalog / F01|T-READ|T03/T20；B1/F1|
|createProduct|`/admin/products/new`|catalog / F01|T-CRUD|T03/T20；B1/F1|
|updateProduct|`/admin/products/:id/edit`|catalog / F01|T-CRUD|T03/T20；B1/F1|
|listCarrier|`/admin/carriers`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|getCarrier|`/admin/carriers/:id`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|createCarrier|`/admin/carriers/new`|shipping/redemption / F07|T-CRUD|T09/T17；E2/G2|
|updateCarrier|`/admin/carriers/:id/edit`|shipping/redemption / F07|T-CRUD|T09/T17；E2/G2|
|listAdmin|`/admin/admins`|identity / F00|T-READ|T01/T18；A2/G3|
|getAdmin|`/admin/admins/:id`|identity / F00|T-READ|T01/T18；A2/G3|
|createAdmin|`/admin/admins/new`|identity / F00|T-CRUD|T01/T18；A2/G3|
|updateAdmin|`/admin/admins/:id/edit`|identity / F00|T-CRUD|T01/T18；A2/G3|
|listRole|`/admin/roles`|identity / F00|T-READ|T01/T18；A2/G3|
|getRole|`/admin/roles/:id`|identity / F00|T-READ|T01/T18；A2/G3|
|createRole|`/admin/roles/new`|identity / F00|T-CRUD|T01/T18；A2/G3|
|updateRole|`/admin/roles/:id/edit`|identity / F00|T-CRUD|T01/T18；A2/G3|
|listContent|`/admin/content`|catalog / F01|T-READ|T03/T20；B1/F1|
|getContent|`/admin/content/:id`|catalog / F01|T-READ|T03/T20；B1/F1|
|createContent|`/admin/content/new`|catalog / F01|T-CRUD|T03/T20；B1/F1|
|updateContent|`/admin/content/:id/edit`|catalog / F01|T-CRUD|T03/T20；B1/F1|
|productStatus|`/admin/products`|catalog / F01|T-STATE|T03/T20；B1/F1|
|categories|`/admin/products`|catalog / F01|T-READ|T03/T20；B1/F1|
|createCategory|`/admin/products`|catalog / F01|T-CRUD|T03/T20；B1/F1|
|voucherTypes|`/admin/voucher-types`|voucher / F02|T-READ|T02/T04/T12/T15；C1/C2|
|createVoucherType|`/admin/voucher-types`|voucher / F02|T-CRUD|T02/T04/T12/T15；C1/C2|
|batches|`/admin/voucher-batches`|voucher / F02|T-READ|T02/T04/T12/T15；C1/C2|
|createBatch|`/admin/voucher-batches/new`|voucher / F02|T-CRUD|T02/T04/T12/T15；C1/C2|
|getBatch|`/admin/voucher-batches/:id`|voucher / F02|T-READ|T02/T04/T12/T15；C1/C2|
|generateBatch|`/admin/voucher-batches/:id`|voucher / F02|T-GENERATE|T02/T04/T12/T15；C1/C2|
|sealBatch|`/admin/voucher-batches/:id`|voucher / F02|T-SEAL|T02/T04/T12/T15；C1/C2|
|vouchers|`/admin/vouchers`|voucher / F02|T-READ|T02/T04/T12/T15；C1/C2|
|getVoucher|`/admin/vouchers/:id`|voucher / F02|T-READ|T02/T04/T12/T15；C1/C2|
|editVoucher|`/admin/vouchers/:id`|voucher / F02|T-STATE|T02/T04/T12/T15；C1/C2|
|bulkVouchers|`/admin/vouchers`|voucher / F02|T-BULK|T02/T04/T12/T15；C1/C2|
|resetSecret|`/admin/vouchers/:id`|voucher / F02|T-SEAL|T02/T04/T12/T15；C1/C2|
|sealVoucher|`/admin/vouchers/:id`|voucher / F02|T-SEAL|T02/T04/T12/T15；C1/C2|
|orders|`/admin/orders`|order / F05|T-READ|T07/T11/T14/T19；E1|
|getOrder|`/admin/orders/:id`|order / F05|T-READ|T07/T11/T14/T19；E1|
|privateOrder|`/admin/orders/:id`|order / F05|T-STATE|T07/T11/T14/T19；E1|
|adminOrderEdit|`/admin/orders/:id/edit`|order / F05|T-EDIT|T07/T11/T14/T19；E1|
|orderCancel|`/admin/orders`|order / F05|T-CANCEL|T07/T11/T14/T19；E1|
|orderComplete|`/admin/orders`|order / F05|T-STATE|T07/T11/T14/T19；E1|
|orderTrash|`/admin/orders`|order / F05|T-STATE|T07/T11/T14/T19；E1|
|orderRestore|`/admin/orders`|order / F05|T-STATE|T07/T11/T14/T19；E1|
|pickup|`/admin/pickup`|redemption / F04|T-PICKUP|T04/T05/T06/T10/T11/T12/T13；D1/D2|
|inventory|`/admin/inventory`|inventory / F06|T-READ|T06/T08/T22；B2/H1|
|inventoryLedger|`/admin/inventory/transactions`|inventory / F06|T-READ|T06/T08/T22；B2/H1|
|stockDocuments|`/admin/inventory/documents`|inventory / F06|T-READ|T06/T08/T22；B2/H1|
|createStockDocument|`/admin/inventory/documents/new`|inventory / F06|T-CRUD|T06/T08/T22；B2/H1|
|approveStockDocument|`/admin/inventory/documents`|inventory / F06|T-APPROVE|T06/T08/T22；B2/H1|
|rejectStockDocument|`/admin/inventory/documents`|inventory / F06|T-APPROVE|T06/T08/T22；B2/H1|
|inventoryReconcile|`/admin/inventory/reconcile`|inventory / F06|T-READ|T06/T08/T22；B2/H1|
|shipOrder|`/admin/orders/:id/ship`|shipping/redemption / F07|T-SHIP|T09/T17；E2/G2|
|autoLabel|`/admin/orders/:id/ship`|shipping/redemption / F07|T-JOB|T09/T17；E2/G2|
|trackingWebhook|`worker`|shipping/redemption / F07|T-EVENT|T09/T17；E2/G2|
|afterSales|`/admin/after-sales`|order/shipping / F08|T-READ|T16/T17；G2|
|decideAfterSale|`/admin/after-sales/:id`|order/shipping / F08|T-STATE|T16/T17；G2|
|replacementShipment|`/admin/after-sales/:id`|order/shipping / F08|T-REPLACE|T16/T17；G2|
|feedbackList|`/admin/feedback`|operations / F09|T-READ|T18/T20；F2/G2|
|resolveFeedback|`/admin/feedback`|operations / F09|T-STATE|T18/T20；F2/G2|
|permissions|`/admin/roles`|identity / F00|T-READ|T01/T18；A2/G3|
|auditLogs|`/admin/audit`|identity / F00|T-READ|T01/T18；A2/G3|
|getConfig|`/admin/settings`|operations/identity / F10|T-READ|T01/T11/T17/T22；G3/H2|
|updateConfig|`/admin/settings`|operations/identity / F10|T-CONFIG|T01/T11/T17/T22；G3/H2|
|dashboard|`/admin`|operations/identity / F10|T-READ|T01/T11/T17/T22；G3/H2|
|notifications|`/admin/notifications`|operations/identity / F10|T-READ|T01/T11/T17/T22；G3/H2|
|retryNotification|`/admin/notifications`|operations/identity / F10|T-JOB|T01/T11/T17/T22；G3/H2|
|files|`/admin/files`|operations/voucher/shipping / F11|T-READ|T15/T21；G1|
|uploadFile|`/admin/files`|operations/voucher/shipping / F11|T-CRUD|T15/T21；G1|
|downloadFile|`/admin/files`|operations/voucher/shipping / F11|T-DOWNLOAD|T15/T21；G1|
|importJob|`/admin/imports`|operations/voucher/shipping / F11|T-JOB|T15/T21；G1|
|commitImport|`/admin/imports/:id`|operations/voucher/shipping / F11|T-JOB|T15/T21；G1|
|exportJob|`/admin/exports`|operations/voucher/shipping / F11|T-JOB|T15/T21；G1|
|getJob|`/admin/jobs/:id`|operations/voucher/shipping / F11|T-READ|T15/T21；G1|
|changePassword|`/admin/change-password`|identity / F00|T-AUTH|T01/T18；A2/G3|
|voucherSession|`/redeem`|identity/voucher / F03|T-READ|T01/T12/T15/T18；A2/C2|
|resolveJob|`/admin/jobs/:id`|shipping/redemption / F07|T-STATE|T09/T17；E2/G2|
|resolveNotification|`/admin/notifications`|operations/identity / F10|T-STATE|T01/T11/T17/T22；G3/H2|
|setInventoryWarning|`/admin/inventory`|inventory / F06|T-STATE|T06/T08/T22；B2/H1|
|adminOrderShipments|`/admin/orders/:id`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|adminShipmentEvents|`/admin/orders/:id`|shipping/redemption / F07|T-READ|T09/T17；E2/G2|
|getAfterSale|`/admin/after-sales/:id`|order/shipping / F08|T-READ|T16/T17；G2|

## 已确认行为与决策的边界

|证据约束|实施保留|补足决策/测试|
|---|---|---|
|E05商品编码、图片、属性、有效期、禁配地区|ProductInput/Update对应字段|唯一约束/单商品库存ADR11，T03/T13|
|E06多选一/多选多、数量显隐|ONE_OF/N_OF及数量输入|恰N种各1件ADR01；一次兑换ADR02；T04|
|E06生成数量、前缀、顺序/随机、密码长度/类型|BatchInput字段|安全随机和短期导出ADR09；T02/T15|
|E06导出后加密提示|生成→下载→封存操作顺序|哈希+加密短期文件替代内部未知算法ADR09|
|E06激活/提货两个筛选|active和消费记录独立|不可逆消费、禁重置ADR03；T05/T19|
|E07待发/已发/完成标签|PENDING/SHIPPED/COMPLETED|CANCELLED、软删及合法转换ADR06/08；T07/T09|
|E08入出库与审核表单|单据/审核操作|预占和扣减时点ADR04；T06/T08|
|E08物流公司/单号与人工发货|Carrier/Shipment字段与操作|整单发货、Outbox失败补偿ADR08/10；T09/T17|
|E02卡密鉴真、预约入口及查询字段|相同业务入口，服务端凭据验证|作用域会话、验证码、防枚举ADR12；T18|
|E11写测被拒绝|不宣称端到端原站成功|自有测试闭环T01–T22验证新契约|

## 未知项闭合索引

卡券选品数量→ADR01；分次提货→ADR02；核销时间→ADR03；库存时间→ADR04；日期额度→ADR05；取消→ADR06；编辑→ADR07；已发异常/补发→ADR08；密码与导出→ADR09；供应商失败→ADR10。其余产品粒度/会话/文件/角色/部署/延期见ADR11–16。每项选择都已生效用于新系统实施；原站事实仍未知，不把设计验证写回LIVE_OBSERVED。

## 52页覆盖核对

M01–M52逐页都有原站路径、可访问性、目标路由、operationId或明确“不实施/V2”、工作包；完整矩阵只维护11一份，避免两份表漂移。M07/M13不实施危险初始化；M16用离线runbook；M21/M42/M46/M48–51延期；M41仅失败业务通知重试；其他按矩阵纳入v1。普通商品列表的原型删除不变成后台硬删除权限。

R01–R12全部覆盖。无支付模块，无金额字段，因此本系统不引入金额计算或伪支付测试。公开页面验收与核心事务验收均必须完成，不能仅用UI快照替代持久化核对。
