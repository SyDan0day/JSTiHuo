# 03 业务领域与状态机

**DESIGN_DECISION**：单租户单仓、无支付。商品是可兑换最小单位，固定礼盒作为一个商品管理。卡型定义不可变策略版本；批次复制策略和商品集合；单卡复制商品ID集合，避免后续类型变化影响已发卡。

## 聚合与所有权

Catalog管理商品和媒体；Voucher管理类型/批次/卡券/凭证；Redemption应用服务协调Voucher、Order、Inventory与Calendar；Order管理订单与快照；Inventory管理余额/单据/账本；Shipping管理运单与事件；Identity管理权限会话。跨域事务由用例Service持有同一sql.Tx，不由模块互相提交事务。异步通知通过Outbox。

## 状态维度

|对象|状态/字段|含义及合法转换|
|---|---|---|
|Product|DRAFT、ENABLED、DISABLED|草稿满足发布条件→启用；启用↔停用；不删除已关联对象|
|Batch|DRAFT→GENERATING→GENERATED→SEALED；GENERATING→FAILED|FAILED可同任务重试；GENERATED后不可改生成配置；SEALED终态|
|Voucher凭证|EXPORTABLE→SEALED|生成/重置后可短期导出；封存才可提货；重置仅未兑停用卡，版本递增后回EXPORTABLE|
|Voucher激活|active=false/true|封存后可激活；生成选“激活”只在封存完成时生效；已兑卡不能再次激活用于兑换|
|Voucher消费|redeemed_at为空→非空|订单创建同事务设置；永不清空。停用/软删/过期不清除消费记录|
|Voucher会话|15分钟受限授权|鉴真建立，不改变消费。每次写操作重查卡状态/版本，不信任前端或会话缓存|
|Order|PENDING→SHIPPED→COMPLETED；PENDING→CANCELLED|到店专用核销事务直接创建COMPLETED/PICKUP；不是通用任意跳转|
|Reservation|ACTIVE→CONSUMED / RELEASED|发货消耗、取消释放；终态不可再变|
|StockDocument|PENDING→APPROVED / REJECTED|审核后不可改，纠错用新反向单|
|Shipment|SHIPPED→IN_TRANSIT/DELIVERED/EXCEPTION|EXCEPTION可后续恢复IN_TRANSIT/DELIVERED；DELIVERED不可被旧事件回退|
|AfterSale|OPEN→APPROVED/REJECTED；APPROVED→CLOSED|COMPLAINT可OPEN→CLOSED；REPLACEMENT批准后补发，送达再关闭；驳回终态|

过期是 now>=valid_until 的派生结果；尚未开始是now<valid_from，不用定时器逐卡写状态。可兑换=未删除+已封存+active+有效期内+未redeemed+批次SEALED。状态显示优先：删除>已兑>停用>未封存>未到期>已过期>可用，但API分字段，不丢失其他维度。

订单显示状态独立于售后和deleted_at。回收站只隐藏CANCELLED/COMPLETED订单，不改变库存/卡状态；恢复只清deleted_at，不把订单复活为待发。订单项快照在下单时冻结；商品改名不改历史订单。

## 订单转换表

|转换|触发者与条件|事务/库存|卡券/通知/审计|
|---|---|---|---|
|无→PENDING|持有效卡会话；恰选数量、日期/地区/容量/库存通过|T-REDEEM；reserved+=qty，calendar.used+=1|核销；order.created Outbox；审计|
|PENDING→PENDING编辑|WRITE订单会话或order.write；version匹配；发货日前一日17:00前|T-EDIT；只改地址/日期/备注；跨日期原子迁移容量，不动库存|卡不变；order.updated事件|
|PENDING→CANCELLED|管理员order.cancel；无已分配运单任务|T-CANCEL；reserved-=qty，calendar.used-=1|卡仍已兑；order.cancelled事件|
|PENDING→SHIPPED|order.ship；到预约日；无活跃售后；全单|T-SHIP；on_hand-=qty,reserved-=qty；预约容量不释放|卡不变；shipment.created事件|
|SHIPPED→COMPLETED|签收回调或管理员有理由确认|T-EVENT/T-STATE；库存不变|卡不变；order.completed事件|
|无→COMPLETED/PICKUP|order.pickup；现场校验卡密与商品；无快递/日历|T-PICKUP；available够后on_hand-=qty，不预占|核销；pickup.completed审计；默认不通知|

不能取消已发货订单；只能售后。不能直接编辑商品集合，修改收货信息需重新校验当前禁配地区；客服如需换品，取消原单后另发新卡，不复用原消费记录。
