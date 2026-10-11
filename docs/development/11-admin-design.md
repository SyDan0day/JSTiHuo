# 11 管理后台设计

**DESIGN_DECISION**：Vue AdminLayout + Element Plus。下表中的原站状态逐字取自[既有52页矩阵](../research/live-admin/evidence/prototype-live-matrix.json)，不是本轮新浏览结果。真实菜单可见不代表保存权限。旧页面来源为LIVE_OBSERVED；原型布局为PROTOTYPE_OBSERVED；新路由/API/处理方式全部自主设计。原型不决定卡型或成功提示。

## 52模板落地表

对应原型[单文件HTML](../research/admin-prototype/后台原型（单文件版）.html)，每个M编号与19的追踪保持一致。新路由可合并旧模板，不能因此省略操作验收。受权限保护的按钮依然必须在服务端检查。

|证据ID/模板|原站路径与观察状态|新路由|operationId|包/最终处理|
|---|---|---|---|---|
|M01 行政区域列表 `area_list.html`|`/admin/area/index.html`；尚未调查（菜单已观察）|`/admin/regions`|regions|G3；V1只读静态地区；在线编辑延期|
|M02 管理员列表 `auth_admin.html`|`/admin/auth/admin.html`；真实存在但当前账号无权访问|`/admin/admins`|listAdmin/createAdmin/updateAdmin|A2/G3；V1自主权限实现，原站页面拒绝访问|
|M03 角色管理 `auth_group.html`|`/admin/auth/group.html`；真实存在并可访问|`/admin/roles`|listRole/createRole/updateRole/permissions|A2/G3；V1角色与操作权限|
|M04 批量生成卡券 `code_batch_add.html`|`/admin/code/batch_add.html`；真实存在并可访问|`/admin/batches/new`|createBatch/generateBatch/getBatch|C1；V1批次生成|
|M05 批量删除卡券 `code_batchdel.html`|`/admin/code/batchdel.html`；尚未调查（菜单已观察）|`/admin/vouchers`|bulkVouchers|C2；V1仅未兑停用卡软删除|
|M06 批量修改卡券 `code_batchedit.html`|`/admin/code/batchedit.html`；真实存在并可访问|`/admin/vouchers`|bulkVouchers/editVoucher|C2；V1有效期与启停；权益不可改|
|M07 卡券初始化 `code_card_reset.html`|`/admin/code/card_reset.html`；尚未调查（菜单已观察）|`—`|—|—；不实施清除消费记录；以新补偿卡替代|
|M08 修改卡券密码 `code_editpass.html`|`/admin/code/editpass.html`；尚未调查（菜单已观察）|`/admin/vouchers/:id`|resetSecret/sealVoucher|C2；V1未兑停用卡随机重置|
|M09 卡券加密 `code_jiami.html`|`/admin/code/jiami.html`；尚未调查（菜单已观察）|`/admin/batches/:id`|sealBatch/sealVoucher|C2；V1安全封存替代未知加密内部算法|
|M10 卡券列表 `code_list.html`|`/admin/code/index.html`；真实存在并可访问|`/admin/vouchers`|vouchers/getVoucher|C2；V1卡券列表和详情|
|M11 发送记录 `code_log.html`|`/admin/code/log.html`；尚未调查（菜单已观察）|`/admin/notifications`|notifications|G2；V1统一通知记录，不推断原站发送对象|
|M12 卡券激活 `code_status.html`|`/admin/code/status.html`；真实存在并可访问|`/admin/vouchers`|bulkVouchers|C2；V1激活/停用独立于消费|
|M13 系统初始化 `config_chushi.html`|`/admin/config/chushi.html`；尚未调查（菜单已观察）|`—`|—|H2；不提供Web初始化；空库受控迁移CLI|
|M14 上传文件管理 `config_filemanage.html`|`/admin/config/filemanage.html`；尚未调查（菜单已观察）|`/admin/files`|files/uploadFile/downloadFile|G1；V1文件鉴权/受控下载；清理由worker|
|M15 系统日志管理 `config_log.html`|`/admin/config/log.html`；真实存在但当前账号无权访问|`/admin/audit`|auditLogs|G3；V1不可删除的审计；原页面拒绝访问|
|M16 数据备份恢复 `config_sys_data.html`|`/admin/config/sys_data.html`；尚未调查（菜单已观察）|`运维终端`|—|H2；离线备份恢复runbook，无Web还原|
|M17 系统参数配置 `config_system.html`|`/admin/config/system.html`；真实存在并可访问|`/admin/settings`|getConfig/updateConfig|G3；V1固定配置Schema|
|M18 导出电子卡券 `export_card.html`|`/admin/export/card.html`；尚未调查（菜单已观察）|`/admin/batches/:id`|downloadFile|C2/G1；V1短期CSV；电子卡样式渲染延期|
|M19 批量导出卡券 `export_code.html`|`/admin/export/code.html`；尚未调查（菜单已观察）|`/admin/batches/:id`|downloadFile/exportJob|C2/G1；V1短期凭证与脱敏报表分开|
|M20 导出文档管理 `export_index.html`|`/admin/export/index.html`；尚未调查（菜单已观察）|`/admin/files`|files/getJob/downloadFile|G1；V1任务和导出文件|
|M21 导出小程序码 `export_wxqrcode.html`|`/admin/export/wxqrcode.html`；尚未调查（菜单已观察）|`—`|—|V2；小程序码延期，无假按钮|
|M22 批量导入卡券 `import_batch_one.html`|`/admin/import/batch_one.html`；尚未调查（菜单已观察）|`/admin/imports/vouchers`|uploadFile/importJob/commitImport/getJob|G1；V1预检后整批导入|
|M23 商品入库 `inbound_add.html`|`/admin/inbound/add.html`；真实存在并可访问|`/admin/inventory/documents/new?direction=IN`|createStockDocument|B2；V1入库单|
|M24 入库审核 `inbound_list.html`|`/admin/inbound/index.html`；真实存在并可访问|`/admin/inventory/documents?direction=IN`|stockDocuments/approveStockDocument/rejectStockDocument|B2；V1审核后增库存|
|M25 在线反馈 `message_list.html`|`/admin/message/index.html`；真实存在并可访问|`/admin/feedback`|feedbackList/resolveFeedback|F2/G2；V1反馈处理|
|M26 待发订单 `order_df.html`|`/admin/order/index_df.html`；真实存在并可访问|`/admin/orders?status=PENDING`|orders/getOrder/adminOrderEdit/shipOrder/autoLabel|E1/E2；V1待发及处理|
|M27 导出订单 `order_export.html`|`/admin/order/export.html`；尚未调查（菜单已观察）|`/admin/orders`|exportJob/getJob/downloadFile|G1；V1脱敏订单导出|
|M28 导入快递单 `order_kuaidi.html`|`/admin/order/kuaidi.html`；尚未调查（菜单已观察）|`/admin/imports/shipments`|uploadFile/importJob/commitImport/getJob|G1；V1逐行发货结果|
|M29 全部订单 `order_list.html`|`/admin/order/index.html`；真实存在并可访问|`/admin/orders`|orders/getOrder/privateOrder/orderCancel|E1；V1全单查询/受控PII|
|M30 订单日志 `order_log.html`|`/admin/order/log.html`；真实存在但当前账号无权访问|`/admin/audit?entity_type=Order`|auditLogs|G3；V1订单审计；原页面拒绝访问|
|M31 售后订单 `order_sh.html`|`/admin/order/index_sh.html`；真实存在并可访问|`/admin/after-sales`|afterSales/decideAfterSale/replacementShipment|G2；V1售后独立状态|
|M32 到店提货 `order_tihuo.html`|`/admin/order/tihuo.html`；真实存在并可访问|`/admin/pickup`|pickup|E2；V1直接核销出库完成，无物流|
|M33 完成订单 `order_wc.html`|`/admin/order/index_wc.html`；真实存在并可访问|`/admin/orders?status=COMPLETED`|orders/getOrder/orderTrash|E1；V1完成及回收|
|M34 已发订单 `order_yf.html`|`/admin/order/index_yf.html`；真实存在并可访问|`/admin/orders?status=SHIPPED`|orders/getOrder/orderComplete|E1/E2；V1已发及确认完成|
|M35 回收站 `order_zf.html`|`/admin/order/index_zf.html`；真实存在并可访问|`/admin/orders/trash`|orders/orderRestore|E1；V1软删恢复，不清核销|
|M36 商品出库 `outbound_add.html`|`/admin/outbound/add.html`；真实存在并可访问|`/admin/inventory/documents/new?direction=OUT`|createStockDocument|B2；V1出库单|
|M37 出库审核 `outbound_list.html`|`/admin/outbound/index.html`；真实存在并可访问|`/admin/inventory/documents?direction=OUT`|stockDocuments/approveStockDocument/rejectStockDocument|B2；V1不可挪用预占|
|M38 添加商品 `product_add.html`|`/admin/product/add.html`；真实存在并可访问|`/admin/products/new`|createProduct/categories/uploadFile|B1；V1商品字段与图片|
|M39 商品列表 `product_list.html`|`/admin/product/index.html`；真实存在并可访问|`/admin/products`|listProduct/getProduct/updateProduct/productStatus|B1；V1查询编辑上下架；不提供商品硬删|
|M40 短信列表 `sms_list.html`|`/admin/sms/index.html`；尚未调查（菜单已观察）|`/admin/notifications`|notifications/retryNotification/resolveNotification|G2；V1统一通知列表|
|M41 发送短信 `sms_send.html`|`/admin/sms/send.html`；尚未调查（菜单已观察）|`/admin/notifications`|retryNotification|G2；仅失败业务事件重试；任意群发延期|
|M42 二维码解码器 `tools_decode.html`|`/admin/tools/decode.html`；尚未调查（菜单已观察）|`—`|—|V2；任意二维码解码延期|
|M43 二维码生成器 `tools_makecode.html`|`/admin/tools/makecode.html`；尚未调查（菜单已观察）|`/admin/tools/qr`|无网络写接口，浏览器本地编码公开入口URL|G3；V1不含卡密的公开入口二维码|
|M44 新增物流 `trace_add.html`|`/admin/trace/add.html`；真实存在并可访问|`/admin/carriers/new`|createCarrier|E2；V1新增公司/供应商映射|
|M45 物流管理 `trace_list.html`|`/admin/trace/index.html`；真实存在并可访问|`/admin/carriers`|listCarrier/getCarrier/updateCarrier|E2；V1配置及启停|
|M46 用户列表 `user_list.html`|`/admin/user/index.html`；尚未调查（菜单已观察）|`—`|—|V2；微信关注用户管理延期；不是管理员列表|
|M47 库存明细 `warehousing_list.html`|`/admin/warehousing/index.html`；真实存在并可访问|`/admin/inventory`|inventory/inventoryLedger/setInventoryWarning/inventoryReconcile|B2/H1；V1余额流水与对账|
|M48 自定义菜单 `wechat_menu.html`|`/admin/wechat/index.html`；尚未调查（菜单已观察）|`—`|—|V2；微信自定义菜单延期|
|M49 图文素材 `wechat_news.html`|`/admin/response/news.html`；尚未调查（菜单已观察）|`—`|—|V2；微信图文素材延期|
|M50 回复资源管理 `wechat_response.html`|`/admin/response/index.html`；尚未调查（菜单已观察）|`—`|—|V2；微信自动回复延期|
|M51 消息模板 `wechat_template.html`|`/admin/wechat/template_msg.html`；尚未调查（菜单已观察）|`—`|—|V2；微信模板编辑延期；v1固定业务通知模板|
|M52 后台首页 `welcome.html`|`/admin/index/welcome.html`；真实存在并可访问|`/admin`|dashboard|G3；V1明确口径统计|

