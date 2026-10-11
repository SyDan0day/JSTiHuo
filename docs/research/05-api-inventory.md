# 05 网络请求与静态接口候选

## 捕获口径

**OBSERVED**：Chrome原生DevTools Network可手动查看，但本次没有完整可调用CDP，也没有连续记录所有操作的网络流。完整保存了1条公告文档请求，实际业务JSON API捕获数为0。不能据此断言站点没有业务API。

独立公开HTTP取回20份HTML的记录在[public-http-metadata.json](evidence/network/public-http-metadata.json)，包括URL、GET、状态与Content-Type；这些不是浏览器Network记录，不计入“已观察业务API”。没有导出HAR、Cookie、Authorization、完整请求头或令牌值。

## N01 实际浏览器请求：系统公告文档

|项目|实际证据 / 限制|
|---|---|
|标签|**OBSERVED**|
|页面与动作|首页 `/`，打开Network，刷新页面，自动公告iframe加载后选择notice请求|
|Request URL|`https://demo.haotihuo.com/home/index/notice.html`|
|HTTP Method / Status|GET / 200 OK|
|Request Content-Type|**UNKNOWN**：未保存该头；不把未保存等同于不存在|
|参数名称/类型|URL无查询参数；请求体未捕获，无可确认的业务参数|
|Response Content-Type|`text/html; charset=utf-8`；Content-Encoding gzip|
|JSON结构|不适用，响应为HTML公告文档|
|功能/时机|首页公告iframe加载；不是兑换API|
|JS Initiator|Network Initiator实际显示`(index):559` → `Pop (index):492` → `layer.js:2` → `jquery.min.js:3` → notice文档；详见原始白名单文本|
|认证|**UNKNOWN**：浏览器请求是否依赖既有会话不能由截图判定；另行无显式凭证HTTP取回成功，只支持该公开GET可独立读取，不推广到POST|
|错误处理|**UNKNOWN**：仅观察200，没有制造失败或重放请求|

证据：[白名单Headers](evidence/network/N01-notice-headers-whitelist.txt)、[Initiator文本](evidence/network/N01-notice-initiator.txt)、[Initiator截图](evidence/screenshots/N01-notice-initiator.png)、[实际Response正文](evidence/network/N01-notice-response.html)。Response保存时移除不相关界面文本，内容为公告HTML。Initiator进一步显示公告加载v1.css；调用栈不代表后端实现。

## 静态候选：没有实际发出的业务请求证据

以下均为 **INFERRED**。完整URL以 `https://demo.haotihuo.com` 为前缀；HTTP方法由源码读取，Status、实际Request/Response Content-Type、真实JSON值和服务端认证全部 **UNKNOWN**。字段按表单serialize推断为字符串，不代表服务端类型。`__token__`值未保存；`yzm`只有id无name，不应虚构成已发参数。

|编号/候选URL|静态方法与参数|静态消费者 / 调用时机|错误与成功分支|源码定位|
|---|---|---|---|---|
|C01 `/home/index/check.html`|POST；`bianma:string`、`pass:string`、`__token__:string`|鉴真/预约点击处理器，在非空与验证流程之后；读取`results`、`tishi`、`product`|鉴真results==1清空卡密并插入提示/商品HTML；==2弹窗；其他弹窗并清空。预约results==1触发表单提交，其余提示|[verify](evidence/static/verify.sanitized.html)466–505；[exchange](evidence/static/exchange.sanitized.html)447–484|
|C02 `/home/index/order_query_yzm.html`|POST；`bianma_jd:string`、`phone:string`、`__token__:string`|查询点击处理器；读取`code`、`msg`|code==1进入表单提交，==2清验证值，其他提示/清空分支|[order](evidence/static/order.sanitized.html)445–480|
|C03 `/home/index/feedback_send.html`|POST；`order_sn:string`、`xm:string`、`phone:string`、`email:string`、`message:string`、`__token__:string`|反馈字段校验后；读取`code`、`msg`|code==1 reset、提示后转index.html，其余提示|[feedback](evidence/static/feedback.sanitized.html)408–457|
|F01 `/home/index/order.html`|form method POST；卡券表单字段（同C01）|预约C01成功分支触发表单submit|下一页内容/状态 **UNKNOWN**|[exchange](evidence/static/exchange.sanitized.html)185、469–471|
|F02 `/home/index/order_info.html`|form method POST；查询表单字段（同C02）|查询C02成功分支触发表单submit|订单详情/物流结构 **UNKNOWN**|[order](evidence/static/order.sanitized.html)183、466–468|

**INFERRED**：Ajax采用`$.post(url, form.serialize(), callback)`，可能遵循jQuery默认URL编码；没有浏览器请求头证据，不将其写成确定Content-Type。源代码消费字段的最小形状如下，**不是实际捕获JSON示例或正式API契约**：

```text
C01: { results: UNKNOWN, tishi: UNKNOWN, product: UNKNOWN }
C02/C03: { code: UNKNOWN, msg: UNKNOWN }
```

**UNKNOWN**：字段是否始终存在、数字/字符串区别、空值、状态码枚举、分页、鉴权、CSRF校验、验证挑战附加参数、限流、异常数据与失败重试。

**OBSERVED**：本次只触发空输入/错误手机号前端提示，见[校验截图](03-ui-components.md)。没有用Network证明每个校验分支绝无请求，因此只写“未捕获业务请求”，不写“服务器未收到任何数据”。**INFERRED**：已读内联代码中这些校验位置先于`$.post`，失败时return false。

**INFERRED**：未见这些片段提供`.fail()`/超时恢复；按钮在网络失败后是否卡住需另测。缺少实际失败请求，不能当成确认缺陷。列表注释中的`order_one.html`与残留反馈脚本属于静态线索，不列作真实API。
