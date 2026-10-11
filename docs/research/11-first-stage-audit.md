# 11 第一阶段成果审核

审核日期：2026-10-11。范围为完整读取01–10文档，核验其本地证据、引用、页面映射和静态调用片段；本审核没有联网、重新浏览页面或改动第一阶段文件。第二阶段新增结果应另查12–20。

标签沿用第一阶段：**OBSERVED**指已有浏览器证据中确实记录的内容；**INFERRED**指本地文档/源码审核与归类，不宣称重新执行浏览器操作；**UNKNOWN**指证据不足。审核程序的文件存在、大小与哈希结果属于本地可复核事实，统一在本文标为INFERRED以免混同网站运行行为。

## 完整性与来源

**INFERRED｜审核通过**：[manifest](evidence/manifest.json)列出的131项均存在，文件字节数及SHA-256全部匹配；01–10中385处本地Markdown链接均有目标。63张PNG存在且尺寸为桌面/操作图35张3022×1474、移动28张600×1300，与08说明一致。哈希只证明此次文件与交付清单一致，不能证明源站真实性、当前状态或截图操作的完整性。逐项结果见[audit-check.json](evidence/phase2/audit-check.json)。本次检查不声称逐张重新做了视觉审查。

**INFERRED｜审核通过**：[页面JSON](evidence/page-observations.json)含26条不同URL；每条均有AX、桌面和移动截图文件，共52张页面基础截图，其余11张为交互/网络证据。20份独立HTTP HTML均有[元数据](evidence/network/public-http-metadata.json)与文件；这些并非浏览器捕获。CSS单独保存；第一阶段没有外部JS源码文件。因此06是HTML中脚本引用及内联代码分析，不能称作已解包完整前端。

**OBSERVED｜原证据**：N01白名单明确记录公告GET、200、HTML Content-Type；其[Headers](evidence/network/N01-notice-headers-whitelist.txt)、[Response](evidence/network/N01-notice-response.html)、[Initiator](evidence/network/N01-notice-initiator.txt)存在。第一阶段“1条公告文档请求、0条业务JSON API”口径成立；它不是“没有业务API”。

## 26 URL → 19 内容页面

**INFERRED｜计数成立**：按浏览器记录的可见主要内容归组，7组各有2入口，另12组各1入口，合计26 URL / 19内容组。此合并不是服务器重定向、同一控制器、同一权限或字节完全相同的证明。每个P编号的完整URL与证据见[01](01-page-inventory.md)、[02](02-page-details.md)。

|内容组|页面记录|相对路径（共同域名demo.haotihuo.com）|
|---|---|---|
|首页|P01、P26|`/`；`/home/index/index.html`|
|鉴真|P02|`/home/index/card_verify.html`|
|预约入口|P03、P23|`/home/index/card_exchange.html`；`/home/index/card_exchange`|
|订单查询入口|P04、P24|`/home/index/order_query.html`；`/home/index/order_query`|
|售后反馈入口|P05、P25|`/home/index/feedback.html`；`/home/index/feedback`|
|公开礼品列表|P06、P07|`/home/index/product.html`；`/home/index/product_order`|
|关于|P08、P21|`/home/index/about_us`；`/home/index/about_us.html`|
|联系|P09、P22|`/home/index/contact_us`；`/home/index/contact_us.html`|
|十个详情，分别计组|P10–P19|`/home/index/product_detail.html?id=`分别为57、17、84、88、13、86、78、92、93、94|
|公告|P20|`/home/index/notice.html`|

**UNKNOWN**：26页之外的受卡券限制页面；`product_order`是否在其他会话下呈现不同内容；无后缀与.html入口的后端路由等价性。19组是本次显示内容分类，不应作为整个系统页面总数。

## 已确认与推断边界

|类别|审核结论|证据|
|---|---|---|
|导航、展示、移动菜单、公告、轮播切换|**OBSERVED**有AX/截图记录；不是卡券兑换成功|[03](03-ui-components.md)、[04](04-navigation.md)|
|四种空输入错误、反馈手机号格式错误|**OBSERVED**分别有AX和截图；不能证明服务器响应|[鉴真](evidence/ax/02-verify-empty.txt)、[预约](evidence/ax/03-exchange-empty.txt)、[查询](evidence/ax/04-order-empty.txt)、[反馈空](evidence/ax/05-feedback-empty.txt)、[反馈格式](evidence/ax/05-feedback-invalid-phone.txt)|
|成功鉴真、进入订单页、反馈成功回跳|**INFERRED**只有静态分支，不是实测成功|[05](05-api-inventory.md)、相关HTML|
|多商品、多卡型、可修改未发货订单|**OBSERVED**仅指页面声称；真实支持范围 **UNKNOWN**|[07](07-business-workflows.md)|
|一卡多商品权益、核销、物流状态、售后流转|**UNKNOWN**，第一阶段没有将宣传升级为已实现规则|[07](07-business-workflows.md)、[09](09-unknowns.md)|
|传统多页面、DOM结构、框架使用|**INFERRED**基于完整HTML、链接及脚本声明；后端栈仍 **UNKNOWN**|[06](06-frontend-architecture.md)|

## 三个Ajax与两个表单目标复核

下表均为 **INFERRED / STATIC_CANDIDATE**；函数名是对匿名回调的定位描述，不是原站存在的命名函数。实际请求状态、Content-Type、完整响应与认证要求仍 **UNKNOWN**。

