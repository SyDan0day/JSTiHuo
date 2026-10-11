# 04 商品管理

## 实际读取

**LIVE_OBSERVED**：从真实菜单进入`/admin/product/index.html`，表头为缩略图、品名、编码、卡券数量、库存、有效期、可拖动排序、状态、操作；筛选有`name`和`zt`（1启用/0停用）。正常打开`?zt=1`返回200 HTML；没有改排序或状态。[列表](evidence/product-list-deep-dom.json)、[筛选请求](evidence/product-filter-network.json)

**LIVE_OBSERVED**：打开添加页`/admin/product/add.html`和列表实际编辑入口`/admin/product/edit.html?id=[REDACTED]`，均可读，未保存。[添加DOM](evidence/product-add-deep-dom.json)、[编辑DOM](evidence/product-edit-dom.json)、[添加截图](evidence/product-add-deep-masked.png)

|字段/组件|来源与可确认含义|
|---|---|
|pro_name / pro_img / pro_content|LIVE_OBSERVED：商品名称、主图、富文本详情|
|spbm|LIVE_OBSERVED：商品编码；唯一性UNKNOWN|
|spec_no / spec_name|LIVE_OBSERVED：旺店通规格编码/名称；提示须与旺店通对应；不证明存在独立SKU管理|
|s_time / e_time|LIVE_OBSERVED：兑换时间起止|
|no_area|LIVE_OBSERVED：不发货地区，带选择地区和清空选择控件|
|p_sort / status|LIVE_OBSERVED：排序和是否启用；不是已执行上下架|
|parameter0[] / parameter1[]|LIVE_OBSERVED：编辑页的属性名/值输入；SOURCE_OBSERVED：新增页点击添加属性后动态创建同名数组字段|
|轮播图片|LIVE_OBSERVED：添加图片入口；SOURCE_OBSERVED：客户端提示最多8张；未上传图片|
|pid / type|LIVE_OBSERVED：编辑表单额外字段，当前值未记录；业务语义须结合脚本/服务端验证|

来源同上，动态属性/轮播限制详见[添加页完整内联脚本](evidence/product-add-deep-full-inline.json)最后一段108–150行等；脚本数组下标与行号仅对应捕获副本。

## 调用与规则

**SOURCE_OBSERVED / STATIC_CANDIDATE**：新增`form.on('submit(product)')`判商品名称空，禁用按钮并加载，POST `/admin/product/add_check.html`、serialize `#add-form`；消费`code/msg`，成功回列表，失败提示，回调恢复按钮。图片必填检查处于注释，不能照搬为已执行规则。未见该完整业务回调显式失败兜底；运行时弱网行为UNKNOWN。[脚本](evidence/product-add-deep-full-inline.json)

**SOURCE_OBSERVED / STATIC_CANDIDATE**：编辑POST `/admin/product/update.html`；列表状态开关POST `/admin/product/onestatus.html`，排序POST `/admin/product/update_sort.html`，删除POST `/admin/product/del.html`。因此“改一个排序框”也可能真实写入，本轮未操作。[编辑摘录](evidence/product-edit-source-excerpts.json)、[列表摘录](evidence/product-list-deep-source-excerpts.json)

**INFERRED / UNKNOWN**：商品编码是否唯一、规格与库存关系、前台可见与启用状态的精确关系、失效商品对已发卡权益的影响均未确认。未发现当前菜单有独立“组合商品”页；多商品关系在卡券配置体现，不等同商品套餐实体。

**DESIGN_DECISION**：先实现自有Product目录、属性、媒体和有效期；外部规格编码作为可选集成字段，SKU/组合规则作为扩展。新增→前台展示链须在新系统测试，不能声称原站已完成同名商品的写入验证。WRITE_TEST_REQUIRED：保存商品、上下架、排序、删除及历史卡权益联动。
