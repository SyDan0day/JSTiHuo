# 05 卡券、批次与多商品

## 真实配置与原型差异

**LIVE_OBSERVED**：`/admin/code/batch_add.html`提供`card_type=1 多选一`、`2 多选多`。默认数量框不可见；仅点击“多选多”radio后，`product_num`从visible=false变true，maxLength=2。没有生成卡券，数字长度不是最大可兑换数量的服务端规则。[切换前](evidence/voucher-batch-deep-dom.json)、[切换后](evidence/voucher-multi-dom.json)、[脱敏截图](evidence/voucher-multi-masked.png)

**PROTOTYPE_OBSERVED**：原型同页写普通卡券/电子卡券，不能用于真实权益建模。真实列表另有“电子卡券”展示/发送入口，但不是上述card_type的解释。[对照](03-prototype-vs-live.md)

|字段|LIVE_OBSERVED：表单含义/选项|
|---|---|
|pici|生成批次隐藏字段；不能由前端字段证明独立批次数据库表|
|bname|卡券名称，页面称仅后台展示方便区分|
|card_type / product_num|多选一/多选多与多选多可勾商品数量|
|product / productlist|所属商品隐藏ID集合及名称文本；结构解释见下方源码|
|code_num|生成数量，maxlength8，页面建议单次10万以内；建议非已测硬上限|
|code_pre / sn_start|卡号前缀max4、起始卡号；页面说明总长度=前缀长度+起始号长度|
|code_type|1顺序生成、2随机生成；没有实际生成或评价随机强度|
|pass_len / pass_type|长度选项6/8/9/10/11/12；格式2数字、1字母、0数字和字母|
|b_stime / b_etime|有效期起止|
|status|1是/0否（是否激活）|
|exprot_file / captcha|同时导出文件复选框及验证码；未提交、未识别验证码|

证据：[批量表单](evidence/voucher-batch-deep-dom.json)。所有凭证值都未保留。

## 商品关联与客户端调用

**SOURCE_OBSERVED**：`.getproduct.click`打开`/admin/product/select_list.html?prodid=…`。选择器`.rselect.click`收集`[name='qx_id']:checked`的value到arr、`pro`属性到pro；至少一个，否则alert；随后`parent.$('#product').val(arr)`、`productlist.val(pro)`并关闭层。这是客户端以集合传递商品的证据，不是数据库关系证据，也未实际确认选择。[批量脚本](evidence/voucher-batch-deep-full-inline.json)、[选择器脚本](evidence/product-selector-full-inline.json)

**LIVE_OBSERVED**：正常打开该选择器文档GET200，有复选框、品名/编码/库存/有效期、确定选择；本次直接导航查看，所以没有测试原弹窗尺寸、回填执行或保存。[DOM](evidence/product-selector-dom.json)、[请求](evidence/product-selector-network.json)

**SOURCE_OBSERVED / STATIC_CANDIDATE**：`form.on('submit(batch)')`→序列化`#add-form`→POST `/admin/code/batch_code.html`；消费`code/count/msg/pici`。生成数量判空/前缀长度检查在客户端；源码的最大数量比较仍含模板字面量，不能把它当已确认数值上限。`handleDropdownChange()`按总长减前缀长度生成起始号字符串。未执行提交或覆盖已有卡号。[完整内联](evidence/voucher-batch-deep-full-inline.json)

## 状态、列表与批量入口

**LIVE_OBSERVED**：列表筛选`status`为已激活1/未激活0，`sale`为已提货1/未提货2；列表有名称、商品、批次、有效期、提货时间、状态。点击真实下一页后GET `index.html?page=2`返回200，页面变为第2页。没有保存任何卡号、密码、电子卡或客户资料。[列表](evidence/voucher-list-dom.json)、[分页](evidence/voucher-next-observed-dom.json)

**LIVE_OBSERVED（文案）**：批量页称“只有加密过的卡券才能正常提货”“加密不可逆”；激活页称必须激活才能提货。只证明系统呈现该操作要求，不证明密码加密算法、安全性或核销事务。[批量](evidence/voucher-batch-deep-dom.json)、[激活](evidence/voucher-activation-dom.json)

**LIVE_OBSERVED**：激活页按卡号、批次、名称、日期分组，status为激活1/停用0；批量修改页有按卡号/批次/日期入口。**SOURCE_OBSERVED / STATIC_CANDIDATE**：激活POST `/admin/code/staupdate.html`，单个状态切换POST `onestatus.html`。均未执行。[激活脚本](evidence/voucher-activation-source-excerpts.json)、[列表脚本](evidence/voucher-list-source-excerpts.json)

**INFERRED / UNKNOWN**：不能合并“激活、提货、有效期、加密”成为一个已知状态枚举。过期是否独立标志、停用已兑卡的结果、多选多数量是件数还是不同商品数、能否重复选同商品、分次兑换、拆单/多地址、有效期优先级、退货返还权益仍未知。WRITE_TEST_REQUIRED：生成、激活/停用、加密、核销及其并发/失败规则。[未知清单](12-business-rules-and-unknowns.md)
