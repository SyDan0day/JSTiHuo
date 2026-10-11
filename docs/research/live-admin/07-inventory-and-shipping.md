# 07 库存与发货

## 库存和审核

**LIVE_OBSERVED**：库存明细列包括累计库存、已用库存、可用库存、入库审核、出库审核、警戒线、出入库记录。入/出库列表列包括商品ID、编码、数量、审核状态、方式、备注、凭证、创建时间和单号。[库存DOM](evidence/inventory-dom.json)、[入库](evidence/inbound-list-dom.json)、[出库](evidence/outbound-list-dom.json)

**LIVE_OBSERVED**：新增入/出库表单字段分别为`order_sn/in_pz/in_type/order_info`和`order_sn/out_pz/out_type/order_info`，类型1实物/2虚拟，选择商品区域显示现有库存和数量。没有选择商品、上传凭证或保存。[入库表单](evidence/inbound-add-dom.json)、[出库表单](evidence/outbound-add-dom.json)

**SOURCE_OBSERVED / STATIC_CANDIDATE**：submit(user)分别POST `/admin/inbound/add_check.html`、`/admin/outbound/add_check.html`，serialize表单；审核入口可见，但没有执行审核。不能据按钮推断“审核时才扣库存”。[入库脚本](evidence/inbound-add-source-excerpts.json)、[出库脚本](evidence/outbound-add-source-excerpts.json)

**INFERRED / UNKNOWN**：增加、预占、扣减、释放的实际时点，实物与虚拟的账本含义、可用库存公式、负库存政策、审核可逆性、兑换并发和多商品原子扣减都没有事务证据。WRITE_TEST_REQUIRED，须专用可恢复测试商品、库存单及订单。

## 物流配置与人工发货

**LIVE_OBSERVED**：物流新增表单含`t_name/t_cname/t_code/t_code_kd100/t_code_wdt/partnerid/partnerkey/t_url/t_sort/t_default`，分别对应名称/别名、快递鸟/快递100/旺店通代码、月结账号、面单密钥、网址、排序、默认。只保留字段名，不保留账号密钥值。真实表单多出旺店通代码。[物流新增](evidence/shipping-add-dom.json)、[列表](evidence/shipping-list-dom.json)

**LIVE_OBSERVED**：从待发订单的现有人工发货入口打开`/admin/order/fahuo.html?id=[REDACTED]`。业务输入含快递单号`t_no`、物流公司`t_trace`；还携带订单/商品/收货字段`id/order_sn/order_sns/order_bianma/bianmas/pid/product/products/xm/xms/phone/phones/address/addresss`。重复字段的服务端用途未确定，不机械复制为数据库列。[人工发货DOM](evidence/shipping-manual-form-dom.json)

**SOURCE_OBSERVED / STATIC_CANDIDATE**：人工发货submit(code)→serialize `#edit-form`→POST `/admin/order/fahuo_check.html`。待发页另有`fahuo_auto.html`表单目标、批量面单及批量发货端点，但未打开自动发货或点击获取面单，避免触发供应商操作。[人工脚本](evidence/shipping-manual-form-source-excerpts.json)、[待发脚本](evidence/order-pending-deep-source-excerpts.json)

**INFERRED / UNKNOWN**：自动发货如何调用供应商、单号校验、是否支持多包裹、通知发生时机、轨迹供应商与真实响应均未知。未调用快递鸟/快递100/旺店通服务，没有实际发货、打印或短信/微信通知。WRITE_TEST_REQUIRED：发货保存、自动面单、订单状态联动；供应商侧应使用沙箱或本地替身。
