# 09 API契约

**DESIGN_DECISION**：权威机器契约为[openapi.yaml](openapi.yaml)，OpenAPI 3.1.1，使用YAML序列化。所有操作均含具体Schema、合成示例、鉴权/权限、页面/流程、事务、幂等及测试标识；本文给出共用语义和索引。原站真实请求见[基线](01-research-baseline.md)，禁止混用。

共106个操作、83个路径、108个命名Schema。生成类型时只读Schema，不能把数据库列直接暴露给前端。

## 协议规范

JSON UTF-8，未知字段400；ID十进制字符串、version安全整数、数量整数；时间RFC3339 UTC，ship_date为上海业务日期。所有非GET操作默认CSRF；provider回调用HMAC验签替代。认证cookie分ADMIN/VOUCHER/ORDER/PREAUTH。对查询session仅返回脱敏Order；修改要WRITE作用域，见12。

统一错误：{code,message,request_id,field_errors:[{field,code}],retryable}。400格式/未知状态值；401未认证或错误凭证；403权限/CSRF；404不存在或跨主体；409状态/版本/幂等冲突；422合法格式但业务拒绝；429含Retry-After；503依赖暂不可用。凭证材料过期下载410/EXPORT_EXPIRED。不把HTTP200业务失败复制到新系统。

列表统一id DESC及after_id排他游标，limit默认20最大100；数据变更不保证总数，响应不返回total；无下一页省略next_after_id。q仅产品name/code、订单order_no、卡券code、反馈message、审计action等声明字段；其他列表q非空400。status只接受对应资源枚举。额外日期/电话号码筛选不在v1通用GET里传PII；手机号查询仅POST。

响应例子为Schema最小合法形状，不是可直接在当前日期提交的业务夹具。跨字段约束在05与04：类型数量匹配、日期先后、媒体READY、ID存在、数组去重、versions长度对应ids。出现冲突以状态/事务规格为准并修正OpenAPI，不能忽略约束。

## 幂等与事务对应

T-READ：只读；T-AUTH：认证/会话/challenge原子变更；T-CRUD：对象/子表/审计同事务；T-STATE：行锁+version+合法转换+审计；T-BULK：显式ID按升序锁，最多500，任一失败全回滚；T-REDEEM/T-PICKUP/T-CANCEL/T-EDIT/T-SHIP/T-APPROVE/T-REPLACE按13事务表；T-GENERATE见04；T-SEAL见12；T-JOB只创建任务与Outbox、返回202，实际任务状态轮询；T-CONFIG锁配置及未来日历、禁止容量降至used以下；T-EVENT回调事件去重、完成转换+Outbox；T-DOWNLOAD不缓存敏感字节，审计下载授权。

x-tests引用每操作基础契约测试，并与15的流程级并发/故障用例联合验收。不是本轮已执行的业务测试。请求为multipart或binary响应时示例为文件类型说明，实施以测试夹具PNG/CSV验证。

## 操作索引

|operationId|方法/路径|输入→输出|权限|流程/事务|工作包|
|---|---|---|---|---|---|

