# 08 数据库详细设计

所有表、字段、类型和关系均为 **DESIGN_DECISION**。原站概念依据见[统一基线](01-research-baseline.md)。[完整DDL](schema.sql)是实施规范，本轮未执行MySQL迁移。

默认单租户、单仓、单次兑换。Product自身是库存单位；不创建ProductVariants。地址加密快照并入orders；入/出库共用inventory_documents。每卡voucher_products冻结集合。无支付、余额及退款账。

共用字段：id BIGINT UNSIGNED自增主键；created_at/updated_at DATETIME(6)非空默认CURRENT_TIMESTAMP(6)，updated_at自动更新；version BIGINT UNSIGNED非空默认1，所有业务修改必须显式递增。JSON在Go严格校验形状；不存在未声明的状态。API ID以十进制字符串返回。全部FK ON DELETE RESTRICT/ON UPDATE RESTRICT，引用索引显式生成。

字符集utf8mb4，排序规则utf8mb4_0900_as_cs；卡号/编码规范化为大写ASCII后唯一，商品名不作唯一键。DATETIME为UTC，预约DATE用Asia/Shanghai；所有数量为整数，禁止浮点及负库存。业务状态除下表枚举外无其他值。

## admin_users

管理员账户

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|login_name|VARCHAR(64)|否|必须提供|login_name|
|password_hash|VARCHAR(255)|否|必须提供|Argon2id PHC|
|enabled|BOOLEAN|否|1|启用|
|force_password_change|BOOLEAN|否|1|临时密码首次登录必须修改|
|auth_version|BIGINT|否|1|凭证/角色变化递增|
|last_login_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(login_name)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## roles

角色

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|name|VARCHAR(64)|否|必须提供|name|
|enabled|BOOLEAN|否|1|启用|

约束/索引：UNIQUE(name)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## permissions

编译期权限目录

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|permission_key|VARCHAR(80)|否|必须提供|permission_key|
|description|VARCHAR(200)|否|必须提供|description|

约束/索引：UNIQUE(permission_key)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## admin_user_roles

管理员角色关联

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|admin_id|BIGINT UNSIGNED|否|必须提供|admin_users引用；FK→admin_users.id|
|role_id|BIGINT UNSIGNED|否|必须提供|roles引用；FK→roles.id|