|候选|调用函数/代码位置|方法、字段与后续|
|---|---|---|
|`/home/index/check.html`|鉴真`$('#smt').click(function(){…})`：verify466/487；预约同名选择器匿名click：exchange447/469|`$.post`，序列化`#order-form`；`bianma/pass/__token__`。鉴真消费`results/tishi/product`；预约results==1调用`$('#order-form').submit()`|
|`/home/index/order_query_yzm.html`|order445匿名click、466 `$.post`|POST；`bianma_jd/phone/__token__`；code==1调用表单submit，其他code消费msg|
|`/home/index/feedback_send.html`|feedback409匿名click、441 `$.post`|POST；`order_sn/xm/phone/email/message/__token__`；code==1调用clearForm456的reset，再由alert回调跳首页|
|`/home/index/order.html`|exchange185 form action；471 submit|原生表单`method="post"`，不是GET导航；未执行|
|`/home/index/order_info.html`|order183 form action；468 submit|原生表单`method="post"`，不是GET导航；未执行|

来源：[verify](evidence/static/verify.sanitized.html)、[exchange](evidence/static/exchange.sanitized.html)、[order](evidence/static/order.sanitized.html)、[feedback](evidence/static/feedback.sanitized.html)。`yzm`只有id，无name，不能由serialize推定为传参；外部验证脚本是否另外补字段第一阶段未分析。三个主要表单通过`$HTH.onsuccess`延迟1秒再次点击提交，只有静态证据，未证明挑战通过。

## 已知JS URL补齐

**INFERRED**：下面是第一阶段源码出现的全部7种script src字符串，不是7个已执行/成功加载脚本。原始定位清单在[audit-check.json](evidence/phase2/audit-check.json)的script_declarations；普通声明也不能替代Network证据。

|完整URL|声明类别与第一阶段证据|
|---|---|
|`https://demo.haotihuo.com/static/home/js/validate.js?v=88`|普通声明；home19等；未保存脚本源|
|`https://demo.haotihuo.com/static/home/weiui/jquery.min.js`|普通声明；home399等；未保存脚本源|
|`https://demo.haotihuo.com/static/bootstrap/js/bootstrap.min.js`|普通声明；home400等；未保存脚本源|
|`https://demo.haotihuo.com/static/layer/layer.js`|普通声明；home401等；N01另有执行栈线索|
|`https://demo.haotihuo.com/static/home/weiui/swiper.min.js`|部分详情普通声明；detail-57第381行、detail-92第392行等|
|`https://demo.haotihuo.com/static/home/js/html5.js`|home20–23的`[if lt IE 9]`条件注释；不代表现代Chrome加载|
|`https://demo.haotihuo.com/static/home/js/jqthumb.js`|products455 HTML注释；457–470调用也在块注释中，仅残留线索|

## 单独记录的缺漏、表述问题及证据限制

|编号|标签|问题|第二阶段处理方式|
|---|---|---|---|
|A01|**INFERRED**|06仅列脚本名，没有完整URL表；未列jqthumb注释线索，html5条件性未展开|以上补齐，保持注释/条件/普通声明区别，不补造加载事实|
|A02|**INFERRED**|03“已读取7份详情HTML”与最终保存10份详情不是数量矛盾，但只说明那段无form检查覆盖7份|不把该段自动扩为10份；新增3份源已保存，可另行检查|
|A03|**INFERRED**|02套用“没有执行密码显隐测试”等模板到查询/反馈，无密码框，易误读|仅鉴真/预约有密码显隐待测；查询/反馈此项不适用，不当成缺失功能|
|A04|**INFERRED**|08把AX组概括为目标页面子树；`02-verify-script-excerpt.txt`实际是DevTools Elements可访问性差分|将它视为浏览器开发工具源码摘录，非网站运行时DOM完整快照；不影响表单错误AX|
|A05|**UNKNOWN**|09记录REA doctor、Chrome插件、Console限制，但第一阶段manifest没有对应工具原始诊断文件|保留为第一阶段叙述，第二阶段使用新诊断补证；不能回填伪造第一阶段日志|
|A06|**INFERRED**|静态HTML所有value与长数字被脱敏，部分资源名失真|用于逻辑定位，不可当作可运行镜像、完整原始资源清单或默认字段值证据|
|A07|**INFERRED**|02/03/05对源码与实测做了区分；未发现将三个候选写成真实成功API的实质分类错误|继续沿用STATIC_CANDIDATE，同时把请求实测与静态来源分开|
|A08|**UNKNOWN**|移动截图为桌面模拟并缩放，非真机；长页只截视口；没有连续Network/HAR|63张截图不是完整交互覆盖、全页视觉或全站请求证据|

第一阶段原件不修订、不覆盖；这些澄清只新增在本文及审核JSON。

## 管理后台与后续阻塞

**INFERRED｜第一阶段未调查后台**：04明确“未进入…管理端”，10当时边界也排除管理端；26 URL清单、20条HTTP元数据和N01均无`/admin/`。因此没有经过第一阶段验证的商品管理、卡型批次、多商品配置、订单发货、售后或角色权限。新提供的后台登录入口属于第二阶段新增范围，不能倒记为第一阶段成果。

**UNKNOWN｜仍需补证**：验证脚本协议与实际验证码关系、选品/地址/日期页、核销与库存幂等、一卡多商品数量/分次/套装规则、查询结果与物流字段、修改窗口、反馈成功/通知、服务端认证与真实错误码。所需验证条件已在[09](09-unknowns.md)列出：只读或可重置沙箱卡、授权测试订单、明确的无副作用边界、合法后台测试账户。源码分析无法单独确认服务端原子性，也不能替代授权数据。
