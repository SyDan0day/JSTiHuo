# 08 真实请求与静态候选

**LIVE_OBSERVED**：本轮共1087个请求起始事件，全部GET；1070个捕获到200响应，其余17个观察窗口未取得响应状态，不能写成失败或200。0 POST。这里包含重复页面加载与静态资源，不是1087个业务API。唯一捕获完整JSON结构的XHR端点为首页统计getcount。逐条证据含页面采集文件、ID、Method、URL、参数类型、Content-Type、Initiator和状态：[网络索引](evidence/network-index.json)。

**LIVE_OBSERVED**：实际菜单导航和表单查看返回HTML；验证码和二维码返回图片。页面错误可以HTTP200返回。下表按admin路径去重，不重复列静态CSS/JS/font/image资源；query值均脱敏。请求参数按URL字符串编码标注，不宣称服务端类型。

|实际路径|Method / Status|参数名|响应类型/结构|证据|
|---|---|---|---|
|`/admin/auth/admin.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[admins-N1](evidence/admins-network.json)|
|`/admin/auth/group.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[roles-deep-N1](evidence/roles-deep-network.json)|
|`/admin/auth/group_edit.html`|GET / 200|id|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[role-edit-N1](evidence/role-edit-network.json)|
|`/admin/code/batch_add.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[voucher-batch-deep-N1](evidence/voucher-batch-deep-network.json)|
|`/admin/code/batchedit.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[voucher-edit-batch-N1](evidence/voucher-edit-batch-network.json)|
|`/admin/code/index.html`|GET / 200|page|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[voucher-list-N1](evidence/voucher-list-network.json)|
|`/admin/code/status.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[voucher-activation-N1](evidence/voucher-activation-network.json)|
|`/admin/code/yzm.html`|GET / 200|无|image/png; charset=utf-8；正文不保存|[voucher-batch-deep-N12](evidence/voucher-batch-deep-network.json)|
|`/admin/config/log.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[system-log-N1](evidence/system-log-network.json)|
|`/admin/config/system.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[settings-N1](evidence/settings-network.json)|
|`/admin/inbound/add.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[inbound-add-N1](evidence/inbound-add-network.json)|
|`/admin/inbound/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[inbound-list-N1](evidence/inbound-list-network.json)|
|`/admin/index/getcount.html`|GET / 200/NOT_CAPTURED|无|application/json; charset=utf-8；见下方JSON结构|[welcome-deep-N17](evidence/welcome-deep-network.json)|
|`/admin/index/welcome.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[welcome-deep-N1](evidence/welcome-deep-network.json)|
|`/admin/message/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[feedback-N1](evidence/feedback-network.json)|
|`/admin/order/edit.html`|GET / 200|id|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-edit-N1](evidence/order-edit-network.json)|
|`/admin/order/fahuo.html`|GET / 200|id|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[shipping-manual-form-N1](evidence/shipping-manual-form-network.json)|
|`/admin/order/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-list-N1](evidence/order-list-network.json)|
|`/admin/order/index_df.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-detail-entry-final-N1](evidence/order-detail-entry-final-network.json)|
|`/admin/order/index_sh.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-aftercare-N1](evidence/order-aftercare-network.json)|
|`/admin/order/index_wc.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-complete-N1](evidence/order-complete-network.json)|
|`/admin/order/index_yf.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-shipped-N1](evidence/order-shipped-network.json)|
|`/admin/order/index_zf.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-trash-N1](evidence/order-trash-network.json)|
|`/admin/order/log.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-log-N1](evidence/order-log-network.json)|
|`/admin/order/orderinfo.html`|GET / 200|order_sn|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-detail-fields-N1](evidence/order-detail-fields-network.json)|
|`/admin/order/tihuo.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[order-pickup-N1](evidence/order-pickup-network.json)|
|`/admin/outbound/add.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[outbound-add-N1](evidence/outbound-add-network.json)|
|`/admin/outbound/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[outbound-list-N1](evidence/outbound-list-network.json)|
|`/admin/product/add.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[product-add-deep-N1](evidence/product-add-deep-network.json)|
|`/admin/product/edit.html`|GET / 200|id|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[product-edit-N1](evidence/product-edit-network.json)|
|`/admin/product/index.html`|GET / 200|zt|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[product-edit-entry-N1](evidence/product-edit-entry-network.json)|
|`/admin/product/select_list.html`|GET / 200|prodid|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[product-selector-N1](evidence/product-selector-network.json)|
|`/admin/qrcode/qrcode.html`|GET / 200|color, size, wishes|image/png；正文不保存|[welcome-deep-N7](evidence/welcome-deep-network.json)|
|`/admin/trace/add.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[shipping-add-N1](evidence/shipping-add-network.json)|
|`/admin/trace/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[shipping-list-N1](evidence/shipping-list-network.json)|
|`/admin/warehousing/index.html`|GET / 200|无|text/html; charset=utf-8；HTML字段另见对应DOM；非JSON|[inventory-N1](evidence/inventory-network.json)|

