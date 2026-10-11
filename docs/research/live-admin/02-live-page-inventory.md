# 02 真实后台页面清单

**LIVE_OBSERVED**：以下是本轮经当前会话实际打开的内容路径，按路径去重；列表筛选、分页及重复补证不另算页面。最外层 `/admin/index/index.html` 是独立外壳，另计1页。请求200不等于有权限，需结合内容判断。

共33个内容路径：30个呈现内容/表单，3个返回权限提示。52原型映射中24个可访问、3个拒绝、25个仅确认菜单。

|路径|结果|观察及证据|
|---|---|---|
|`/admin/auth/admin.html`|LIVE_OBSERVED：权限受限|[DOM](evidence/admins-dom.json) / [请求](evidence/admins-network.json) / [脱敏截图](evidence/admins-masked.png)|
|`/admin/auth/group.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/roles-edit-entry-dom.json) / [请求](evidence/roles-edit-entry-network.json) / [脱敏截图](evidence/roles-edit-entry-masked.png)|
|`/admin/auth/group_edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/role-edit-dom.json) / [请求](evidence/role-edit-network.json) / [脱敏截图](evidence/role-edit-masked.png)|
|`/admin/code/batch_add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/voucher-multi-dom.json) / [请求](evidence/voucher-multi-network.json) / [脱敏截图](evidence/voucher-multi-masked.png)|
|`/admin/code/batchedit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/voucher-edit-batch-dom.json) / [请求](evidence/voucher-edit-batch-network.json) / [脱敏截图](evidence/voucher-edit-batch-masked.png)|
|`/admin/code/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/voucher-pagination-entry-dom.json) / [请求](evidence/voucher-pagination-entry-network.json) / [脱敏截图](evidence/voucher-pagination-entry-masked.png)|
|`/admin/code/status.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/voucher-activation-dom.json) / [请求](evidence/voucher-activation-network.json) / [脱敏截图](evidence/voucher-activation-masked.png)|
|`/admin/config/log.html`|LIVE_OBSERVED：权限受限|[DOM](evidence/system-log-dom.json) / [请求](evidence/system-log-network.json) / [脱敏截图](evidence/system-log-masked.png)|
|`/admin/config/system.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/settings-dom.json) / [请求](evidence/settings-network.json) / [脱敏截图](evidence/settings-masked.png)|
|`/admin/inbound/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/inbound-add-dom.json) / [请求](evidence/inbound-add-network.json) / [脱敏截图](evidence/inbound-add-masked.png)|
|`/admin/inbound/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/inbound-list-dom.json) / [请求](evidence/inbound-list-network.json) / [脱敏截图](evidence/inbound-list-masked.png)|
|`/admin/index/welcome.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/welcome-deep-dom.json) / [请求](evidence/welcome-deep-network.json) / [脱敏截图](evidence/welcome-deep-masked.png)|
|`/admin/message/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/feedback-dom.json) / [请求](evidence/feedback-network.json) / [脱敏截图](evidence/feedback-masked.png)|
|`/admin/order/edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-edit-dom.json) / [请求](evidence/order-edit-network.json) / [脱敏截图](evidence/order-edit-masked.png)|
|`/admin/order/fahuo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/shipping-manual-form-dom.json) / [请求](evidence/shipping-manual-form-network.json) / [脱敏截图](evidence/shipping-manual-form-masked.png)|
|`/admin/order/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-list-dom.json) / [请求](evidence/order-list-network.json) / [脱敏截图](evidence/order-list-masked.png)|
|`/admin/order/index_df.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/shipping-form-entry-dom.json) / [请求](evidence/shipping-form-entry-network.json) / [脱敏截图](evidence/shipping-form-entry-masked.png)|
|`/admin/order/index_sh.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-aftercare-dom.json) / [请求](evidence/order-aftercare-network.json) / [脱敏截图](evidence/order-aftercare-masked.png)|
|`/admin/order/index_wc.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-complete-dom.json) / [请求](evidence/order-complete-network.json) / [脱敏截图](evidence/order-complete-masked.png)|
|`/admin/order/index_yf.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-shipped-dom.json) / [请求](evidence/order-shipped-network.json) / [脱敏截图](evidence/order-shipped-masked.png)|
|`/admin/order/index_zf.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-trash-dom.json) / [请求](evidence/order-trash-network.json) / [脱敏截图](evidence/order-trash-masked.png)|
|`/admin/order/log.html`|LIVE_OBSERVED：权限受限|[DOM](evidence/order-log-dom.json) / [请求](evidence/order-log-network.json) / [脱敏截图](evidence/order-log-masked.png)|
|`/admin/order/orderinfo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-detail-fields-dom.json) / [请求](evidence/order-detail-fields-network.json) / [脱敏截图](evidence/order-detail-fields-masked.png)|
|`/admin/order/tihuo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/order-pickup-dom.json) / [请求](evidence/order-pickup-network.json) / [脱敏截图](evidence/order-pickup-masked.png)|
|`/admin/outbound/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/outbound-add-dom.json) / [请求](evidence/outbound-add-network.json) / [脱敏截图](evidence/outbound-add-masked.png)|
|`/admin/outbound/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/outbound-list-dom.json) / [请求](evidence/outbound-list-network.json) / [脱敏截图](evidence/outbound-list-masked.png)|
|`/admin/product/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/product-add-dom.json) / [请求](evidence/product-add-network.json)|
|`/admin/product/edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/product-edit-dom.json) / [请求](evidence/product-edit-network.json) / [脱敏截图](evidence/product-edit-masked.png)|
|`/admin/product/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/product-list-dom.json) / [请求](evidence/product-list-network.json)|
|`/admin/product/select_list.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/product-selector-dom.json) / [请求](evidence/product-selector-network.json) / [脱敏截图](evidence/product-selector-masked.png)|
|`/admin/trace/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/shipping-add-dom.json) / [请求](evidence/shipping-add-network.json) / [脱敏截图](evidence/shipping-add-masked.png)|
|`/admin/trace/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/shipping-list-dom.json) / [请求](evidence/shipping-list-network.json) / [脱敏截图](evidence/shipping-list-masked.png)|
|`/admin/warehousing/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](evidence/inventory-dom.json) / [请求](evidence/inventory-network.json) / [脱敏截图](evidence/inventory-masked.png)|

## 操作口径

**LIVE_OBSERVED**：从真实外壳菜单的`lay-href`发现目标，在同一授权浏览器的内容iframe导航；详情/编辑目标来自实际行入口和加载脚本，实例标识仅为正常查看在内存传递。没有遍历猜测后台路径。

**LIVE_OBSERVED**：商品`zt=1`正常筛选返回HTML200；卡券点击下一页后地址出现`page=2`并捕获GET200。见[筛选](evidence/product-filter-network.json)、[分页](evidence/voucher-next-observed-network.json)。较早`voucher-page2`尝试未找到控件、无请求，不算分页成功。

**INFERRED**：早期DOM辅助工具把无href链接解析为当前页，不能拿这类fallback地址当导航目标；真实菜单用dataHref，分页以控件data-page和实际请求为准。

**LIVE_OBSERVED**：排序输入/状态开关存在，但未触碰；源码显示它们可能即时POST。表单opened不等于保存权限。详见[调用关系](09-js-callgraph.md)。