|bootstrap|GET `/api/v1/session/bootstrap`|文件 → Session|public|F03 / T-AUTH|A|
|createChallenge|POST `/api/v1/challenges`|ChallengeInput → Challenge|preauth.scoped|F03 / T-AUTH|C|
|adminLogin|POST `/api/v1/admin/session`|Login → Session|preauth.scoped|F00 / T-AUTH|A|
|adminSession|GET `/api/v1/admin/session`|文件 → Session|admin.scoped|F00 / T-READ|A|
|adminLogout|DELETE `/api/v1/admin/session`|文件 → 无正文|admin.scoped|F00 / T-AUTH|A|
|adminReauth|POST `/api/v1/admin/session/reauth`|Reauth → Session|admin.scoped|F00 / T-AUTH|A|
|publicProducts|GET `/api/v1/products`|文件 → ProductPublicList|public|F01 / T-READ|F|
|publicContent|GET `/api/v1/content`|文件 → ContentList|public|F01 / T-READ|F|
|regions|GET `/api/v1/regions`|文件 → RegionList|public|F01 / T-READ|F|
|publicProduct|GET `/api/v1/products/{id}`|文件 → ProductPublic|public|F01 / T-READ|F|
|publicContentPage|GET `/api/v1/content/{slug}`|文件 → Content|public|F01 / T-READ|F|
|verifyVoucher|POST `/api/v1/voucher-sessions`|Credentials → Verification|preauth.scoped|F03 / T-AUTH|C|
|redemptionOptions|GET `/api/v1/redemption/options`|文件 → Options|voucher.scoped|F04 / T-READ|D|
|redemptionCalendar|GET `/api/v1/redemption/calendar`|文件 → Calendar|voucher.scoped|F04 / T-READ|D|
|redeem|POST `/api/v1/redemptions`|Redeem → Order|voucher.WRITE scoped；先查幂等重放|F04 / T-REDEEM|D|
|queryOrder|POST `/api/v1/order-sessions`|QueryCredentials → Session|preauth.scoped|F05 / T-AUTH|E|
|orderStepUp|POST `/api/v1/order-session/step-up`|Credentials → Session|order.scoped|F05 / T-AUTH|E|
|customerOrder|GET `/api/v1/order`|文件 → Order|order.scoped|F05 / T-READ|E|
|customerOrderEdit|PATCH `/api/v1/order`|OrderEdit → Order|order.WRITE scoped; must own order|F05 / T-EDIT|E|
|customerShipments|GET `/api/v1/order/shipments`|文件 → ShipmentList|order.scoped|F07 / T-READ|E|
|customerTracking|GET `/api/v1/shipments/{id}/events`|文件 → TrackingEventList|order.scoped|F07 / T-READ|E|
|customerAfterSale|POST `/api/v1/order/after-sales`|AfterSaleInput → AfterSale|order.WRITE scoped; must own order|F08 / T-STATE|G|
|customerAfterSales|GET `/api/v1/order/after-sales`|文件 → AfterSaleList|order.scoped|F08 / T-READ|G|
|createFeedback|POST `/api/v1/feedback`|FeedbackInput → Feedback|preauth.scoped|F09 / T-STATE|F|
|listProduct|GET `/api/v1/admin/products`|文件 → ProductList|product.read|F01 / T-READ|B|
|getProduct|GET `/api/v1/admin/products/{id}`|文件 → Product|product.read|F01 / T-READ|B|
|createProduct|POST `/api/v1/admin/products`|ProductInput → Product|product.write|F01 / T-CRUD|B|
|updateProduct|PATCH `/api/v1/admin/products/{id}`|ProductUpdate → Product|product.write|F01 / T-CRUD|B|
|listCarrier|GET `/api/v1/admin/carriers`|文件 → CarrierList|carrier.read|F07 / T-READ|E|
|getCarrier|GET `/api/v1/admin/carriers/{id}`|文件 → Carrier|carrier.read|F07 / T-READ|E|
|createCarrier|POST `/api/v1/admin/carriers`|CarrierInput → Carrier|carrier.write|F07 / T-CRUD|E|
|updateCarrier|PATCH `/api/v1/admin/carriers/{id}`|CarrierUpdate → Carrier|carrier.write|F07 / T-CRUD|E|
|listAdmin|GET `/api/v1/admin/admins`|文件 → AdminList|admin.read|F00 / T-READ|G|
|getAdmin|GET `/api/v1/admin/admins/{id}`|文件 → Admin|admin.read|F00 / T-READ|G|
|createAdmin|POST `/api/v1/admin/admins`|AdminInput → Admin|admin.write|F00 / T-CRUD|G|
|updateAdmin|PATCH `/api/v1/admin/admins/{id}`|AdminUpdate → Admin|admin.write|F00 / T-CRUD|G|
|listRole|GET `/api/v1/admin/roles`|文件 → RoleList|role.read|F00 / T-READ|G|
|getRole|GET `/api/v1/admin/roles/{id}`|文件 → Role|role.read|F00 / T-READ|G|
|createRole|POST `/api/v1/admin/roles`|RoleInput → Role|role.write|F00 / T-CRUD|G|
|updateRole|PATCH `/api/v1/admin/roles/{id}`|RoleUpdate → Role|role.write|F00 / T-CRUD|G|
|listContent|GET `/api/v1/admin/content`|文件 → ContentList|content.read|F01 / T-READ|F|
|getContent|GET `/api/v1/admin/content/{id}`|文件 → Content|content.read|F01 / T-READ|F|
|createContent|POST `/api/v1/admin/content`|ContentInput → Content|content.write|F01 / T-CRUD|F|
|updateContent|PATCH `/api/v1/admin/content/{id}`|ContentUpdate → Content|content.write|F01 / T-CRUD|F|
|productStatus|POST `/api/v1/admin/products/{id}/status`|ProductStatus → Product|product.write|F01 / T-STATE|B|
|categories|GET `/api/v1/admin/categories`|文件 → CategoryList|product.read|F01 / T-READ|B|
|createCategory|POST `/api/v1/admin/categories`|CategoryInput → Category|product.write|F01 / T-CRUD|B|
|voucherTypes|GET `/api/v1/admin/voucher-types`|文件 → VoucherTypeList|voucher.read|F02 / T-READ|C|
|createVoucherType|POST `/api/v1/admin/voucher-types`|TypeInput → VoucherType|voucher.generate|F02 / T-CRUD|C|
|batches|GET `/api/v1/admin/voucher-batches`|文件 → BatchList|voucher.read|F02 / T-READ|C|
|createBatch|POST `/api/v1/admin/voucher-batches`|BatchInput → Batch|voucher.generate|F02 / T-CRUD|C|
|getBatch|GET `/api/v1/admin/voucher-batches/{id}`|文件 → Batch|voucher.read|F02 / T-READ|C|
|generateBatch|POST `/api/v1/admin/voucher-batches/{id}/generate`|Version → Batch|voucher.generate|F02 / T-GENERATE|C|
|sealBatch|POST `/api/v1/admin/voucher-batches/{id}/seal`|VersionReason → Batch|voucher.seal|F02 / T-SEAL|C|
|vouchers|GET `/api/v1/admin/vouchers`|文件 → VoucherList|voucher.read|F02 / T-READ|C|
|getVoucher|GET `/api/v1/admin/vouchers/{id}`|文件 → Voucher|voucher.read|F02 / T-READ|C|
|editVoucher|PATCH `/api/v1/admin/vouchers/{id}`|VoucherEdit → Voucher|voucher.write|F02 / T-STATE|C|
|bulkVouchers|POST `/api/v1/admin/vouchers/actions`|BulkVouchers → BulkResult|voucher.write|F02 / T-BULK|C|
|resetSecret|POST `/api/v1/admin/vouchers/{id}/reset-secret`|SecretReset → CredentialDelivery|voucher.secret.reset|F02 / T-SEAL|C|
|sealVoucher|POST `/api/v1/admin/vouchers/{id}/seal`|VersionReason → Voucher|voucher.seal|F02 / T-SEAL|C|
|orders|GET `/api/v1/admin/orders`|文件 → OrderList|order.read|F05 / T-READ|E|
|getOrder|GET `/api/v1/admin/orders/{id}`|文件 → Order|order.read|F05 / T-READ|E|
|privateOrder|POST `/api/v1/admin/orders/{id}/private-view`|VersionReason → OrderPrivate|order.pii|F05 / T-STATE|E|
|adminOrderEdit|PATCH `/api/v1/admin/orders/{id}`|OrderEdit → Order|order.write|F05 / T-EDIT|E|
|orderCancel|POST `/api/v1/admin/orders/{id}/cancel`|VersionReason → Order|order.cancel|F05 / T-CANCEL|E|
|orderComplete|POST `/api/v1/admin/orders/{id}/complete`|VersionReason → Order|order.complete|F05 / T-STATE|E|
|orderTrash|POST `/api/v1/admin/orders/{id}/trash`|VersionReason → Order|order.trash|F05 / T-STATE|E|
|orderRestore|POST `/api/v1/admin/orders/{id}/restore`|VersionReason → Order|order.restore|F05 / T-STATE|E|
|pickup|POST `/api/v1/admin/pickups`|Pickup → Order|order.pickup|F04 / T-PICKUP|E|
|inventory|GET `/api/v1/admin/inventory`|文件 → InventoryList|inventory.read|F06 / T-READ|B|
|inventoryLedger|GET `/api/v1/admin/inventory/transactions`|文件 → LedgerList|inventory.read|F06 / T-READ|B|
|stockDocuments|GET `/api/v1/admin/inventory/documents`|文件 → StockDocumentList|inventory.read|F06 / T-READ|B|
|createStockDocument|POST `/api/v1/admin/inventory/documents`|StockDocumentInput → StockDocument|inventory.write|F06 / T-CRUD|B|
|approveStockDocument|POST `/api/v1/admin/inventory/documents/{id}/approve`|VersionReason → StockDocument|inventory.approve|F06 / T-APPROVE|B|
|rejectStockDocument|POST `/api/v1/admin/inventory/documents/{id}/reject`|VersionReason → StockDocument|inventory.approve|F06 / T-APPROVE|B|
|inventoryReconcile|GET `/api/v1/admin/inventory/reconciliation`|文件 → Reconcile|inventory.read|F06 / T-READ|H|
|shipOrder|POST `/api/v1/admin/orders/{id}/shipments`|Ship → Shipment|order.ship|F07 / T-SHIP|E|
|autoLabel|POST `/api/v1/admin/orders/{id}/label-jobs`|VersionReason → Job|order.ship|F07 / T-JOB|G|
|trackingWebhook|POST `/api/v1/integrations/{provider}/tracking-events`|ProviderEvent → 无正文|provider.scoped|F07 / T-EVENT|G|
|afterSales|GET `/api/v1/admin/after-sales`|文件 → AfterSaleList|after_sale.read|F08 / T-READ|G|
|decideAfterSale|POST `/api/v1/admin/after-sales/{id}/decision`|AfterSaleDecision → AfterSale|after_sale.write|F08 / T-STATE|G|
|replacementShipment|POST `/api/v1/admin/after-sales/{id}/shipments`|Ship → Shipment|order.ship|F08 / T-REPLACE|G|
|feedbackList|GET `/api/v1/admin/feedback`|文件 → FeedbackList|feedback.read|F09 / T-READ|G|
|resolveFeedback|POST `/api/v1/admin/feedback/{id}/resolve`|FeedbackResolve → Feedback|feedback.write|F09 / T-STATE|G|
|permissions|GET `/api/v1/admin/permissions`|文件 → PermissionList|role.read|F00 / T-READ|G|
|auditLogs|GET `/api/v1/admin/audit-logs`|文件 → AuditList|audit.read|F00 / T-READ|G|
|getConfig|GET `/api/v1/admin/config`|文件 → Config|config.read|F10 / T-READ|G|
|updateConfig|PUT `/api/v1/admin/config`|Config → Config|config.write|F10 / T-CONFIG|G|
|dashboard|GET `/api/v1/admin/dashboard`|文件 → Dashboard|dashboard.read|F10 / T-READ|G|
|notifications|GET `/api/v1/admin/notifications`|文件 → NotificationList|notification.read|F10 / T-READ|G|
|retryNotification|POST `/api/v1/admin/notifications/{id}/retry`|VersionReason → Notification|notification.retry|F10 / T-JOB|G|
|files|GET `/api/v1/admin/files`|文件 → FileList|file.read|F11 / T-READ|G|
|uploadFile|POST `/api/v1/admin/files`|文件 → File|file.write|F11 / T-CRUD|B|
|downloadFile|POST `/api/v1/admin/files/{id}/download`|文件 → 二进制|file.read；CREDENTIAL_EXPORT额外voucher.export + recent reauth|F11 / T-DOWNLOAD|C|
|importJob|POST `/api/v1/admin/import-jobs`|Import → Job|file.import|F11 / T-JOB|G|
|commitImport|POST `/api/v1/admin/import-jobs/{id}/commit`|VersionReason → Job|file.import|F11 / T-JOB|G|
|exportJob|POST `/api/v1/admin/export-jobs`|Export → Job|file.export|F11 / T-JOB|G|
|getJob|GET `/api/v1/admin/jobs/{id}`|文件 → Job|file.read plus permission of originating job kind and ownership|F11 / T-READ|G|
|changePassword|POST `/api/v1/admin/session/password`|PasswordChange → Action|self.password|F00 / T-AUTH|A|
|voucherSession|GET `/api/v1/voucher-session`|文件 → Session|voucher.scoped|F03 / T-READ|D|
|resolveJob|POST `/api/v1/admin/jobs/{id}/resolve`|ResolveJob → Job|integration.manage|F07 / T-STATE|G|
|resolveNotification|POST `/api/v1/admin/notifications/{id}/resolve`|ResolveNotification → Notification|notification.manage|F10 / T-STATE|G|
|setInventoryWarning|PATCH `/api/v1/admin/inventory/{id}/warning`|InventoryWarning → Inventory|inventory.write|F06 / T-STATE|B|
|adminOrderShipments|GET `/api/v1/admin/orders/{id}/shipments`|文件 → ShipmentList|order.read|F07 / T-READ|E|
|adminShipmentEvents|GET `/api/v1/admin/shipments/{id}/events`|文件 → TrackingEventList|order.read|F07 / T-READ|E|
|getAfterSale|GET `/api/v1/admin/after-sales/{id}`|文件 → AfterSale|aftersale.read|F08 / T-READ|G|
