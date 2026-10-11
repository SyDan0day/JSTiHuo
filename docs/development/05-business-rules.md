# 05 规则常量、字段与异常

**DESIGN_DECISION**；未经注明的默认值都是新系统规则，不是原站数值。ADR是决策来源，本文件为实施常量。

|规则ID|确定默认|错误码/验收|
|---|---|---|
|BR01|ONE_OF恰选1；N_OF恰选N种，N=2..99，每项quantity=1，去重；集合至少N|SELECTION_INVALID；T04|
|BR02|一卡单次提交生成一个订单、一个地址、一个日期；不分次不拆原发包|ALREADY_REDEEMED；T05|
|BR03|提交事务核销；鉴真/选品不核销不占库存；取消不恢复卡|T05/T07|
|BR04|available=on_hand-reserved>=0；下单预占/发货双减/取消释放|OUT_OF_STOCK；T06–T09|
|BR05|Asia/Shanghai；最早今天+1天，今天>=17点则+2天；最晚今天+30天；不含blackout|DATE_UNAVAILABLE；T10|
|BR06|每日100单容量，0=关闭；每订单占1，不按件数；取消释放，发货不释放|CAPACITY_EXCEEDED；T11|
|BR07|卡有效期检查当前时刻[from,until)；商品还要求预约日当地00:00位于商品有效区间；不要求预约日早于卡有效期|VOUCHER_UNAVAILABLE/DATE_UNAVAILABLE；T12|
|BR08|禁配按省/市/区任一级命中即拒绝；多个商品禁配取并集|REGION_RESTRICTED；T13|
|BR09|订单只准改地址/日期/备注；PENDING、预约日前一天17点前；禁商品替换|ILLEGAL_TRANSITION/VERSION_CONFLICT；T14|
|BR10|取消管理员操作且PENDING无未决运单任务；卡不恢复|ILLEGAL_TRANSITION；T07|
|BR11|商品启用不要求有库存；下架不影响既有订单出库但阻止新兑|T03|
|BR12|批次1..1000张；批量操作最多500；原子生成无部分业务成功|VALIDATION_FAILED；T02|
|BR13|凭证导出24h；seal立即拒绝下载并销毁短期材料；密码重置仅未兑停用卡|EXPORT_EXPIRED；T15|
|BR14|入/出库仅审核生效；出库不得占用预占量；拒绝无变化|T08|
|BR15|订单主状态PENDING/SHIPPED/COMPLETED/CANCELLED；售后独立；已发不能取消|ILLEGAL_TRANSITION；T09/T16|
|BR16|生产通知默认DISABLED；Outbox重试有上限，超时未知先对账|T17|
|BR17|商品编码、卡号、订单号大小写规范后唯一且删除不复用|409冲突；T02/T03|
|BR18|无金额结算、现金退款、找零；展示价格如将来加入只作文本属性|不创建支付端点|

## 字段统一

卡号trim后大写ASCII；secret原样比较，6..32字符。新卡密码只允许生成长度6/8/9/10/11/12/16/24/32，字母为大写无易混字母表，默认12位混合；导入同长度白名单。卡号总长12..24；随机尾段至少8字符。产品name1..120 Unicode码点、code1..64 ASCII，属性最多20组(name1..40,value1..120)，轮播最多8+主图1。有效期可空表示商品不额外限时；卡有效期不可空。

中国大陆收货手机11位ASCII数字，生产服务端要求^1[3-9][0-9]{9}$；测试fixture号码仅在APP_ENV=test配合MOCK使用10000000000，绝不交付真实短信供应商。省/市/区须来自部署版本的地区数据且父子关系匹配，姓名1..40、详细地址5..200，不接受只有空白。公开反馈email选填，填则标准邮箱解析；message1..200统一计Unicode码点。HTML属性转义、正文服务端allowlist清洗；去除script/iframe/style/事件处理器/javascript URL。

## 冲突与边界

所有编辑字段version乐观锁；0行更新409，客户端重新加载并展示变化不静默覆盖。列表search不支持任意SQL排序；固定id DESC。API规范化后再生成HMAC请求摘要；数组按业务排序（items/product_ids升序）后摘要；地址字段trim，secret不trim且永不进入审计。

日期服务器计算，前端禁用日期只是提示；修改日历配置不能让used>capacity，已有订单保留原日期。商品期限当前时刻和预约日双校验；下单之后期限变化不撤销履约。过期卡鉴真可显示状态但不得兑换；已兑卡仅能用于订单step-up身份确认，不再创建订单。

部分规则跨字段（N_OF至少2、起止时间、父子地区、图片READY）由服务端验证并在Schema说明/合同测试覆盖，不能只依赖OpenAPI单字段min/max。具体测试表见15。

## 实施时消除歧义的常量

DESIGN_DECISION：混合密码字母表为ABCDEFGHJKLMNPQRSTUVWXYZ23456789，LETTERS只用前述字母，DIGITS用0123456789；按crypto/rand拒绝采样，不能用取余造成偏差。卡号随机尾段用ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789，至少8字符；sequential区间起始值按十进制字符串处理，禁止JavaScript Number计算24位序号。

服务端产生业务号：订单为O_+26位大写ULID、批次B_+ULID、库存单I_+ULID；数据库唯一索引兜底、冲突重试最多3次。ULID使用密码学随机熵，不依赖它作为认证秘密。订单号带下划线，与仅大写字母数字卡号区分；queryOrder按O_前缀识别订单，其余按卡号查关联兑换。客户端不能指定order_no或batch_no。

BR05表中为默认值；配置生效时最早日=上海today+min_lead_days+(当前小时>=cutoff_hour?1:0)，且不得为过去日期，最晚today+horizon_days（v1上限30）；默认min_lead_days=1/cutoff_hour=17/horizon_days=30。读取与提交使用同一规则版本，配置变化使预览失效时返回DATE_UNAVAILABLE要求重选，不接受客户端自报可用日期。

商品ProductUpdate采用完整可编辑快照提交：必填字段均发送；可选category_id/redeem_from/redeem_until省略表示清空对应可空字段。这点不同于一般稀疏PATCH，前端必须先读取完整商品并带version再编辑；不能仅提交一个name。其他Update类遵循各Schema的required；未定义的稀疏修改一律400。有效期上下界允许单边，但两个同时存在要求from<until。
