# 03 52页原型与真实后台对照

**PROTOTYPE_OBSERVED**：原型含52个`script[type="text/html"][data-page]`模板。[原型](../admin-prototype/后台原型（单文件版）.html)第577–732行；菜单约753行起，模拟登录约1033行，Toast约861行。原型是PAGES注入DOM的单文件演示，不代表真实API/权限。

**LIVE_OBSERVED**：52个模板标题全部能在真实外壳菜单找到同名入口；这只是名称映射。不存在“52页均已验证”的结论。原型之外另打开商品选择、商品编辑、订单详情、订单编辑、人工发货、角色编辑等内容页面。[真实菜单](evidence/initial-dom.json)

|编号|原型模板（标题）|真实路径|调查状态|证据|
|---|---|---|---|---|
|M01|`area_list.html` 行政区域列表|`/admin/area/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M02|`auth_admin.html` 管理员列表|`/admin/auth/admin.html`|真实存在但当前账号无权访问|[DOM](evidence/admins-dom.json)|
|M03|`auth_group.html` 角色管理|`/admin/auth/group.html`|真实存在并可访问|[DOM](evidence/roles-edit-entry-dom.json)|
|M04|`code_batch_add.html` 批量生成卡券|`/admin/code/batch_add.html`|真实存在并可访问|[DOM](evidence/voucher-multi-dom.json)|
|M05|`code_batchdel.html` 批量删除卡券|`/admin/code/batchdel.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M06|`code_batchedit.html` 批量修改卡券|`/admin/code/batchedit.html`|真实存在并可访问|[DOM](evidence/voucher-edit-batch-dom.json)|
|M07|`code_card_reset.html` 卡券初始化|`/admin/code/card_reset.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M08|`code_editpass.html` 修改卡券密码|`/admin/code/editpass.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M09|`code_jiami.html` 卡券加密|`/admin/code/jiami.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M10|`code_list.html` 卡券列表|`/admin/code/index.html`|真实存在并可访问|[DOM](evidence/voucher-pagination-entry-dom.json)|
|M11|`code_log.html` 发送记录|`/admin/code/log.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M12|`code_status.html` 卡券激活|`/admin/code/status.html`|真实存在并可访问|[DOM](evidence/voucher-activation-dom.json)|
|M13|`config_chushi.html` 系统初始化|`/admin/config/chushi.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M14|`config_filemanage.html` 上传文件管理|`/admin/config/filemanage.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M15|`config_log.html` 系统日志管理|`/admin/config/log.html`|真实存在但当前账号无权访问|[DOM](evidence/system-log-dom.json)|
|M16|`config_sys_data.html` 数据备份恢复|`/admin/config/sys_data.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M17|`config_system.html` 系统参数配置|`/admin/config/system.html`|真实存在并可访问|[DOM](evidence/settings-dom.json)|
|M18|`export_card.html` 导出电子卡券|`/admin/export/card.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M19|`export_code.html` 批量导出卡券|`/admin/export/code.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M20|`export_index.html` 导出文档管理|`/admin/export/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M21|`export_wxqrcode.html` 导出小程序码|`/admin/export/wxqrcode.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M22|`import_batch_one.html` 批量导入卡券|`/admin/import/batch_one.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M23|`inbound_add.html` 商品入库|`/admin/inbound/add.html`|真实存在并可访问|[DOM](evidence/inbound-add-dom.json)|
|M24|`inbound_list.html` 入库审核|`/admin/inbound/index.html`|真实存在并可访问|[DOM](evidence/inbound-list-dom.json)|
|M25|`message_list.html` 在线反馈|`/admin/message/index.html`|真实存在并可访问|[DOM](evidence/feedback-dom.json)|
|M26|`order_df.html` 待发订单|`/admin/order/index_df.html`|真实存在并可访问|[DOM](evidence/shipping-form-entry-dom.json)|
|M27|`order_export.html` 导出订单|`/admin/order/export.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M28|`order_kuaidi.html` 导入快递单|`/admin/order/kuaidi.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M29|`order_list.html` 全部订单|`/admin/order/index.html`|真实存在并可访问|[DOM](evidence/order-list-dom.json)|
|M30|`order_log.html` 订单日志|`/admin/order/log.html`|真实存在但当前账号无权访问|[DOM](evidence/order-log-dom.json)|
|M31|`order_sh.html` 售后订单|`/admin/order/index_sh.html`|真实存在并可访问|[DOM](evidence/order-aftercare-dom.json)|
|M32|`order_tihuo.html` 到店提货|`/admin/order/tihuo.html`|真实存在并可访问|[DOM](evidence/order-pickup-dom.json)|
|M33|`order_wc.html` 完成订单|`/admin/order/index_wc.html`|真实存在并可访问|[DOM](evidence/order-complete-dom.json)|
|M34|`order_yf.html` 已发订单|`/admin/order/index_yf.html`|真实存在并可访问|[DOM](evidence/order-shipped-dom.json)|
|M35|`order_zf.html` 回收站|`/admin/order/index_zf.html`|真实存在并可访问|[DOM](evidence/order-trash-dom.json)|
|M36|`outbound_add.html` 商品出库|`/admin/outbound/add.html`|真实存在并可访问|[DOM](evidence/outbound-add-dom.json)|
|M37|`outbound_list.html` 出库审核|`/admin/outbound/index.html`|真实存在并可访问|[DOM](evidence/outbound-list-dom.json)|
|M38|`product_add.html` 添加商品|`/admin/product/add.html`|真实存在并可访问|[DOM](evidence/product-add-dom.json)|
|M39|`product_list.html` 商品列表|`/admin/product/index.html`|真实存在并可访问|[DOM](evidence/product-list-dom.json)|
|M40|`sms_list.html` 短信列表|`/admin/sms/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M41|`sms_send.html` 发送短信|`/admin/sms/send.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M42|`tools_decode.html` 二维码解码器|`/admin/tools/decode.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M43|`tools_makecode.html` 二维码生成器|`/admin/tools/makecode.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M44|`trace_add.html` 新增物流|`/admin/trace/add.html`|真实存在并可访问|[DOM](evidence/shipping-add-dom.json)|
|M45|`trace_list.html` 物流管理|`/admin/trace/index.html`|真实存在并可访问|[DOM](evidence/shipping-list-dom.json)|
|M46|`user_list.html` 用户列表|`/admin/user/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M47|`warehousing_list.html` 库存明细|`/admin/warehousing/index.html`|真实存在并可访问|[DOM](evidence/inventory-dom.json)|
|M48|`wechat_menu.html` 自定义菜单|`/admin/wechat/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M49|`wechat_news.html` 图文素材|`/admin/response/news.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M50|`wechat_response.html` 回复资源管理|`/admin/response/index.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M51|`wechat_template.html` 消息模板|`/admin/wechat/template_msg.html`|尚未调查（菜单已观察）|[DOM](evidence/initial-dom.json)|
|M52|`welcome.html` 后台首页|`/admin/index/welcome.html`|真实存在并可访问|[DOM](evidence/welcome-deep-dom.json)|