## 已观察JSON：GET /admin/index/getcount.html

**LIVE_OBSERVED**：welcome正常加载/定时调用，3次完整响应均200、`application/json; charset=utf-8`。无查询参数；GET Request Content-Type未捕获，不写成JSON请求。Initiator：welcome.html `get_today`约601行→jQuery each/ajax/send；初次调用约598行。证据ID `welcome-deep-N17`（其他事务见同文件）。响应只保留以下字段及number类型，不保留值。[原始脱敏事务](evidence/welcome-deep-network.json)

```text
yesterday_order_count, today_order_count, today_df_count, tomorrow_df_count,
up_count, down_count, fwm_count, pro_count, order_count_all, order_count,
trace_count, admin_count, sms_count, sale_count, order_wc: number
```

**INFERRED / UNKNOWN**：字段含义部分可与仪表盘标签对照，但统计口径、分页、异常响应、无登录响应、权限过滤未知；没有脱离授权会话重放。getcount属于管理统计，不证明任何写入交易成功。

## 查询、分页与错误处理

**LIVE_OBSERVED**：商品`zt=1`筛选为GET HTML200；卡券真实下一页触发GET `page=2`、HTML200；分页控件有`data-page=2`，不是普通href链接。[商品](evidence/product-filter-network.json)、[卡券](evidence/voucher-next-observed-network.json)、[分页控件](evidence/voucher-pagination-control-dom.json)。排序没有执行，数据库排序规则未知。

**LIVE_OBSERVED**：管理员列表、订单日志、系统日志用HTTP200返回权限受限内容。没有制造HTTP错误、超时、异常写入以验证恢复行为。17个无响应记录包含切换页面时的未完成捕获，不能等同站点错误。

## 仅由已加载源码发现的候选

**SOURCE_OBSERVED / STATIC_CANDIDATE**：以下端点未因该静态声明被认定为成功请求；完整调用位置和表达式见[候选JSON](evidence/static-candidates.json)。GET也可能有副作用，例如清缓存、默认物流、配置备份恢复，本轮未调用。表单serialize字段见业务文档及相应DOM；返回code/msg等消费者不是已捕获响应类型。

