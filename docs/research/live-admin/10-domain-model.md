# 10 Go / MySQL领域模型与API规格草案

本文件不是原网站内部数据库。所有新字段名、数据类型、关系、约束及接口路径均为 **DESIGN_DECISION**；“来源”列只说明支撑业务概念的真实字段/页面。缺少原站字段映射的设计字段也明确列为DESIGN_DECISION，不用原型补证。可更换的权益规则须带版本。

## 实体及字段来源

通用字段`id:uint64, created_at:datetime, updated_at:datetime, version:uint64`均为 **DESIGN_DECISION**；原站HTML参数id不证明其存储类型。金额未观察到作为兑换预算，因此不预设金额卡。

|实体|新字段及来源（每个分组均为设计字段）|关系/约束：DESIGN_DECISION|
|---|---|---|
|Product|`name,image_url,content`←LIVE_OBSERVED pro_name/pro_img/pro_content；`code`←spbm；`external_spec_code,external_spec_name`←spec_no/spec_name；`redeem_from,redeem_until,excluded_regions,sort_order,enabled`←s_time/e_time/no_area/p_sort/status；`attributes:[{name,value}]`←parameter0/1；`media[]`←轮播组件。[商品](04-product-workflow.md)|产品编码采用项目范围内唯一约束，是否允许空码由产品约定；属性/图片独立子表；外部规格不是已证实SKU关系|
|VoucherType|`name,selection_mode,rule_version,rule_config`←LIVE_OBSERVED card_type与product_num；新字段命名及规则JSON结构均为设计。[卡券](05-voucher-workflow.md)|独立类型表是设计；mode采用自有稳定标识；规则版本发布后不可修改历史权益|
|VoucherBatch|`type_id,label,valid_from,valid_until,code_policy,credential_policy`←pici/bname/b_stime/b_etime/code_pre/code_type/pass_len/pass_type；`generation_request_id`无原站证据，设计幂等字段|FK type_id；生成请求唯一；批次与卡券关系1:N为设计选择，不宣称原站有此表|
|Voucher|`batch_id,code,credential_hash,active,expires_at,redeemed_at,revoked_at`←批次/卡号/激活/提货/有效期业务概念；hash和revoked_at为自主保护/状态设计|code唯一；不保留可读卡密；凭证校验、激活和兑换状态分离；到期可由时间计算；不能复制原站“加密”作为算法|
|VoucherProduct|`voucher_id,product_id,rule_version,limit_config`←SOURCE_OBSERVED product数组与选择器IDs|复合唯一(voucher_id,product_id,rule_version)，外键；批次发布时快照复制权益的方式为设计，关系基数不从原站数据库推断|
|Order|`order_no,voucher_id,redemption_request_id,status,requested_ship_date,recipient_name,recipient_phone,recipient_address,customer_note,admin_note,version`←LIVE_OBSERVED订单列及编辑字段；请求幂等/版本为设计|order_no与redemption_request_id唯一；订单与Voucher/CardPolicy关联；个人资料加密/访问审计；外部展示单号不作主键|
|OrderItem|`order_id,product_id,quantity,product_snapshot,rule_version`←LIVE_OBSERVED product[]及提货商品；quantity数值规则尚未确认|FK；商品标题/规格快照；是否允许重复商品及单位数由版本化策略裁决|
|Inventory|`product_id,on_hand,reserved,version`←LIVE_OBSERVED累计/已用/可用库存概念；reserved独立字段为DESIGN_DECISION，无预占实测|每商品唯一；不把原站列名等同公式；新系统available=on_hand-reserved，事务保证非负|
|InventoryTransaction|`product_id,quantity_delta,reserved_delta,kind,approval_state,reference_type,reference_id,idempotency_key,actor_id,occurred_at`←LIVE_OBSERVED入出库数量/类型/审核/单号；差额/引用/幂等/审计字段为设计|不可变账本；idempotency_key唯一；更正用反向分录，不直接改历史账|
|Shipment|`order_id,carrier_id,tracking_no,status,shipped_at,external_request_id`←LIVE_OBSERVED t_trace/t_no与物流信息；外部请求标识为设计|订单1:N运输记录为可扩展设计，原站是否多包裹未知；外部请求唯一，重复回调幂等|
|AdminUser|`login_name,credential_hash,enabled`←旧公开登录name/pass概念；当前管理员页面被拒绝，不能称字段实测|唯一login_name；密码验证材料保护；当前账号资料不迁移|
|Role|`name,enabled`←LIVE_OBSERVED角色列表/表单title/hide概念|角色名称唯一策略为设计；AdminUserRole多对多为设计|
|Permission|`key,resource,action,description`←LIVE_OBSERVED rule[]细粒度操作名称；键及资源分解为设计|RolePermission多对多；路由与服务层强制校验；默认拒绝；菜单仅用于体验|