## 已确认差异

|项|原型证据|真实证据|影响|
|---|---|---|---|
|卡券类型|PROTOTYPE_OBSERVED：普通卡券/电子卡券|LIVE_OBSERVED：card_type=1多选一、2多选多；切换2显示product_num。[前](evidence/voucher-batch-deep-dom.json)、[后](evidence/voucher-multi-dom.json)|必须改正权益模型，不能将电子卡展示载体当兑换类型|
|商品/物流编码|PROTOTYPE_OBSERVED：商品普通编码、物流两种平台代码|LIVE_OBSERVED：商品spec_no/spec_name及物流t_code_wdt；订单列表旺店通列。[商品](evidence/product-edit-dom.json)、[物流](evidence/shipping-add-dom.json)|独立记录外部系统编码，不等同SKU体系已确认|
|角色权限粒度|PROTOTYPE_OBSERVED：少量模块复选框|LIVE_OBSERVED：角色编辑rule[]包含查看、确认、排序、导出等细粒度操作。[角色](evidence/role-edit-dom.json)|新系统需要服务端操作权限；未验证授权保存|
|登录与数据|PROTOTYPE_OBSERVED：非空输入即可隐藏登录层、固定演示数据/Toast|LIVE_OBSERVED：现有用户会话、真实HTML请求、3个页面权限受限|模拟成功不作真实证据|
|菜单可见性|PROTOTYPE_OBSERVED：模板可随意切换|LIVE_OBSERVED：管理员列表、订单日志、系统日志入口可见但拒绝|前端显示和服务端授权分开|

**INFERRED**：25个模板对应菜单尚未调查，并非缺失或无权限。导出、短信发送、初始化、备份恢复及非核心微信工具未作为只读核心研究扩展，不能推断其功能。历史报告未被修订。