约束/索引：UNIQUE(admin_id,role_id)；INDEX(role_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## role_permissions

角色权限关联

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|role_id|BIGINT UNSIGNED|否|必须提供|roles引用；FK→roles.id|
|permission_id|BIGINT UNSIGNED|否|必须提供|permissions引用；FK→permissions.id|

约束/索引：UNIQUE(role_id,permission_id)；INDEX(permission_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## categories

自主单层商品分类

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|name|VARCHAR(80)|否|必须提供|name|
|sort_order|BIGINT|否|0|展示排序|

约束/索引：UNIQUE(name)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## files

本地受控文件与短期凭证导出

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|storage_key|VARCHAR(200)|否|必须提供|服务端随机相对路径|
|kind|VARCHAR(24)|否|'IMAGE'|枚举：IMAGE,IMPORT,CREDENTIAL_EXPORT,REPORT|
|mime_type|VARCHAR(100)|否|必须提供|mime_type|
|size_bytes|BIGINT|否|0|文件字节数|
|sha256|BINARY(32)|否|必须提供|完整性摘要|
|state|VARCHAR(24)|否|'STAGED'|枚举：STAGED,READY,EXPIRED,DELETED|
|owner_admin_id|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|
|key_version|INT UNSIGNED|是|NULL|加密密钥版本|
|expires_at|DATETIME(6)|是|NULL|UTC时间|
|deleted_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(storage_key)；INDEX(state,expires_at)；INDEX(owner_admin_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## products

最小可兑换单位；v1不设SKU

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|code|VARCHAR(64)|否|必须提供|全局唯一、停用也不复用|
|name|VARCHAR(120)|否|必须提供|name|
|category_id|BIGINT UNSIGNED|是|NULL|categories引用；FK→categories.id|
|attributes|JSON|否|必须提供|最多20个{name,value}|
|description_html|MEDIUMTEXT|否|必须提供|服务端清洗后的富文本|
|external_spec_code|VARCHAR(64)|是|NULL|external_spec_code|
|external_spec_name|VARCHAR(120)|是|NULL|external_spec_name|
|redeem_from|DATETIME(6)|是|NULL|UTC时间|
|redeem_until|DATETIME(6)|是|NULL|UTC时间|
|excluded_region_codes|JSON|否|必须提供|地区代码数组；禁配覆盖优先|
|sort_order|BIGINT|否|0|展示排序|
|status|VARCHAR(24)|否|'DRAFT'|枚举：DRAFT,ENABLED,DISABLED|
|deleted_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(code)；INDEX(status,sort_order,id)；INDEX(category_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## product_media

主图及轮播引用

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|file_id|BIGINT UNSIGNED|否|必须提供|files引用；FK→files.id|
|position|BIGINT|否|0|0主图，1..8轮播|

约束/索引：UNIQUE(product_id,position)；INDEX(file_id)。CHECK：position BETWEEN 0 AND 8。

## voucher_types

不可变版本权益类型

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|name|VARCHAR(80)|否|必须提供|name|
|revision|INT UNSIGNED|否|1|同名递增版本|
|selection_mode|VARCHAR(24)|否|'ONE_OF'|枚举：ONE_OF,N_OF|
|selection_count|INT UNSIGNED|否|1|ONE_OF=1,N_OF=2..99|
|enabled|BOOLEAN|否|1|可用于新批次|

约束/索引：UNIQUE(name,revision)。CHECK：selection_count BETWEEN 1 AND 99；selection_mode <> 'ONE_OF' OR selection_count = 1。

## voucher_batches

批次配置及生成任务

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|batch_no|VARCHAR(40)|否|必须提供|batch_no|
|type_id|BIGINT UNSIGNED|否|必须提供|voucher_types引用；FK→voucher_types.id|
|label|VARCHAR(100)|否|必须提供|label|
|policy_snapshot|JSON|否|必须提供|模式/数量/商品集合/规则版本不可变|
|quantity|INT UNSIGNED|否|必须提供|1..1000|
|code_prefix|VARCHAR(4)|否|必须提供|code_prefix|
|code_length|INT UNSIGNED|否|16|12..24|
|code_mode|VARCHAR(24)|否|'RANDOM'|枚举：SEQUENTIAL,RANDOM|
|start_serial|VARCHAR(24)|是|NULL|start_serial|
|secret_length|INT UNSIGNED|否|12|6/8/9/10/11/12/16/24/32，默认12|
|secret_format|VARCHAR(24)|否|'MIXED'|枚举：DIGITS,LETTERS,MIXED|
|valid_from|DATETIME(6)|否|必须提供|UTC时间|
|valid_until|DATETIME(6)|否|必须提供|UTC时间|
|activate_on_seal|BOOLEAN|否|0|封存成功后激活|
|state|VARCHAR(24)|否|'DRAFT'|枚举：DRAFT,GENERATING,GENERATED,SEALED,FAILED|
|export_file_id|BIGINT UNSIGNED|是|NULL|files引用；FK→files.id|
|created_by|BIGINT UNSIGNED|否|必须提供|admin_users引用；FK→admin_users.id|
|lease_until|DATETIME(6)|是|NULL|UTC时间|
|generation_epoch|BIGINT UNSIGNED|否|0|每次领取递增，防旧worker提交|
|last_error_code|VARCHAR(80)|是|NULL|last_error_code|

约束/索引：UNIQUE(batch_no)；INDEX(state,lease_until)；INDEX(type_id)；INDEX(export_file_id)；INDEX(created_by)。CHECK：quantity BETWEEN 1 AND 1000；valid_until > valid_from；code_length BETWEEN 12 AND 24；secret_length IN (6,8,9,10,11,12,16,24,32)。

## vouchers

单次消费卡券；已兑换不可复用

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|batch_id|BIGINT UNSIGNED|否|必须提供|voucher_batches引用；FK→voucher_batches.id|
|credential_file_id|BIGINT UNSIGNED|是|NULL|files引用；FK→files.id|
|code|VARCHAR(32)|否|必须提供|code|
|secret_hash|VARCHAR(255)|否|必须提供|Argon2id PHC；禁止明文|
|credential_version|INT UNSIGNED|否|1|重置递增|
|active|BOOLEAN|否|0|激活位|
|credential_state|VARCHAR(24)|否|'EXPORTABLE'|枚举：EXPORTABLE,SEALED|
|valid_from|DATETIME(6)|否|必须提供|UTC时间|
|valid_until|DATETIME(6)|否|必须提供|UTC时间|
|redeemed_at|DATETIME(6)|是|NULL|UTC时间|
|deleted_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(code)；INDEX(batch_id,id)；INDEX(active,redeemed_at)；INDEX(credential_file_id)。CHECK：valid_until > valid_from。

## voucher_products

每卡生成时冻结可兑商品集合

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|voucher_id|BIGINT UNSIGNED|否|必须提供|vouchers引用；FK→vouchers.id|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|

约束/索引：UNIQUE(voucher_id,product_id)；INDEX(product_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## voucher_redemptions

不可变消费记录；取消不删除

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|voucher_id|BIGINT UNSIGNED|否|必须提供|vouchers引用；FK→vouchers.id|
|policy_snapshot|JSON|否|必须提供|消费规则版本与集合摘要|
|redeemed_at|DATETIME(6)|否|必须提供|UTC时间|

约束/索引：UNIQUE(voucher_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## shipping_calendars

按上海业务日的预约容量

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|ship_date|DATE|否|必须提供|唯一业务日期|
|capacity|INT UNSIGNED|否|100|默认100，0关闭|
|used_slots|INT UNSIGNED|否|0|待发+已发+已完成占用|
|closed|BOOLEAN|否|0|显式停发|

约束/索引：UNIQUE(ship_date)。CHECK：used_slots <= capacity。

## orders

兑换订单；不含付款

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_no|VARCHAR(40)|否|必须提供|order_no|
|redemption_id|BIGINT UNSIGNED|否|必须提供|voucher_redemptions引用；FK→voucher_redemptions.id|
|status|VARCHAR(24)|否|'PENDING'|枚举：PENDING,SHIPPED,COMPLETED,CANCELLED|
|fulfillment_mode|VARCHAR(24)|否|'DELIVERY'|枚举：DELIVERY,PICKUP|
|calendar_id|BIGINT UNSIGNED|是|NULL|shipping_calendars引用；FK→shipping_calendars.id|
|recipient_cipher|BLOB|否|必须提供|姓名手机地址JSON的AEAD密文|
|pii_key_version|INT UNSIGNED|否|必须提供|密钥版本|
|phone_lookup|BINARY(32)|否|必须提供|独立索引密钥HMAC规范手机号|
|customer_note|VARCHAR(200)|否|必须提供|customer_note|
|admin_note|VARCHAR(1000)|否|必须提供|admin_note|
|shipped_at|DATETIME(6)|是|NULL|UTC时间|
|completed_at|DATETIME(6)|是|NULL|UTC时间|
|cancelled_at|DATETIME(6)|是|NULL|UTC时间|
|cancel_reason|VARCHAR(200)|是|NULL|cancel_reason|
|deleted_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(order_no)；UNIQUE(redemption_id)；INDEX(status,calendar_id,id)；INDEX(phone_lookup)；INDEX(deleted_at)；INDEX(calendar_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## order_items

商品数量及历史快照

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_id|BIGINT UNSIGNED|否|必须提供|orders引用；FK→orders.id|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|quantity|INT UNSIGNED|否|1|v1固定1|
|product_snapshot|JSON|否|必须提供|name/code/attributes/image_file_id|

约束/索引：UNIQUE(order_id,product_id)；INDEX(product_id)。CHECK：quantity = 1。

## inventory_balances

单仓账面实存与预占

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|on_hand|BIGINT|否|0|实际库存|
|reserved|BIGINT|否|0|已预占|
|warning_level|BIGINT|否|0|库存警戒值|

约束/索引：UNIQUE(product_id)。CHECK：on_hand >= 0；reserved >= 0；reserved <= on_hand；warning_level >= 0。

## inventory_documents

入/出库统一单据

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|document_no|VARCHAR(40)|否|必须提供|document_no|
|direction|VARCHAR(24)|否|'IN'|枚举：IN,OUT|
|stock_kind|VARCHAR(24)|否|'PHYSICAL'|枚举：PHYSICAL,VIRTUAL|
|state|VARCHAR(24)|否|'PENDING'|枚举：PENDING,APPROVED,REJECTED|
|note|VARCHAR(500)|否|必须提供|note|
|proof_file_id|BIGINT UNSIGNED|是|NULL|files引用；FK→files.id|
|created_by|BIGINT UNSIGNED|否|必须提供|admin_users引用；FK→admin_users.id|
|reviewed_by|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|
|reviewed_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(document_no)；INDEX(state,id)；INDEX(proof_file_id)；INDEX(created_by)；INDEX(reviewed_by)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## inventory_document_items

审核单商品数量

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|document_id|BIGINT UNSIGNED|否|必须提供|inventory_documents引用；FK→inventory_documents.id|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|quantity|INT UNSIGNED|否|必须提供|正数|

约束/索引：UNIQUE(document_id,product_id)；INDEX(product_id)。CHECK：quantity > 0。

## inventory_reservations

每订单项唯一预占

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_item_id|BIGINT UNSIGNED|否|必须提供|order_items引用；FK→order_items.id|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|quantity|INT UNSIGNED|否|必须提供|对应订单项数量|
|state|VARCHAR(24)|否|'ACTIVE'|枚举：ACTIVE,CONSUMED,RELEASED|

约束/索引：UNIQUE(order_item_id)；INDEX(product_id,state)。CHECK：quantity > 0。

## carriers

物流公司及适配器配置

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|code|VARCHAR(40)|否|必须提供|code|
|name|VARCHAR(80)|否|必须提供|name|
|alias|VARCHAR(80)|否|必须提供|alias|
|kdniao_code|VARCHAR(40)|是|NULL|kdniao_code|
|kuaidi100_code|VARCHAR(40)|是|NULL|kuaidi100_code|
|wdt_code|VARCHAR(40)|是|NULL|wdt_code|
|provider|VARCHAR(24)|否|'MANUAL'|枚举：MANUAL,MOCK,KDNIAO,KUAIDI100|
|enabled|BOOLEAN|否|1|启用|
|secret_ref|VARCHAR(120)|是|NULL|仅引用挂载密钥名称，不存密钥|

约束/索引：UNIQUE(code)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## after_sales

售后独立状态；不逆转原订单

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_id|BIGINT UNSIGNED|否|必须提供|orders引用；FK→orders.id|
|kind|VARCHAR(24)|否|'COMPLAINT'|枚举：COMPLAINT,REPLACEMENT|
|state|VARCHAR(24)|否|'OPEN'|枚举：OPEN,APPROVED,REJECTED,CLOSED|
|reason|VARCHAR(500)|否|必须提供|reason|
|items|JSON|否|必须提供|[{order_item_id,quantity}]；补发数量不超原数量|
|resolution|VARCHAR(1000)|否|必须提供|resolution|
|handled_by|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|
|closed_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：INDEX(order_id,state)；INDEX(handled_by)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## shipments

原始全单或售后补发运单

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_id|BIGINT UNSIGNED|否|必须提供|orders引用；FK→orders.id|
|after_sale_id|BIGINT UNSIGNED|是|NULL|after_sales引用；FK→after_sales.id|
|shipment_key|VARCHAR(100)|否|必须提供|原发order:ID:primary，补发after_sale:ID|
|kind|VARCHAR(24)|否|'PRIMARY'|枚举：PRIMARY,REPLACEMENT|
|carrier_id|BIGINT UNSIGNED|否|必须提供|carriers引用；FK→carriers.id|
|tracking_no|VARCHAR(80)|否|必须提供|tracking_no|
|status|VARCHAR(24)|否|'SHIPPED'|枚举：SHIPPED,IN_TRANSIT,DELIVERED,EXCEPTION|
|shipped_at|DATETIME(6)|否|必须提供|UTC时间|
|delivered_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(shipment_key)；UNIQUE(carrier_id,tracking_no)；INDEX(order_id)；INDEX(after_sale_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## shipment_events

物流事件去重与时间线

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|shipment_id|BIGINT UNSIGNED|否|必须提供|shipments引用；FK→shipments.id|
|provider_event_id|VARCHAR(120)|否|必须提供|provider_event_id|
|event_type|VARCHAR(40)|否|必须提供|event_type|
|description|VARCHAR(300)|否|必须提供|不保存完整地址或手机号|
|occurred_at|DATETIME(6)|否|必须提供|UTC时间|

约束/索引：UNIQUE(shipment_id,provider_event_id)；INDEX(shipment_id,occurred_at)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## inventory_transactions

不可变库存/预占差额账

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|product_id|BIGINT UNSIGNED|否|必须提供|products引用；FK→products.id|
|on_hand_delta|BIGINT|否|0|实存差额|
|reserved_delta|BIGINT|否|0|预占差额|
|event_key|VARCHAR(120)|否|必须提供|确定性操作+对象+商品唯一键|
|order_id|BIGINT UNSIGNED|是|NULL|orders引用；FK→orders.id|
|document_id|BIGINT UNSIGNED|是|NULL|inventory_documents引用；FK→inventory_documents.id|
|shipment_id|BIGINT UNSIGNED|是|NULL|shipments引用；FK→shipments.id|
|reason|VARCHAR(200)|否|必须提供|reason|

约束/索引：UNIQUE(event_key)；INDEX(product_id,id)；INDEX(order_id)；INDEX(document_id)；INDEX(shipment_id)。CHECK：on_hand_delta <> 0 OR reserved_delta <> 0。

## sessions

随机会话只存SHA256摘要

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|token_hash|BINARY(32)|否|必须提供|随机256bit opaque token摘要|
|kind|VARCHAR(24)|否|'PREAUTH'|枚举：ADMIN,VOUCHER,ORDER,PREAUTH|
|scope|VARCHAR(24)|否|'READ'|枚举：READ,WRITE,AUTH_ONLY|
|admin_id|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|
|voucher_id|BIGINT UNSIGNED|是|NULL|vouchers引用；FK→vouchers.id|
|order_id|BIGINT UNSIGNED|是|NULL|orders引用；FK→orders.id|
|subject_version|BIGINT UNSIGNED|否|0|签发时auth/credential版本|
|csrf_hash|BINARY(32)|否|必须提供|CSRF随机值摘要|
|expires_at|DATETIME(6)|否|必须提供|UTC时间|
|last_seen_at|DATETIME(6)|否|必须提供|UTC时间|
|reauth_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：UNIQUE(token_hash)；INDEX(expires_at)；INDEX(admin_id)；INDEX(voucher_id)；INDEX(order_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## challenges

绑定预会话的一次性校验证明

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|session_id|BIGINT UNSIGNED|否|必须提供|sessions引用；FK→sessions.id|
|purpose|VARCHAR(30)|否|必须提供|purpose|
|answer_hash|BINARY(32)|否|必须提供|验证码答案HMAC；不回传答案|
|attempts|INT UNSIGNED|否|0|失败次数|
|expires_at|DATETIME(6)|否|必须提供|UTC时间|
|solved_at|DATETIME(6)|是|NULL|UTC时间|
|consumed_at|DATETIME(6)|是|NULL|UTC时间|

约束/索引：INDEX(session_id,purpose)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## idempotency_keys

原子提交结果；不存明文请求

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|scope|VARCHAR(120)|否|必须提供|主体ID+操作ID|
|request_key|VARCHAR(64)|否|必须提供|request_key|
|request_hash|BINARY(32)|否|必须提供|规范请求HMAC；含PII不可用普通hash|
|response_status|SMALLINT UNSIGNED|是|NULL|成功HTTP状态|
|result_ref|JSON|否|必须提供|成功对象类型/ID，不保存凭证或PII|
|expires_at|DATETIME(6)|否|必须提供|UTC时间|

约束/索引：UNIQUE(scope,request_key)；INDEX(expires_at)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## outbox_events

提交后可靠异步事件

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|event_key|VARCHAR(140)|否|必须提供|event_key|
|event_type|VARCHAR(80)|否|必须提供|event_type|
|payload|JSON|否|必须提供|仅实体ID、版本、模板名；不放凭证|
|state|VARCHAR(24)|否|'PENDING'|枚举：PENDING,PROCESSING,DONE,DEAD|
|attempts|INT UNSIGNED|否|0|尝试次数|
|next_attempt_at|DATETIME(6)|否|必须提供|UTC时间|
|lease_until|DATETIME(6)|是|NULL|UTC时间|
|last_error_code|VARCHAR(80)|是|NULL|last_error_code|

约束/索引：UNIQUE(event_key)；INDEX(state,next_attempt_at)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## notifications

通知投递及不确定结果

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|outbox_id|BIGINT UNSIGNED|否|必须提供|outbox_events引用；FK→outbox_events.id|
|order_id|BIGINT UNSIGNED|是|NULL|orders引用；FK→orders.id|
|channel|VARCHAR(24)|否|'MOCK'|枚举：SMS,EMAIL,WECHAT,MOCK|
|template_key|VARCHAR(80)|否|必须提供|template_key|
|state|VARCHAR(24)|否|'PENDING'|枚举：PENDING,SENT,FAILED,UNKNOWN,SUPPRESSED|
|provider_message_id|VARCHAR(120)|是|NULL|provider_message_id|
|last_error_code|VARCHAR(80)|是|NULL|last_error_code|

约束/索引：UNIQUE(outbox_id,channel)；INDEX(order_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## integration_jobs

自动运单/导入/导出后台作业

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|job_key|VARCHAR(120)|否|必须提供|job_key|
|kind|VARCHAR(24)|否|'LABEL'|枚举：LABEL,TRACKING,IMPORT_VOUCHERS,IMPORT_SHIPMENTS,EXPORT_REPORT|
|state|VARCHAR(24)|否|'PENDING'|枚举：PENDING,RUNNING,READY,FAILED,UNKNOWN|
|order_id|BIGINT UNSIGNED|是|NULL|orders引用；FK→orders.id|
|file_id|BIGINT UNSIGNED|是|NULL|files引用；FK→files.id|
|result_file_id|BIGINT UNSIGNED|是|NULL|files引用；FK→files.id|
|options|JSON|否|必须提供|任务类型校验的JSON，无凭证|
|result_summary|JSON|否|必须提供|成功数/错误行号/错误码|
|lease_until|DATETIME(6)|是|NULL|UTC时间|
|provider_request_id|VARCHAR(120)|是|NULL|provider_request_id|

约束/索引：UNIQUE(job_key)；INDEX(state,id)；INDEX(order_id)；INDEX(file_id)；INDEX(result_file_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## feedback

匿名反馈独立于售后

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|order_id|BIGINT UNSIGNED|是|NULL|orders引用；FK→orders.id|
|contact_cipher|BLOB|否|必须提供|联系方式AEAD密文|
|pii_key_version|INT UNSIGNED|否|必须提供|密钥版本|
|message|VARCHAR(200)|否|必须提供|message|
|state|VARCHAR(24)|否|'OPEN'|枚举：OPEN,RESOLVED|
|resolution|VARCHAR(500)|否|必须提供|resolution|

约束/索引：INDEX(state,id)；INDEX(order_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## content_pages

首页/公告/须知/信息页

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|slug|VARCHAR(64)|否|必须提供|slug|
|title|VARCHAR(120)|否|必须提供|title|
|body_html|MEDIUMTEXT|否|必须提供|清洗正文|
|kind|VARCHAR(24)|否|'PAGE'|枚举：PAGE,ANNOUNCEMENT,BANNER|
|published|BOOLEAN|否|0|公开状态|
|sort_order|BIGINT|否|0|排序|

约束/索引：UNIQUE(slug)；INDEX(published,sort_order)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## system_configs

版本化类型配置；禁止密钥

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|config_key|VARCHAR(80)|否|必须提供|config_key|
|value|JSON|否|必须提供|按固定key Schema校验|
|updated_by|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|

约束/索引：UNIQUE(config_key)；INDEX(updated_by)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## audit_logs

不可变、脱敏审计

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|actor_admin_id|BIGINT UNSIGNED|是|NULL|admin_users引用；FK→admin_users.id|
|actor_kind|VARCHAR(20)|否|必须提供|actor_kind|
|action|VARCHAR(100)|否|必须提供|action|
|entity_type|VARCHAR(50)|否|必须提供|entity_type|
|entity_id|BIGINT UNSIGNED|是|NULL|对象ID|
|request_id|VARCHAR(64)|否|必须提供|request_id|
|diff|JSON|否|必须提供|仅白名单变化；PII用字段名标记|
|result|VARCHAR(40)|否|必须提供|result|

约束/索引：INDEX(entity_type,entity_id,id)；INDEX(actor_admin_id,id)；INDEX(request_id)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## rate_limits

多进程共享固定窗口限流

|字段|类型|可空|默认值|含义/关系|
|---|---|---|---|---|
|id|BIGINT UNSIGNED|否|AUTO_INCREMENT|自增主键|
|created_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC创建时间|
|updated_at|DATETIME(6)|否|CURRENT_TIMESTAMP(6)|UTC更新时间|
|version|BIGINT UNSIGNED|否|1|乐观锁版本|
|bucket_hash|BINARY(32)|否|必须提供|HMAC IP/标识+用途+窗口|
|window_end|DATETIME(6)|否|必须提供|UTC时间|
|attempts|INT UNSIGNED|否|0|原子递增|

约束/索引：UNIQUE(bucket_hash)；INDEX(window_end)。CHECK：状态字段按枚举CHECK；其他跨行规则由Service事务保证。

## 关系、迁移及跨行约束

Product→VoucherProduct←Voucher→VoucherRedemption→Order→OrderItem；VoucherBatch→Voucher，VoucherType→Batch。每卡唯一redemption，每redemption唯一order，不允许取消后再次消费。Batch policy_snapshot与单卡集合在生成时冻结。批量修改仅未兑卡的有效期/激活位，禁止改商品集合；换权益须新批次。

OrderItem→InventoryReservation，Product→InventoryBalance及InventoryTransaction；订单取消只能ACTIVE→RELEASED，发货只能ACTIVE→CONSUMED。账本唯一event_key使同一业务动作不重复入账。实存及预占总和由[事务规格](13-inventory-and-consistency.md)维护，不能靠CHECK证明跨表一致。

Order→Shipment/AfterSale；原发shipment_key=order:{id}:primary唯一，售后补发=after_sale:{id}唯一。Shipment→Events。物流事件只追加，订单状态不因旧事件回退。不得直接删除有引用的实体；商品/卡券/订单使用deleted_at并且代码/单号永不复用。

sessions必须按kind在Service验证恰好一个主体（PREAUTH无主体）；subject_version验证管理员/卡券版本，ORDER主体不依赖已消费卡密。验证码绑定PREAUTH会话与purpose；5分钟过期，至多5次尝试，一次消费。

迁移拆分：0001身份/目录/文件；0002卡券；0003日历/订单/库存；0004物流售后；0005会话/幂等/Outbox/内容/运维。schema.sql用于空库完整建模，实施时按依赖拆文件、FK放最后；DDL非业务事务，采用单独迁移账号与GET_LOCK互斥，schema_migrations(version,checksum,applied_at)由迁移工具创建。迁移checksum不允许变更已发布脚本；expand→回填→验证→contract跨发布，不执行自动破坏性down。

部署前必须在MySQL 8.4空库实际执行、检查约束及EXPLAIN，本轮只做静态校验。读写账号无DDL权限。启动核对迁移版本，不自动迁移。备份同时包括加密数据密钥版本，否则密文不可恢复。

## 固定JSON形状与投影规则

DESIGN_DECISION：JSON字段不允许任意客户端透传。`voucher_batches.policy_snapshot`={policy_version:"selection-v1",type_revision:number,selection_mode:"ONE_OF"|"N_OF",selection_count:number,product_ids:string[]}；`voucher_redemptions.policy_snapshot`在复制前述字段后增加selected_product_ids:string[]。policy_version代表语义版本，不是可编辑数字；校验请求版本与批次一致。batch product_ids/选择模式/数量API均由此快照投影，不能读取已修改类型代替。

`order_items.product_snapshot`={name,code,attributes:[{name,value}],image_file_id:string}，生成时快照而不是查询时回填。`recipient_cipher`解密得到Address Schema，以实体类型Order、预生成唯一order_no和pii_key_version作为AEAD关联数据；事务前准备密文，不在日志留明文。必须对同一订单的每次地址更新产生新随机nonce。feedback采用预生成内部随机对象引用绑定AAD，并在密文封装中保存引用，不保存PII明文。

`after_sales.items`=[{order_item_id:string,quantity:1}]，只允许同订单项目、去重；COMPLAINT可空，REPLACEMENT至少1项，不能只靠JSON列约束。`system_configs.value`的business键严格对应Config（version由表列返回，不重复存入value）；identity_guard键只供最后管理员行锁，值{}。`idempotency_keys.result_ref`={entity_type,id,http_status}，可选file_id/order_id，绝不存完整HTTP响应。Outbox payload={entity_id,entity_version,template_key}按事件类型白名单。integration_jobs JSON精确形状见14。

列表中的api状态、数量或派生属性（如available/redeemed）由数据库列计算映射，不增设第二份可漂移状态。公共审计API对无entity_id的登录失败事件省略该属性；Schema将entity_id设置为可选，日志本体仍保留失败结果。