## 页面和表单行为

列表固定游标翻页，保留筛选到URL（禁止手机号、姓名、卡密、地址入URL）；页大小20/50/100。选择当前页ID，切换筛选清空选择；批量操作显式ID+version，最大500，不能“全选所有匹配结果”隐式扩大作用域。展示加载、空态、失败重试、无权四种状态；超时不显示确定失败。筛选API参数白名单以openapi为准。

商品编辑分基本信息/属性图片/有效期与禁配地区；DRAFT新建，单独发布动作；图片先uploadFile拿READY id再提交。code唯一，编辑展示version；409要求重新加载并人工对比，不覆盖他人改动。分类首版仅创建/读取，已有分类名称不支持在线改名删除。productStatus显式目标状态，原型删除按钮改为下架；订单快照不跟着产品编辑变更。

卡型在 `/admin/voucher-types` 用voucherTypes/createVoucherType；同名新定义是新revision，不修改已发批次快照。批次创建展示选品集合、选择数和每项1件、一次提货；生成异步轮询getBatch，成功后显示下载期限，按钮顺序为下载→封存→激活（若未自动激活）。封存不可逆有二次确认及reason。列表不显示secret，密码重置展示受控导出文件ID；下载弹出管理员复核，不把复核密码写日志。激活与“已提货”两列独立。卡型与详情这些新页面是业务补足，不声称原型52页中都存在。

