# 11 前后台业务映射

本轮六条链路均未完成原站端到端写入验证。**LIVE_OBSERVED**只覆盖读端和表单/入口；**SOURCE_OBSERVED**覆盖提交候选；关联解释为**INFERRED**。不把字段同名、商品ID相同或原型Toast当作完整交易证据。

|目标链路|真实后台证据|已有前台证据|结论与缺口|
|---|---|---|---|
|添加商品→前台展示|LIVE_OBSERVED商品列表/新增/编辑；SOURCE_OBSERVED add_check/update|历史LIVE_OBSERVED七项展示/十详情|INFERRED部分目录概念对应；未新增/上下架后比较前台，缓存及展示筛选UNKNOWN。[商品](04-product-workflow.md)、[旧清单](../01-page-inventory.md)|
|生成卡券→前台鉴真|LIVE_OBSERVED生成规则表单；SOURCE_OBSERVED batch_code|SOURCE_OBSERVED前台check消费者|未生成新卡，也未用任何列表卡号鉴真；凭证算法、激活/加密门槛服务端行为UNKNOWN。[卡券](05-voucher-workflow.md)、[前台](../15-voucher-workflow.md)|
|卡券商品配置→前台选品|LIVE_OBSERVED多选一/多选多；SOURCE_OBSERVED product集合回填|前台校验后order.html仍无授权成功页证据|后台可配置多商品已确认，消费者选几件/可否分次等仍UNKNOWN；不能把选择器当消费者选品页|
|提交兑换→后台订单|LIVE_OBSERVED后台真实订单列表/详情/编辑字段|SOURCE_OBSERVED前台后续POST，未创建|订单存在不证明本轮发起兑换；拆单、幂等、库存/核销时点UNKNOWN。[订单](06-order-workflow.md)|
|后台处理→前台状态|LIVE_OBSERVED后台筛选0待发/1已发/2完成及售后入口|历史前台查询入口文案，结果页未到达|状态字段可用于设计候选；转换权限/触发器/前台映射UNKNOWN|
|后台发货→前台物流|LIVE_OBSERVED人工发货表单t_trace/t_no；SOURCE_OBSERVED fahuo_check|历史前台物流文案|未发货、未调用物流供应商；轨迹格式、刷新、通知和异常UNKNOWN。[库存发货](07-inventory-and-shipping.md)|

**DESIGN_DECISION**：新系统可独立建立Product→VoucherPolicy→Voucher→Redemption→Order→Shipment闭环，用自有种子数据和物流替身跑集成测试；这是开发验收方案，不是对原站链路的证明。每次从候选升级到LIVE_OBSERVED必须新增真实操作/请求证据，不修改历史标签掩盖当时缺口。