**DESIGN_DECISION**：所有FK建立索引，订单/账本等不可用级联删除破坏审计；商品停用不删除订单快照。`tenant_id`多租户不是当前已知需求，暂不凭空引入；未来需要多租户应重新裁决唯一约束范围。

## 权益与交易边界

**LIVE_OBSERVED**只确认多选一/多选多UI和数量框。**DESIGN_DECISION（可替换MVP）**：策略接口输入商品ID与数量集合，输出校验结果与消耗计划；MVP可以明确选择“单次兑换、选不同商品、每项1件”，但该规则必须作为自主产品决定记录，不宣称原站也是如此。不同业务规则以rule_version迁移，不把product_num硬编码成终身余额或数据库固定列。

**DESIGN_DECISION**：最小闭环采用创建订单时预占、发货时扣实物/释放预占、取消时释放的自有规则；原站时点UNKNOWN。库存策略接口可改为提交即扣减，但同一订单必须绑定规则版本，禁止中途切换口径。

**DESIGN_DECISION**：兑换事务顺序：

1. 校验Idempotency-Key及请求摘要；同键不同请求拒绝。
2. 锁定Voucher并校验激活、有效期、凭证和未消费状态；只处理自有系统数据。
3. 按product_id固定顺序锁库存，重新校验权益与可用量。
4. 写Order/OrderItem、权益消耗、预占账本与审计，原子提交；任何失败整体回滚。
5. 提交后Outbox发送通知/物流集成事件；唯一事件键防重复，不在数据库事务中等待外部网络。

**DESIGN_DECISION**：敏感凭证不入日志、不在列表明文展示；收货信息加密存储并最小权限读取；请求审计只存操作、对象ID、脱敏差异和结果，避免把卡密/个人资料写入审计。原站是否如此实现UNKNOWN。

## 自有API草案（全部DESIGN_DECISION）

|新系统接口|关键字段及响应|约束|
|---|---|---|
|GET /api/admin/products|query: q,enabled,cursor,limit；响应items,next_cursor|筛选/分页独立于原站URL；limit设上限|
|POST /api/admin/products；PATCH /api/admin/products/{id}|Product可编辑字段；修改带version|服务端字段校验与权限；更新冲突409|
|POST /api/admin/voucher-batches|type_id,rule_version,product_ids,quantity,valid_from,valid_until；返回batch_id/job_id|幂等键；卡密发行交付另做受控方案，不接演示站导出|
|GET /api/admin/vouchers|batch_id,active,redeemed,cursor,limit|默认不返回凭证材料|
|POST /api/voucher-sessions|code,secret,challenge_proof；返回短期受限访问凭证|新系统身份设计，不等同原站yzm或Cookie机制|
|GET /api/redemption-options|session凭证；可选商品与规则版本|会话限制到特定权益|
|POST /api/redemptions|rule_version,items[{product_id,quantity}],recipient,requested_ship_date|幂等/并发/库存事务；返回order_no与status|
|GET /api/orders/{order_no}|经独立查询授权；返回脱敏订单及物流|不直接复制“手机号即授权”的假设|
|POST /api/admin/inventory-transactions/{id}/approve|version；返回审核结果|幂等、权限、审计；批准与账本变化原子化|
|POST /api/admin/orders/{id}/shipments|carrier_id,tracking_no,version|只在自有沙箱开发测试；事件与物流适配器隔离|

**DESIGN_DECISION**：统一错误结构`{code,message,field_errors,request_id}`；400格式错误、401未认证、403无权限、409并发/重复冲突、422业务拒绝，5xx可重试系统错误。字段及状态约定属于新系统，不能映射为原站数字code/results。客户端失败也必须恢复可操作状态，不复制旧代码潜在卡住问题。