订单列表默认脱敏；详情 `getOrder`，PII按需 `privateOrder` 复核审计，关闭弹窗释放内存。订单详情显示快照明细、状态、预约日、库存预占/运单、审计入口。编辑仅地址/日期/客户备注；取消/完成/回收/恢复都是独立按钮，按状态启用，但服务端再次校验。物流及售后详情用后台受控读取端点（见09），不复用用户cookie。发货表单选择公司及单号，自动运单只生成待确认结果；UNKNOWN显示人工核对，不提供无条件重试。

库存新增单据不直接改变余额，审核显示单据版本/明细/来源类型，批准/拒绝分权限；警戒值独立表单。售后审批、补发分动作，不允许自由编辑状态下拉框。设置字段按Config类型，不提供任意JSON执行。管理员权限多选只呈现当前操作者可授予集合，最后管理员保护服务端事务执行。

`/admin/jobs/:id` 展示getJob的phase/state/processed/failed；PREVIEW/READY才允许commitImport；APPLY/READY才表示处理结束。脱敏逐行错误下载可追踪，不能把有failed的运单批次显示全部成功。UNKNOWN核对用resolveJob/resolveNotification，必须填写外部核对引用与原因。

## 权限、布局与验收

侧栏按产品、库存、卡券、订单、物流、售后、内容、权限、运维分组；菜单根据会话permissions隐藏，直接URL越权403。权限字符串以OpenAPI x-permission为准，A2初始化提取后种子化；任何新endpoint须先声明权限。导航守卫只做UX，Handler仍鉴权。

桌面1440左栏220px，内容最小宽度不把表格撑出容器；平板768侧栏折叠，390移动只读摘要和纵向表单，长表内部滚动。危险按钮与普通按钮分隔；Toast仅依据已提交成功响应，异步202显示“任务已受理”。表单提交禁双击并复用幂等键，但这不能替代事务。T20遍历所有V1活动路由和按钮；M07/M13等取消功能必须没有可执行入口，V2明确标未提供。
