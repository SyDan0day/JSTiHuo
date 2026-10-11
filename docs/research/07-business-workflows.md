# 07 业务流程与状态证据

本文件的“成功分支”如标 **INFERRED**，仅来自前端源码；没有真实成功订单、核销或物流记录。所有输入字段细节见[03](03-ui-components.md)，候选接口见[05](05-api-inventory.md)。

|业务|入口与步骤|输入|状态/成功/失败证据|实际API与缺失条件|
|---|---|---|---|---|
|卡券鉴真|**OBSERVED** P02打开→清空卡号密码→确定提交→提示→关闭|卡号、密码；验证挑战机制未知|**OBSERVED** 空输入→“请输入卡号”。**INFERRED** 验证中→成功显示tishi/product，或错误弹窗。实际成功及卡券无效/过期/已兑均 **UNKNOWN**|实际业务API0；C01候选。缺只读测试卡、允许的验证条件。证据：[P02 AX](evidence/ax/02-verify-empty.txt)、[源码](evidence/static/verify.sanitized.html)466–505|
|卡券兑换|**OBSERVED** P03预约入口→空输入→提示；未进入下一页|当前仅卡号/密码可见；其他 **UNKNOWN**|**OBSERVED** “请输入卡号”。**INFERRED** check成功→order.html；何时核销、成功回执与恢复均 **UNKNOWN**|实际业务API0；C01/F01候选。缺明确无真实库存/核销副作用的测试卡与沙箱。证据：[P03](evidence/ax/03-exchange-empty.txt)、[源码](evidence/static/exchange.sanitized.html)185、447–484|
|商品选择|**OBSERVED** P06/P07七项列表→打开详情；首页另有三个营销详情|未观察到数量/勾选/购物车输入|**OBSERVED** 只完成浏览详情，未完成兑换选品；选中、超额、缺货、成功加入集合均 **UNKNOWN**|实际业务API0，无已证实选品接口。缺绑定多商品测试卡及选品页。证据：[列表](evidence/ax/06-products.txt)、[详情57](evidence/ax/10-detail-57.txt)|
|预约提货|**OBSERVED** P03为卡券入口；后续日期、地址、确认未进入|已见卡号/密码；收货人、地址、日期字段均 **UNKNOWN**|**OBSERVED** 仅空卡号失败。**INFERRED** 文案谈到发货日期；不能据此确认日期选择器/限制算法。成功、不可选日期、区域限制 **UNKNOWN**|实际业务API0；F01仅静态提交目标。缺测试卡、非生产订单环境与可撤销流程。证据：[P03](evidence/ax/03-exchange-empty.txt)|
|订单查询|**OBSERVED** P04→清空订单号和手机→立即查询→提示|卡号或订单号、收货手机|**OBSERVED** “请输入提货卡号或订单号”。**INFERRED** C02成功→order_info.html。订单不存在、手机号不匹配、成功详情 **UNKNOWN**|实际业务API0；C02/F02候选。缺脱敏测试订单与配套测试手机。证据：[P04](evidence/ax/04-order-empty.txt)、[源码](evidence/static/order.sanitized.html)445–480|
|物流查询|**OBSERVED** P04文案“可查询订单状态和物流轨迹”；没有进入物流界面|是否继承订单查询、是否需运单号 **UNKNOWN**|全部物流状态、承运商、轨迹刷新、异常物流 **UNKNOWN**；页面文案不能证明接口能力|实际API0，无确认物流接口。缺已发货沙箱订单及模拟轨迹。证据：[P04截图](evidence/screenshots/04-order-desktop.png)|
|售后反馈|**OBSERVED** P05空提交→姓名提示；TEST/abc→手机提示→清空|订单号、姓名、手机、邮箱、反馈内容|**OBSERVED** 两类前端失败；**INFERRED** code1→重置/提示/首页，其余msg提示；反馈成功/工单状态 **UNKNOWN**|实际业务API0；C03候选。缺不会发送真实工单/通知的反馈沙箱。证据：[空值](evidence/ax/05-feedback-empty.txt)、[格式错误](evidence/ax/05-feedback-invalid-phone.txt)|

## 一卡多商品：明确未确认的核心规则

**OBSERVED**：关于页出现“多商品/多卡型”营销说明，列表同时展示多个礼品，详情还有“套餐”名称。[关于AX](evidence/ax/08-about.txt)、[列表AX](evidence/ax/06-products.txt)。这些事实只能证明展示/宣传，不能证明同一张卡可兑换几件、怎样组合或分次兑换。

**INFERRED**：预约先校验卡号密码，再提交到后续页面，说明兑换资格与公开展示之间可能存在另一个步骤；该步骤是否就是多商品选择仍 **UNKNOWN**。[预约源码](evidence/static/exchange.sanitized.html)469–471。

|待确认规则|当前结论|需要的最小证据|
|---|---|---|
|多选一、任选N件、固定套装、余额抵扣|**UNKNOWN**|明确标注卡型的测试卡，观察资格响应与选品组件|
|同商品多件、规格、数量上限、搭配约束|**UNKNOWN**|测试卡可选集合、数量错误响应及规则说明|
|一次兑完/分次兑换/剩余额度|**UNKNOWN**|安全沙箱中的兑换前后卡状态与订单关联|
|多地址、多日期、拆单/合单|**UNKNOWN**|沙箱预览、订单结构与业务方规则|
|库存占用/核销时机、并发、失败回滚|**UNKNOWN**|服务端契约和经授权沙箱记录，不能靠前端猜测|
|有效期、缺货替代、退换后卡权益恢复|**UNKNOWN**|明确错误/售后案例与权威规则|

**OBSERVED**：提货须知声称兑换成功后卡号密码失效、不找零不兑现金、日期为发货日期、运输约1–4天等。[查询AX](evidence/ax/04-order-empty.txt)。这是站点文案，既不能确认服务端实现，也不能据“失效”排除多商品一次兑换或推定不存在分次兑换。开发前需要业务方确认；不将历史公告或演示商品属性升格为规则。

**UNKNOWN**：页面所称“可修改未发货订单”的权限、时间窗、字段、接口、状态迁移；本次未执行修改请求。所有成功流程因缺少授权测试条件而停止，没有猜卡密、订单号、验证码或绕过校验。