|Method / URL字面量|调用页面|源码定位|
|---|---|---|
|GET `/admin/login/quit.html`|`/admin/index/index.html`|[admins-source-excerpts.json](evidence/admins-source-excerpts.json) inline 0 行12|
|GET `/admin/admin/clearcache.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](evidence/feedback-source-excerpts.json) inline 0 行247|
|POST `/admin/message/messagestatus.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](evidence/feedback-source-excerpts.json) inline 1 行35|
|POST `/admin/message/emptymessage.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](evidence/feedback-source-excerpts.json) inline 1 行55|
|POST `/admin/message/delmessage.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](evidence/feedback-source-excerpts.json) inline 1 行89|
|POST `/admin/message/message_detail.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](evidence/feedback-source-excerpts.json) inline 1 行114|
|POST `/admin/inbound/add_check.html`|`/admin/inbound/add.html`|[inbound-add-source-excerpts.json](evidence/inbound-add-source-excerpts.json) inline 1 行101|
|POST `/admin/inbound/del_choose.html`|`/admin/inbound/index.html`|[inbound-list-source-excerpts.json](evidence/inbound-list-source-excerpts.json) inline 1 行66|
|POST `/admin/warehousing/jjx_choose.html?e=1`|`/admin/warehousing/index.html`|[inventory-source-excerpts.json](evidence/inventory-source-excerpts.json) inline 1 行62|
|POST `/admin/order/order_cancel_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](evidence/order-aftercare-source-excerpts.json) inline 1 行187|
|POST `/admin/order/exportorder_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](evidence/order-aftercare-source-excerpts.json) inline 1 行226|
|POST `/admin/order/exportorder.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](evidence/order-aftercare-source-excerpts.json) inline 1 行236|
|POST `/admin/order/sh_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](evidence/order-aftercare-source-excerpts.json) inline 1 行266|
|POST `/admin/order/orderstatus.html`|`/admin/order/index_wc.html`|[order-complete-source-excerpts.json](evidence/order-complete-source-excerpts.json) inline 1 行154|
|POST `/admin/order/edit_choose.html?e=1`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](evidence/order-detail-entry-final-source-excerpts.json) inline 1 行262|
|POST `/admin/order/dzmd_batch_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](evidence/order-detail-entry-final-source-excerpts.json) inline 1 行313|
|POST `/admin/order/fahuo_batch_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](evidence/order-detail-entry-final-source-excerpts.json) inline 1 行345|
|POST `/admin/order/del_dzmd_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](evidence/order-detail-entry-final-source-excerpts.json) inline 1 行409|
|POST `/admin/order/edit_orderinfo.html`|`/admin/order/orderinfo.html`|[order-detail-fields-source-excerpts.json](evidence/order-detail-fields-source-excerpts.json) inline 1 行4|
|POST `/admin/order/edit_orderinfo.html?type=1`|`/admin/order/orderinfo.html`|[order-detail-fields-source-excerpts.json](evidence/order-detail-fields-source-excerpts.json) inline 1 行25|
|POST `/admin/order/editcheck.html`|`/admin/order/edit.html`|[order-edit-source-excerpts.json](evidence/order-edit-source-excerpts.json) inline 1 行46|
|POST `/admin/order/orderupload.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](evidence/order-list-source-excerpts.json) inline 1 行194|
|POST `/admin/order/orderdownload.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](evidence/order-list-source-excerpts.json) inline 1 行232|
|POST `/admin/order/wdt_del_choose.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](evidence/order-list-source-excerpts.json) inline 1 行272|
|POST `/admin/order/bianma_check.html`|`/admin/order/tihuo.html`|[order-pickup-source-excerpts.json](evidence/order-pickup-source-excerpts.json) inline 1 行17|
|POST `/admin/order/fahuo_cancel.html`|`/admin/order/index_yf.html`|[order-shipped-source-excerpts.json](evidence/order-shipped-source-excerpts.json) inline 1 行329|
|POST `/admin/order/order_cancel.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](evidence/order-trash-source-excerpts.json) inline 1 行138|
|POST `/admin/order/del.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](evidence/order-trash-source-excerpts.json) inline 1 行167|
|POST `/admin/order/order_clear.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](evidence/order-trash-source-excerpts.json) inline 1 行187|
|POST `/admin/order/order_recovery.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](evidence/order-trash-source-excerpts.json) inline 1 行206|
|POST `/admin/outbound/add_check.html`|`/admin/outbound/add.html`|[outbound-add-source-excerpts.json](evidence/outbound-add-source-excerpts.json) inline 1 行101|
|POST `/admin/outbound/del_choose.html`|`/admin/outbound/index.html`|[outbound-list-source-excerpts.json](evidence/outbound-list-source-excerpts.json) inline 1 行65|
|POST `/admin/product/add_check.html`|`/admin/product/add.html`|[product-add-deep-source-excerpts.json](evidence/product-add-deep-source-excerpts.json) inline 1 行122|
|POST `/admin/product/onestatus.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](evidence/product-edit-entry-source-excerpts.json) inline 1 行36|
|POST `/admin/product/update_sort.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](evidence/product-edit-entry-source-excerpts.json) inline 1 行130|
|POST `/admin/product/del.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](evidence/product-edit-entry-source-excerpts.json) inline 1 行154|
|POST `/admin/product/update.html`|`/admin/product/edit.html`|[product-edit-source-excerpts.json](evidence/product-edit-source-excerpts.json) inline 1 行65|
|POST `/admin/auth/group_edit2.html`|`/admin/auth/group_edit.html`|[role-edit-source-excerpts.json](evidence/role-edit-source-excerpts.json) inline 1 行49|
|POST `/admin/auth/group_status.html`|`/admin/auth/group.html`|[roles-deep-source-excerpts.json](evidence/roles-deep-source-excerpts.json) inline 1 行30|
|POST `/admin/auth/group_del.html`|`/admin/auth/group.html`|[roles-deep-source-excerpts.json](evidence/roles-deep-source-excerpts.json) inline 1 行60|
|POST `/admin/config/system_edit.html`|`/admin/config/system.html`|[settings-source-excerpts.json](evidence/settings-source-excerpts.json) inline 3 行329|
|GET `/admin/config/bak.html?bak=1`|`/admin/config/system.html`|[settings-source-excerpts.json](evidence/settings-source-excerpts.json) inline 3 行348|
|GET `/admin/config/recover_bak.html?bak=1`|`/admin/config/system.html`|[settings-source-excerpts.json](evidence/settings-source-excerpts.json) inline 3 行365|
|GET `/admin/config/getrandpass.html?n=`|`/admin/config/system.html`|[settings-source-excerpts.json](evidence/settings-source-excerpts.json) inline 3 行511|
|GET `/admin/wechat/get_template_msg_haotihuo.html?e=`|`/admin/config/system.html`|[settings-source-excerpts.json](evidence/settings-source-excerpts.json) inline 3 行522|
|POST `/admin/trace/addcheck.html`|`/admin/trace/add.html`|[shipping-add-source-excerpts.json](evidence/shipping-add-source-excerpts.json) inline 1 行14|
|POST `/admin/trace/onestatus.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](evidence/shipping-list-source-excerpts.json) inline 1 行38|
|POST `/admin/trace/del.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](evidence/shipping-list-source-excerpts.json) inline 1 行70|
|GET `/admin/trace/setdefault.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](evidence/shipping-list-source-excerpts.json) inline 1 行86|
|POST `/admin/order/fahuo_check.html`|`/admin/order/fahuo.html`|[shipping-manual-form-source-excerpts.json](evidence/shipping-manual-form-source-excerpts.json) inline 1 行22|
|POST `/admin/code/staupdate.html`|`/admin/code/status.html`|[voucher-activation-source-excerpts.json](evidence/voucher-activation-source-excerpts.json) inline 1 行27|
|POST `/admin/code/batch_code.html`|`/admin/code/batch_add.html`|[voucher-batch-deep-source-excerpts.json](evidence/voucher-batch-deep-source-excerpts.json) inline 1 行60|
|POST `/admin/code/onestatus.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](evidence/voucher-list-source-excerpts.json) inline 1 行79|
|POST `/admin/code/sndel.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](evidence/voucher-list-source-excerpts.json) inline 1 行258|
|POST `/admin/code/pass_choose.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](evidence/voucher-list-source-excerpts.json) inline 1 行310|
|POST `/admin/code/exportcode.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](evidence/voucher-list-source-excerpts.json) inline 1 行422|
|GET `/admin/index/getcount.html`|`/admin/index/welcome.html`|[welcome-deep-source-excerpts.json](evidence/welcome-deep-source-excerpts.json) inline 1 行5|

**SOURCE_OBSERVED**：动态拼接URL、iframe导航目标与未命中`$.post/get`的调用不保证全量收集；这不是完整后台API定义。方法、编码、鉴权和响应契约必须独立验证。用户端原有三个Ajax及两个表单仍保持上一阶段STATIC_CANDIDATE身份，后台登录成功会话不自动证明前台兑换成功。
