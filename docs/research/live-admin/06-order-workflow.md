# 06 订单管理

**LIVE_OBSERVED**：全部、待发、已发、完成、售后、到店提货及回收站页面可读；订单日志权限受限。全部订单筛选status=0待发货、1已发货、2已完成；这三个值是当前筛选选项，不是完整服务端生命周期。[全部](evidence/order-list-dom.json)、[售后](evidence/order-aftercare-dom.json)、[日志截图](evidence/order-log-masked.png)

## 列表与详情字段

**LIVE_OBSERVED**：各列表共同呈现订单编号、提货卡号、商品、收货人、手机、下单时间、状态；待发包含预约发货和面单，已发/完成包含物流信息，完成有完成时间，回收站有删除时间。全部列表另有旺店通列。筛选字段含`id/phone/product/starttime/endtime/yf_stime/yf_etime/status`，待发还有`dzmd`（0未获取/1已获取）、`t_code`快递选择。[待发](evidence/order-pending-dom.json)、[完成](evidence/order-complete-dom.json)、[回收站](evidence/order-trash-dom.json)

**LIVE_OBSERVED**：从真实行详情入口读取`/admin/order/orderinfo.html?order_sn=[REDACTED]`，键标签包括订单编号、提货商品、收货人、收货地址、下单时间、客户备注、客户投诉、后台备注；存在添加投诉/备注按钮。客户数据未保留，详情截图整体数据表被遮盖，因此字段证据主要是脱敏DOM。[详情标签](evidence/order-detail-fields-dom.json)、[备注字段](evidence/order-detail-dom.json)

**LIVE_OBSERVED**：从待发行编辑入口打开`/admin/order/edit.html?id=[REDACTED]`，可见字段`bianma/order_sn/order_sns/id/product[]/xm/phone/province/city/district/address/order_date`。只查看一条现有订单，不修改字段、不保存；不能把可打开编辑页推广到任何状态都允许修改。[编辑](evidence/order-edit-dom.json)

## 调用与限制

**SOURCE_OBSERVED**：待发页`.getinfo.click`从行`order_sn`属性构造详情iframe；`.editfwm.click`从`oid`构造编辑iframe。实际调查直接使用相同现有行值导航读取，实例参数均脱敏。[列表脚本](evidence/order-pending-deep-source-excerpts.json)

**SOURCE_OBSERVED / STATIC_CANDIDATE**：编辑`form.on('submit(code)')`→serialize `#edit-form`→POST `/admin/order/editcheck.html`。待发页还声明取消、批量编辑、批量面单、批量发货、导出和旺店通同步等端点；只是候选，详见[API清单](08-live-api-inventory.md)。没有发送这些请求。

**LIVE_OBSERVED**：到店提货入口当前有`bianma/pass`输入。原型另画验证码/历史核销表，本轮DOM未见这些同样结构；不能用原型补齐实际页面。[到店页](evidence/order-pickup-dom.json)

**INFERRED / UNKNOWN**：售后/取消/回收站与0/1/2的关系、多商品拆成几张订单、允许修改字段的服务端约束、完成触发来源、订单日志格式和权限规则仍未验证。没有把页面标签当作已观察状态迁移。

**DESIGN_DECISION**：新系统将订单状态、售后状态和软删除分离；订单项保存商品快照，订单修改使用版本检查和审计。WRITE_TEST_REQUIRED：真实创建、编辑、取消、恢复、投诉/备注保存、到店核销、物流/完成状态变化。
