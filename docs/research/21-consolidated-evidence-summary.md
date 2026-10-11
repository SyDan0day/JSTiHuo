# JSTiHuo 用户端与管理端证据汇总

更新日期：2026-10-11。项目：`/Users/chenbeier/Codes/JSZN/JSTiHuo`。目标：[好提货高级版演示站](https://demo.haotihuo.com/)。

本文汇总公开用户端调查、REA 静态分析、真实后台只读调查、52页原型对照，以及最近一次获授权的隔离写入测试。它是现有证据的统一阅读入口，不代表本次重新访问网站或完成全部业务验证。保留全部历史文档及证据；不包含账号、密码、Cookie、Token、卡密、客户电话或地址。

## 1. 当前结论与证据口径

**LIVE_OBSERVED**：用户端已访问26个URL，按可见内容合并为19组；后台已实际打开33个内容路径，其中30个呈现页面或表单，3个显示权限限制，后台外壳另计1页。**INFERRED**：可开展公开页面、商品管理及卡券配置界面开发；完整兑换、库存和发货兼容性仍缺实测业务契约。[用户端清单](01-page-inventory.md)、[后台清单](live-admin/02-live-page-inventory.md)

**LIVE_OBSERVED**：目前确有完整JSON类型结构证据的业务/统计端点为后台统计 `GET /admin/index/getcount.html` 和商品新增 `POST /admin/product/add_check.html`。后者返回业务 `code=0`，未观察到创建成功。前台三个Ajax及两个后续POST表单仍未实际执行。[后台网络](live-admin/08-live-api-inventory.md)、[最新写测](write-tests/2026-10-11/02-results.md)

**LIVE_OBSERVED**：真实后台支持“多选一/多选多”的配置界面，并实测了多选多数量框显隐。**UNKNOWN**：消费者选品数量口径、分次兑换、重复选品、拆单、多地址、核销和库存时点。不能把后台配置控件当成完整兑换已验证。[卡券证据](live-admin/05-voucher-workflow.md)

|标签|本文含义|
|---|---|
|LIVE_OBSERVED|已保存的浏览器页面、交互、请求证据；对应早期文档中的浏览器OBSERVED，不表示本次重新执行|
|SOURCE_OBSERVED|实际加载并捕获的JS/前端资源代码；不等于代码分支执行成功|
|PROTOTYPE_OBSERVED|单文件后台原型内容；模拟登录、Toast、演示数据均不能证明真实功能|
|INFERRED|有限证据解释、独立HTTP源码分析或本地审核结论|
|DESIGN_DECISION|拟建新系统的自主设计，不是原站内部结构|
|UNKNOWN / STATIC_CANDIDATE / WRITE_TEST_REQUIRED|分别表示未确认、静态接口候选、仍需受控写入验证；不作为实测标签|

**INFERRED（来源限定）**：第一阶段独立HTTP下载的HTML按历史口径保持INFERRED；第二阶段实际浏览器捕获的脚本可按SOURCE_OBSERVED引用。页面声明的规则仅证明文案存在；HTTP200仅证明响应，不证明权限或业务成功。[第一阶段审核](11-first-stage-audit.md)

## 2. 调查规模、阶段与方法

|阶段|已记录范围|关键限制及来源|
|---|---|---|
|公开用户端|26 URL→19内容组；63截图；4个空值提示、1个非法手机提示，轮播/菜单/公告操作|桌面及390×844模拟移动视口，保存PNG另有缩放，非真机、非全页拼接；1条公告浏览器请求；业务JSON 0。[一期索引](08-evidence-index.md)|
|REA深化|三个核心用户端入口与公开后台登录；脚本捕获/导出、聚焦分析；50 GET、24不同URL、均200|无业务POST；该阶段网络缺响应正文、头值及Initiator，不能补造。[二期网络](14-network-evidence.md)|
|真实后台只读|33内容路径，30可读/3拒绝；52菜单映射；49脱敏截图；1087请求事件|全GET，其中1070捕获200、17状态未捕获；是窗口事件数，不是业务API数。[后台索引](live-admin/14-evidence-index.md)|
|隔离写入测试|一次实际商品新增POST，业务code=0；测试名称查询0匹配|未生成批次/卡券/订单，无已确认新记录需停用。[测试报告](write-tests/2026-10-11/02-results.md)|

**INFERRED**：各阶段范围和记录方式不同，不能直接相加得到全站页面数、唯一API数或完整覆盖率。后台原型映射24可访问+3拒绝+25仅菜单=52，另6个非原型内容路径，故后台实际33路径不矛盾。

**LIVE_OBSERVED（历史工具记录）**：Chrome扩展连接曾超时，前期使用Chrome原生界面/DevTools；REA阶段使用6.4.0 CLI连接用户指定CDP，MCP注册缺失/config drift未通过修改全局配置解决；真实后台阶段使用授权的 `127.0.0.1:9222`，未重复执行已足够的REA分析。第三方包完整语义分析曾受内存限制，聚焦7文件分析不能称全站语义完整。[REA方法](12-rea-js-analysis.md)、[后台方法](live-admin/01-access-and-permissions.md)

## 3. 用户端页面与交互

### 3.1 页面映射

下表为历史浏览器观察的内容分组；合并不证明服务器重定向或后端路由完全等价。实际URL均以 `https://demo.haotihuo.com` 为域名。[26条逐页记录](02-page-details.md)、[页面机器记录](evidence/page-observations.json)

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

### 3.2 已验证的交互与字段

**LIVE_OBSERVED**：首页桌面导航、移动折叠菜单与底栏、轮播切换、公告展示/关闭、礼品图片名称链接及详情滚动均有AX或截图。公开礼品列表不是卡券授权选品页。详情中的通用预约入口不能证明商品详情支持直接兑换。[组件](03-ui-components.md)、[导航](04-navigation.md)

|业务入口|静态字段及校验（INFERRED，非服务端契约）|已观察提示（LIVE_OBSERVED）|
|---|---|---|
|鉴真、预约|bianma：HTML email/max32，但业务是卡号，JS只判非空；pass：password/max20，判非空|空卡号：“请输入卡号”|
|查询|bianma_jd：email/max32；phone：tel/max11；当前内联逻辑判非空，未见手机号格式正则|空标识：“请输入提货卡号或订单号”|
|反馈|order_sn可选/max30；xm必填/max20；phone必填/max12，正则`/^1[34578]\d{9}$/`；email可选/max100；message必填/max210|空姓名：“请填写联系人姓名”；测试手机abc：“手机号码有误，请重填”|

来源：[静态字段与5次校验截图索引](03-ui-components.md)。**UNKNOWN**：服务端长度、trim、真实卡号格式、现代手机号范围。反馈提示200字符而maxlength210，不能擅自选一个作为原站契约。

### 3.3 鉴真、预约、查询和反馈调用流程

**SOURCE_OBSERVED（捕获JS）/ INFERRED（流程解释）**：鉴真/预约/查询 `#smt` 匿名click回调先做非空校验；`#yzm`为空则触发 `.pintu.click()`，验证码成功回调延迟约1秒再次点击；通过客户端门槛后序列化表单并Ajax预检。[调用图与函数定位](13-api-call-graph.md)

|流程|静态分支|实测边界|
|---|---|---|
|鉴真|POST check；results==1渲染tishi/product并清卡密；==2弹窗；其他分支提示及清空|入口及空值提示已测；真实鉴真成功/服务端失败未测|
|预约|POST check；results==1触发原生表单POST order.html；否则提示并恢复界面|订单填写页、消费者选品、地址/日期及提交未到达|
|查询|POST order_query_yzm；code==1表单POST order_info；==2清验证码；其他分支清标识/手机/验证码|结果页、物流轨迹与用户修改入口未验证|
|反馈|字段及拼图门槛后POST feedback_send；code==1 reset，弹窗确认后回首页；失败展示msg|未提交反馈；是否创建工单、通知和订单关联未知|

**SOURCE_OBSERVED / STATIC_CANDIDATE**：`pintu.refresh → Image.src → makecode`；拖动结束 `pintu._send_result → _ajax.request → validate_code`，成功回调再进入业务按钮逻辑。未识别/完成拼图、未绕过验证。`yzm`只有id没有name，不能把它虚构为serialize参数；`__token__`用途及服务端会话绑定未知。[卡券流程](15-voucher-workflow.md)、[查询流程](16-order-workflow.md)

## 4. 真实后台访问范围与原型差异

**LIVE_OBSERVED**：当前会话能读取商品、卡券、订单、库存/审核、物流、反馈、系统设置及角色相关核心页面。管理员列表、订单日志、系统日志返回HTTP200但内容受限。角色编辑表单可读不代表可保存，更不能证明当前账号角色或完整权限。[权限记录](live-admin/01-access-and-permissions.md)

**PROTOTYPE_OBSERVED**：原型52模板使用PAGES注入、模拟登录和Toast。**LIVE_OBSERVED**：52个模板标题均有真实菜单同名入口，但其中25项未展开。完整逐项映射列于附录B。

|差异|证据与结论|
|---|---|
|卡券类型|原型普通卡券/电子卡券；真实card_type为多选一/多选多。权益模型以真实表单为准|
|商品|真实有旺店通规格编码/名称；不能由此推断独立SKU系统|
|物流|真实有旺店通物流代码及供应商字段；不能将原型字段视为全量|
|到店提货|真实观察到bianma/pass；原型额外验证码/历史表结构未在本轮DOM发现|
|权限|真实角色编辑有rule[]细分操作；原型模拟成功不能替代端点授权证据|

上述差异来源：[原型对照](live-admin/03-prototype-vs-live.md)、[商品](live-admin/04-product-workflow.md)、[订单](live-admin/06-order-workflow.md)、[物流](live-admin/07-inventory-and-shipping.md)、[角色DOM](live-admin/evidence/role-edit-dom.json)。

## 5. 后台业务字段与规则证据

### 5.1 商品

**LIVE_OBSERVED**：列表列包含图片、品名、编码、卡券数、库存、有效期、排序、状态。筛选name/zt，zt=1启用、0停用；正常GET筛选已观察。新增/编辑表单字段包括：

- pro_name/pro_img/pro_content：名称、图片、富文本。
- spbm：商品编码；spec_no/spec_name：旺店通规格字段。
- s_time/e_time、no_area：兑换时间和不发货区域。
- p_sort/status：排序、启用；parameter0[]/parameter1[]：属性名/值；轮播图片组件。

来源：[商品DOM/脚本记录](live-admin/04-product-workflow.md)。**SOURCE_OBSERVED**：新增submit(product)判名称非空后serialize；图片必填检查被注释，不能写成必填实测。状态开关、排序处理可能立即POST，不是纯界面动作。**UNKNOWN**：编码唯一性、独立SKU、组合商品实体、修改对已发卡的影响及前台展示同步。

### 5.2 卡券、批次及一卡多商品

**LIVE_OBSERVED**：批量生成页 `card_type=1`多选一、`2`多选多；切换2后product_num显示，maxLength2。生成表单如下，均为字段/选项证据，非服务端生成规则实测。[完整批量DOM](live-admin/evidence/voucher-batch-deep-dom.json)、[切换后](live-admin/evidence/voucher-multi-dom.json)

|字段|已观察含义|
|---|---|
|pici/bname|批次隐藏字段、后台区分用卡券名称|
|product/productlist|商品ID集合/名称展示；数据流见下文|
|code_num/code_pre/sn_start|生成数量、前缀max4、起始号；页面建议单次10万以内不等于硬限制|
|code_type|1顺序、2随机；随机强度未验证|
|pass_len/pass_type|长度6/8/9/10/11/12；类型2数字、1字母、0混合|
|b_stime/b_etime/status|有效期；是否激活1/0|
|exprot_file/captcha|同时导出复选框、验证码；没有生成或导出|

**SOURCE_OBSERVED**：`.getproduct.click`打开商品选择器；`.rselect.click`把选中的qx_id收集到arr、名称收集到pro，回填父页product/productlist；没有执行“确定选择”或保存。批次submit(batch)序列化后候选POST batch_code，消费code/count/msg/pici。最大数量比较存在未替换模板字面量，不能确定实际数值上限。[批次完整脚本](live-admin/evidence/voucher-batch-deep-full-inline.json)、[选择器脚本](live-admin/evidence/product-selector-full-inline.json)

**LIVE_OBSERVED**：卡券列表筛选status为1已激活/0未激活，sale为1已提货/2未提货；有效期、批次、提货时间另有字段。实际点击下一页捕获page=2 GET200。激活页有按卡号/批次/名称/日期分组。**LIVE_OBSERVED（文案）**：“只有加密过的卡券才能正常提货”“加密不可逆”“须激活”；没有验证其算法或服务端执行。[卡券报告](live-admin/05-voucher-workflow.md)

**UNKNOWN**：数量是件数还是不同商品数、可否重复/分次兑换、多地址/拆单、过期优先级、退货返权益、加密与核销机制。激活、提货、有效期、加密应保持独立证据维度，不能擅自合成一个原站状态机。

### 5.3 订单、售后与到店提货

**LIVE_OBSERVED**：全部/待发/已发/完成/售后/回收站及到店提货页面可读。筛选status=0待发、1已发、2完成；售后、软删除与这三个值的关系未知。列表字段包括订单号、卡号、商品、收货人、手机、下单/预约/发货/完成时间、物流、面单与旺店通信息；只保留字段，不保留个人值。[订单报告](live-admin/06-order-workflow.md)

**LIVE_OBSERVED**：详情字段含订单编号、商品、收货信息、客户备注/投诉及后台备注。编辑表单包括bianma、order_sn/order_sns、id、product[]、xm、phone、province/city/district、address、order_date。只读取既有记录，不保存。到店页有bianma/pass。**SOURCE_OBSERVED**：`.getinfo/.editfwm`从行order_sn/oid构造详情或编辑iframe；编辑候选POST editcheck。售后、取消、回收、完成转换及各状态修改限制均未验证。[详情字段](live-admin/evidence/order-detail-fields-dom.json)、[编辑字段](live-admin/evidence/order-edit-dom.json)

### 5.4 库存、入出库及发货

**LIVE_OBSERVED**：库存列有累计、已用、可用、入库审核、出库审核、警戒线和记录。入/出库表单有order_sn、in_pz/out_pz、in_type/out_type、order_info，类型1实物/2虚拟，商品选择区含数量及库存。**UNKNOWN**：审核何时增加或扣减库存、预占/释放、负库存、并发一致性；未提交或审核。[库存报告](live-admin/07-inventory-and-shipping.md)

**LIVE_OBSERVED**：人工发货表单输入t_no快递单号、t_trace物流公司，并携带订单/商品/收货引用字段；物流配置有t_name/t_cname、t_code、t_code_kd100、t_code_wdt、partnerid/partnerkey、t_url/t_sort/t_default。**SOURCE_OBSERVED**：人工发货候选POST fahuo_check，另有自动面单/批量发货调用线索。未调用供应商或真实发货，字段重复不证明数据库冗余列。[人工发货DOM](live-admin/evidence/shipping-manual-form-dom.json)

### 5.5 设置、反馈和权限

**LIVE_OBSERVED**：系统设置有fahuo_after/fahuo_time、no_s_time/no_e_time、max_yy等预约/发货字段，以及短信、邮件、微信通知开关；未记录设置值，故不能认定通知已关闭。在线反馈列表可读。角色编辑包含title/hide/gid/rule[]等及细分查看/添加/修改/导出权限；管理员列表受限。[设置DOM](live-admin/evidence/settings-dom.json)、[反馈DOM](live-admin/evidence/feedback-dom.json)、[角色DOM](live-admin/evidence/role-edit-dom.json)

**UNKNOWN**：地区限制精确算法、日期/限额优先级、反馈是否工单、通知触发时机、账号绑定角色、权限保存是否成功及端点授权完整性。

## 6. 网络证据：实际请求与静态候选

### 6.1 实际捕获的关键请求

|端点/功能|实测信息|限制与证据|
|---|---|---|
|GET /home/index/notice.html；公告|200，text/html；Initiator含首页Pop/layer/jQuery|非业务JSON。[N01](evidence/network/N01-notice-headers-whitelist.txt)、[调用栈](evidence/network/N01-notice-initiator.txt)|
|GET /admin/login/yzm.html；登录验证码图|REA导航捕获200|仅图片加载；未捕获登录POST。[登录调查](17-admin-research.md)|
|后台列表/详情/表单 GET|HTML200；商品zt筛选、卡券page=2已测；详情参数来自真实入口|可访问与权限拒绝由内容区分；33路径见附录A。[网络索引](live-admin/evidence/network-index.json)|
|GET /admin/code/yzm.html；批次验证码|200，image/png|未识别或提交验证码。[批次网络](live-admin/evidence/voucher-batch-deep-network.json)|
|GET /admin/qrcode/qrcode.html|200，image/png；query含color/size/wishes|正文未保存，不代表二维码业务已执行。[欢迎页网络](live-admin/evidence/welcome-deep-network.json)|
|GET /admin/index/getcount.html；统计|3次完整200 JSON结构；无query；Request Content-Type未捕获；get_today→jQuery|当前已登录会话；未测未登录/异常权限。[welcome-deep-N17](live-admin/evidence/welcome-deep-network.json)|
|POST /admin/product/add_check.html；新增测试商品|200，业务code=0；请求URL编码，响应JSON；实际点击→layui→add.html:565→jQuery|仅失败响应；未观察创建成功。[product-submit-observed-N1](write-tests/2026-10-11/evidence/product-submit-observed-network.json)|

**LIVE_OBSERVED**：getcount响应15字段均观察为number：yesterday_order_count、today_order_count、today_df_count、tomorrow_df_count、up_count、down_count、fwm_count、pro_count、order_count_all、order_count、trace_count、admin_count、sms_count、sale_count、order_wc。未保留实际统计数值，字段口径仍UNKNOWN。

**LIVE_OBSERVED**：新增商品请求Content-Type为 `application/x-www-form-urlencoded; charset=UTF-8`，字段为pro_name/pro_img/spbm/spec_no/spec_name/s_time/e_time/no_area/pro_content/p_sort/status，传输类型均string。响应Content-Type `application/json; charset=utf-8`，结构为 `{code:number,msg:string,data:string,url:string,wait:number}`，code=0；msg原文未留存，不能用页面权限横幅替代JSON消息。来源同上。

### 6.2 用户端及登录静态候选

以下均未捕获实际业务调用成功；源码读取和真实请求分开。响应消费者不是完整JSON契约。[调用图](13-api-call-graph.md)

|候选方法/路径|调用者/参数|消费与后续|
|---|---|---|
|POST /home/index/check.html|鉴真/预约各自#smt匿名click；serialize bianma/pass/__token__|results/tishi/product；预约成功后表单提交|
|POST /home/index/order_query_yzm.html|查询#smt匿名click；bianma_jd/phone/__token__|code/msg；成功后结果表单提交|
|POST /home/index/feedback_send.html|反馈#smt匿名click；order_sn/xm/phone/email/message/__token__|code/msg；reset与回首页|
|POST /home/index/order.html|预约form action，预检成功后的原生表单submit|未获取成功订单填写页|
|POST /home/index/order_info.html|查询form action，预检成功后的原生表单submit|未获取成功查询结果页|
|GET /home/index/makecode|pintu.refresh；t、可选nowebp|挑战图片候选|
|GET /home/index/validate_code|pintu._send_result→_ajax.request；xcode|验证成功回调候选|
|POST /admin/login/login_check.html|btn-login匿名click；name/pass/yzm|code===1跳后台，否则提示；源码含网络错误恢复|
|GET /admin/login/edit.html|LoadUrl找回密码iframe目标|未打开、未提交|

**INFERRED**：后台已登录外壳现在有实测证据，但用户手动登录不等于Agent捕获过login_check。上述前台POST、验证码候选的真实头、响应类型、认证和错误枚举仍UNKNOWN。旧前台回调缺显式fail/timeout恢复仅是代码风险线索，未故障注入验证。[错误处理](13-api-call-graph.md)

### 6.3 后台源代码端点全集的当前分类

**SOURCE_OBSERVED**：历史文件抽取57个Method/URL字面量，包含导航、读写和动态服务候选，不是57个未调用的业务API。其中getcount当时已有实测，add_check现有失败实测；在这57项中其余55项仍只有本清单范围内的静态证据。动态拼接及未匹配调用不保证完整，不能声称全站API仅57个。逐项含来源和状态见附录C。

## 7. 最近一次隔离写入测试与阻塞

**LIVE_OBSERVED（会话授权与操作记录）**：用户已授权写入测试，并选择新建测试前缀隔离数据，结束停用商品与未用卡券、保留审计。计划前缀JSTEST261011，最多3商品、2批次（各最多2卡）、2订单；这是计划上限，不是实际数量。[计划](write-tests/2026-10-11/01-test-plan.md)

**LIVE_OBSERVED**：一次实际提交名称 `JSTEST261011-A-隔离测试勿发货`、编码 `JSTEST261011-A`、状态停用的商品。前两次定位动作只填写表单，没有到达提交；第三次定位到实际input按钮后捕获一次POST。HTTP200/code0；页面权限横幅存在；随后按测试名称查询0匹配、空列表提示。[结果](write-tests/2026-10-11/02-results.md)、[列表核对](write-tests/2026-10-11/evidence/product-absence.json)

**INFERRED**：组合证据支持商品新增受权限限制，但不能断言所有写接口均被拒绝。**LIVE_OBSERVED**：未继续生成批次、卡券、订单，未发货/通知/导出/修改权限；无已确认新记录需停用。**UNKNOWN**：服务器内部日志或部分副作用不可由列表为空完全排除。

![测试商品提交后的权限提示（已脱敏）](write-tests/2026-10-11/evidence/product-submit-observed-masked.png)

**当前阻塞**：需要用户自行切换到具有商品新增及测试数据停用权限的合法测试账号；已有同类写测授权继续有效。订单提交前仍需确定无真实通知/物流影响及专用收件测试资料。不是再次等待通用写入许可，也不要求用户交出密码/Token。

## 8. 前后台端到端证据链

以下六条均没有完整成功闭环。后台真实订单存在不能证明本次前台兑换成功。[映射原报告](live-admin/11-frontend-backend-workflows.md)

|链路|已有证据|尚缺|
|---|---|---|
|新增商品→前台展示|两端目录与详情；新增一次失败响应|成功新增、启用/停用及前台同步|
|生成卡券→鉴真|后台生成表单；前台check静态流程|专用新卡、生成及鉴真实测|
|配置卡券商品→前台选品|多选一/多选多显隐；ID集合回填源代码|实际消费者选品规则及持久化|
|兑换→后台订单|前台order.html POST候选；后台订单字段|成功提交、拆单/核销/库存/幂等|
|订单处理→前台查询状态|后台0/1/2筛选；前台预检及POST候选|处理后的真实查询结果与状态映射|
|后台发货→前台物流|人工发货表单；物流配置|发货保存、轨迹响应、刷新及通知|

## 9. 前端结构与技术边界

**INFERRED**：用户端符合传统多页面、完整HTML导航、局部jQuery Ajax和原生POST跳页；不能仅凭.html后缀定性。共享资源含Bootstrap、jQuery、layer、validate.js、站点v1.css；部分详情Swiper。HTML5脚本为旧IE条件声明、jqthumb是注释残留，不能视作现代Chrome已加载。[架构](06-frontend-architecture.md)、[JS完整URL声明表](11-first-stage-audit.md)

**SOURCE_OBSERVED / LIVE_OBSERVED**：后台外壳+内容iframe；加载jQuery、layui/layuiAdmin、KindEditor等。表单状态依靠DOM、layui事件、jQuery serialize、layer提示；不是已确认的Vue/React SPA。静态页面代码以页面脚本为主，未获得服务器源码或完整构建工程。[后台调用关系](live-admin/09-js-callgraph.md)

**INFERRED**：CSS由第三方基础样式、共用v1.css和内联覆盖组成；用户端主色#cc0000，移动底栏固定，桌面/移动布局有现成证据。**UNKNOWN**：全量计算样式、真机兼容、完整资源缓存策略、未访问区域技术栈、服务端语言/框架/数据库/事务结构。

## 10. 新系统模型与开发就绪度

以下均为 **DESIGN_DECISION**；只是以观察到的业务概念为依据，不是原站数据库逆向结果。技术栈按用户要求：Go + net/http、MySQL、Vue 3 + TypeScript + Vite、Docker Compose、Go testing、Playwright。[字段级设计及来源](live-admin/10-domain-model.md)

|领域|证据支持的概念|自主设计/待决点|
|---|---|---|
|Product|名称、编码、图片、属性、有效期、区域、排序启用|编码唯一约束、子表、内容保护；SKU未确认|
|VoucherType / VoucherBatch / Voucher|多选一/多选多、数量、批次、生成、激活、有效期|独立表、规则版本、唯一卡号、凭证hash；原站加密实现未知|
|VoucherProduct|源代码传递商品ID集合|关系表、权益快照及limit_config；件数/分次规则须裁决|
|Order / OrderItem|订单及收货字段、product[]、备注、状态筛选|订单项快照、幂等键、版本冲突；拆单/数量策略可替换|
|Inventory / InventoryTransaction|累计/已用/可用、出入库、审核|不可变账本、预占字段、审核事务、唯一幂等引用|
|Shipment|物流公司、单号及订单引用|供应商适配器、多包裹扩展、回调幂等、Outbox|
|AdminUser / Role / Permission|登录字段、角色和细分操作控件|密码保护、角色关联、服务端默认拒绝、审计|

**DESIGN_DECISION（拟议，非原站事实）**：订单创建预占、发货扣实物、取消释放；Voucher锁与库存固定顺序加锁；权益、Order/OrderItem、库存账和审计在同一事务提交；外部通知在提交后Outbox处理。单次兑换/不同商品各1件只是可选择的MVP策略，仍需明确采纳，不能擅自固化。

|优先级|可开展内容|必要条件|
|---|---|---|
|P0|自有工程基础、迁移、鉴权、审计、规则接口与错误契约|明确自主设计；不依赖目标站业务API|
|P1|用户端展示/导航/入口表单；商品管理与前台目录|现有视觉/字段可支撑；成功路径用自有数据验收|
|P1|卡券配置及最小兑换订单闭环|权益、库存、幂等策略明确且可替换；自有沙箱验证并发和回滚|
|P2|库存审核、人工发货、查询结果|自有状态机明确；物流/通知先用替身|
|P3|真实供应商、面单、旺店通、售后、完整角色权限|正式接口契约、权限及供应商沙箱；补证高风险规则|

**INFERRED**：可开发独立产品，不等于已经具备完全兼容原站交易行为的证据。本次仅研究与文档，没有业务代码。[增量计划](live-admin/13-development-plan.md)

## 11. 关键未知项及下一步验证

|优先级|UNKNOWN / WRITE_TEST_REQUIRED|最小验证条件|
|---|---|---|
|P0|实际商品新增/停用权限；卡券生成能力|合法可写测试账号；沿用隔离前缀及停用审计方案|
|P0|多选多数量、重复/分次、多地址/拆单|专用批次及可控卡券，先只读选品再受控兑换|
|P0|有效期继承、激活/加密/提货关联|各状态专用样本；不枚举卡密、不绕过验证|
|P0|核销、重复请求、并发、多商品库存一致性|无通知沙箱、可复核账本与可恢复专用库存|
|P0|消费者查询身份/响应/物流、后台状态映射|自有测试订单及专用联系方式，正常验证流程|
|P1|修改、取消、恢复、完成、售后权益返还|各状态测试订单；明确恢复方式和操作范围|
|P1|入出库审核时点、负库存及实物/虚拟语义|可恢复测试商品与库存单；操作前后快照|
|P1|发货、面单、通知、供应商异常|物流/通知沙箱，不产生真实运单或消息|
|P1|商品变更对历史卡/订单影响|专用商品及权益版本，保留快照|
|P1|角色保存与端点权限一致性|另有明确权限测试授权、专用角色与恢复方案|

来源：[前台未知](19-unresolved-business-rules.md)、[后台W01–W08](live-admin/12-business-rules-and-unknowns.md)、[最新写测阻塞](write-tests/2026-10-11/02-results.md)。REA后续适合分析尚未捕获的消费者选品、订单填写、查询结果及复杂物流脚本；继续分析现有登录页不能代替成功业务数据。

## 12. 证据完整性、历史修正与局限

- **INFERRED（历史审核）**：一期manifest 131项存在且hash匹配，385处本地链接有目标；后台阶段对216个历史文件复核未改动，并检查393个本地文档链接。[一期审核](evidence/phase2/audit-check.json)、[后台交付检查](live-admin/evidence/delivery-check.json)
- **INFERRED**：一期63张截图是52张页面基础截图+11张交互/网络图；移动模拟截图实际保存尺寸与逻辑视口不同，不能称真机验证。后台49张遮盖数据表/输入/账号的截图用于布局佐证，不能据灰块推断内容。[一期索引](08-evidence-index.md)、[后台索引](live-admin/14-evidence-index.md)
- **LIVE_OBSERVED（采集记录）**：早期voucher-page2控件定位失败，无请求；只有voucher-next-observed算分页成功。无href元素被采集器赋予当前URL的fallback不是导航证据。
- **INFERRED（脱敏审查）**：后台早期采集曾遗漏账号欢迎文案、卡号链接及入口key，交付前移除并修正工具；当前保留脱敏版本，不能声称早期采集从未出现这些值。没有将含Cookie/Authorization的完整HAR纳入证据。
- **UNKNOWN**：采集窗口不等于全程流量；未捕获状态不等于失败；响应shape只记录类型且数组采样，不能证明全量字段、联合类型或服务端模型。
- **INFERRED（本次统一修正）**：旧17/20“未登录、管理功能0”和旧后台“0 POST”只适用于各自阶段；本报告以最新证据说明已读后台和一次失败POST。旧57项源码抽取包括已实测getcount，不能整体称未调用；商品add_check现也已升级为失败实测。

## 13. 原始材料阅读索引

|材料|范围与入口|
|---|---|
|一期01–10|页面清单、逐页、组件、导航、API、架构、流程、证据、未知、需求；[入口](01-page-inventory.md)、[证据](08-evidence-index.md)|
|二期11–20|一期审核、REA、调用图、网络、卡券/订单、公开登录、模型及就绪；[入口](11-first-stage-audit.md)、[证据清单](evidence/phase2/evidence-index.json)|
|真实后台01–14|权限、页面、原型、商品/卡券/订单/库存、API、JS、模型、映射、未知、计划；[入口](live-admin/01-access-and-permissions.md)、[证据索引](live-admin/14-evidence-index.md)|
|52页单文件原型|仅PROTOTYPE_OBSERVED；[模板索引](live-admin/evidence/prototype-pages.json)，不把演示数据复制进本报告|
|写测计划与结果|授权、隔离方案、一次失败POST、清理台账；[计划](write-tests/2026-10-11/01-test-plan.md)、[结果](write-tests/2026-10-11/02-results.md)|
|完整逐请求/源码位置|[后台network-index](live-admin/evidence/network-index.json)、[57项源码抽取](live-admin/evidence/static-candidates.json)、[写测请求](write-tests/2026-10-11/evidence/product-submit-observed-network.json)|

## 附录A. 已访问后台33个内容路径

**LIVE_OBSERVED**：下表继承只读阶段的页面访问证据；“未执行保存”描述当时操作。唯一后续变化是product/add已尝试提交但业务code0，详见第7节；其余页面不因此获得写入验证。

|路径|结果|观察及证据|
|---|---|---|
|`/admin/auth/admin.html`|LIVE_OBSERVED：权限受限|[DOM](live-admin/evidence/admins-dom.json) / [请求](live-admin/evidence/admins-network.json) / [脱敏截图](live-admin/evidence/admins-masked.png)|
|`/admin/auth/group.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/roles-edit-entry-dom.json) / [请求](live-admin/evidence/roles-edit-entry-network.json) / [脱敏截图](live-admin/evidence/roles-edit-entry-masked.png)|
|`/admin/auth/group_edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/role-edit-dom.json) / [请求](live-admin/evidence/role-edit-network.json) / [脱敏截图](live-admin/evidence/role-edit-masked.png)|
|`/admin/code/batch_add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/voucher-multi-dom.json) / [请求](live-admin/evidence/voucher-multi-network.json) / [脱敏截图](live-admin/evidence/voucher-multi-masked.png)|
|`/admin/code/batchedit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/voucher-edit-batch-dom.json) / [请求](live-admin/evidence/voucher-edit-batch-network.json) / [脱敏截图](live-admin/evidence/voucher-edit-batch-masked.png)|
|`/admin/code/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/voucher-pagination-entry-dom.json) / [请求](live-admin/evidence/voucher-pagination-entry-network.json) / [脱敏截图](live-admin/evidence/voucher-pagination-entry-masked.png)|
|`/admin/code/status.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/voucher-activation-dom.json) / [请求](live-admin/evidence/voucher-activation-network.json) / [脱敏截图](live-admin/evidence/voucher-activation-masked.png)|
|`/admin/config/log.html`|LIVE_OBSERVED：权限受限|[DOM](live-admin/evidence/system-log-dom.json) / [请求](live-admin/evidence/system-log-network.json) / [脱敏截图](live-admin/evidence/system-log-masked.png)|
|`/admin/config/system.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/settings-dom.json) / [请求](live-admin/evidence/settings-network.json) / [脱敏截图](live-admin/evidence/settings-masked.png)|
|`/admin/inbound/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/inbound-add-dom.json) / [请求](live-admin/evidence/inbound-add-network.json) / [脱敏截图](live-admin/evidence/inbound-add-masked.png)|
|`/admin/inbound/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/inbound-list-dom.json) / [请求](live-admin/evidence/inbound-list-network.json) / [脱敏截图](live-admin/evidence/inbound-list-masked.png)|
|`/admin/index/welcome.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/welcome-deep-dom.json) / [请求](live-admin/evidence/welcome-deep-network.json) / [脱敏截图](live-admin/evidence/welcome-deep-masked.png)|
|`/admin/message/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/feedback-dom.json) / [请求](live-admin/evidence/feedback-network.json) / [脱敏截图](live-admin/evidence/feedback-masked.png)|
|`/admin/order/edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-edit-dom.json) / [请求](live-admin/evidence/order-edit-network.json) / [脱敏截图](live-admin/evidence/order-edit-masked.png)|
|`/admin/order/fahuo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/shipping-manual-form-dom.json) / [请求](live-admin/evidence/shipping-manual-form-network.json) / [脱敏截图](live-admin/evidence/shipping-manual-form-masked.png)|
|`/admin/order/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-list-dom.json) / [请求](live-admin/evidence/order-list-network.json) / [脱敏截图](live-admin/evidence/order-list-masked.png)|
|`/admin/order/index_df.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/shipping-form-entry-dom.json) / [请求](live-admin/evidence/shipping-form-entry-network.json) / [脱敏截图](live-admin/evidence/shipping-form-entry-masked.png)|
|`/admin/order/index_sh.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-aftercare-dom.json) / [请求](live-admin/evidence/order-aftercare-network.json) / [脱敏截图](live-admin/evidence/order-aftercare-masked.png)|
|`/admin/order/index_wc.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-complete-dom.json) / [请求](live-admin/evidence/order-complete-network.json) / [脱敏截图](live-admin/evidence/order-complete-masked.png)|
|`/admin/order/index_yf.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-shipped-dom.json) / [请求](live-admin/evidence/order-shipped-network.json) / [脱敏截图](live-admin/evidence/order-shipped-masked.png)|
|`/admin/order/index_zf.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-trash-dom.json) / [请求](live-admin/evidence/order-trash-network.json) / [脱敏截图](live-admin/evidence/order-trash-masked.png)|
|`/admin/order/log.html`|LIVE_OBSERVED：权限受限|[DOM](live-admin/evidence/order-log-dom.json) / [请求](live-admin/evidence/order-log-network.json) / [脱敏截图](live-admin/evidence/order-log-masked.png)|
|`/admin/order/orderinfo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-detail-fields-dom.json) / [请求](live-admin/evidence/order-detail-fields-network.json) / [脱敏截图](live-admin/evidence/order-detail-fields-masked.png)|
|`/admin/order/tihuo.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/order-pickup-dom.json) / [请求](live-admin/evidence/order-pickup-network.json) / [脱敏截图](live-admin/evidence/order-pickup-masked.png)|
|`/admin/outbound/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/outbound-add-dom.json) / [请求](live-admin/evidence/outbound-add-network.json) / [脱敏截图](live-admin/evidence/outbound-add-masked.png)|
|`/admin/outbound/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/outbound-list-dom.json) / [请求](live-admin/evidence/outbound-list-network.json) / [脱敏截图](live-admin/evidence/outbound-list-masked.png)|
|`/admin/product/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/product-add-dom.json) / [请求](live-admin/evidence/product-add-network.json)|
|`/admin/product/edit.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/product-edit-dom.json) / [请求](live-admin/evidence/product-edit-network.json) / [脱敏截图](live-admin/evidence/product-edit-masked.png)|
|`/admin/product/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/product-list-dom.json) / [请求](live-admin/evidence/product-list-network.json)|
|`/admin/product/select_list.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/product-selector-dom.json) / [请求](live-admin/evidence/product-selector-network.json) / [脱敏截图](live-admin/evidence/product-selector-masked.png)|
|`/admin/trace/add.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/shipping-add-dom.json) / [请求](live-admin/evidence/shipping-add-network.json) / [脱敏截图](live-admin/evidence/shipping-add-masked.png)|
|`/admin/trace/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/shipping-list-dom.json) / [请求](live-admin/evidence/shipping-list-network.json) / [脱敏截图](live-admin/evidence/shipping-list-masked.png)|
|`/admin/warehousing/index.html`|LIVE_OBSERVED：可读取页面/表单；未执行保存|[DOM](live-admin/evidence/inventory-dom.json) / [请求](live-admin/evidence/inventory-network.json) / [脱敏截图](live-admin/evidence/inventory-masked.png)|

## 附录B. 52项原型与真实菜单逐项对照

**PROTOTYPE_OBSERVED / LIVE_OBSERVED**：名称映射、页面只读访问和拒绝分类；可访问不代表有保存权限。

|编号|原型模板（标题）|真实路径|调查状态|证据|
|---|---|---|---|---|
|M01|`area_list.html` 行政区域列表|`/admin/area/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M02|`auth_admin.html` 管理员列表|`/admin/auth/admin.html`|真实存在但当前账号无权访问|[DOM](live-admin/evidence/admins-dom.json)|
|M03|`auth_group.html` 角色管理|`/admin/auth/group.html`|真实存在并可访问|[DOM](live-admin/evidence/roles-edit-entry-dom.json)|
|M04|`code_batch_add.html` 批量生成卡券|`/admin/code/batch_add.html`|真实存在并可访问|[DOM](live-admin/evidence/voucher-multi-dom.json)|
|M05|`code_batchdel.html` 批量删除卡券|`/admin/code/batchdel.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M06|`code_batchedit.html` 批量修改卡券|`/admin/code/batchedit.html`|真实存在并可访问|[DOM](live-admin/evidence/voucher-edit-batch-dom.json)|
|M07|`code_card_reset.html` 卡券初始化|`/admin/code/card_reset.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M08|`code_editpass.html` 修改卡券密码|`/admin/code/editpass.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M09|`code_jiami.html` 卡券加密|`/admin/code/jiami.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M10|`code_list.html` 卡券列表|`/admin/code/index.html`|真实存在并可访问|[DOM](live-admin/evidence/voucher-pagination-entry-dom.json)|
|M11|`code_log.html` 发送记录|`/admin/code/log.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M12|`code_status.html` 卡券激活|`/admin/code/status.html`|真实存在并可访问|[DOM](live-admin/evidence/voucher-activation-dom.json)|
|M13|`config_chushi.html` 系统初始化|`/admin/config/chushi.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M14|`config_filemanage.html` 上传文件管理|`/admin/config/filemanage.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M15|`config_log.html` 系统日志管理|`/admin/config/log.html`|真实存在但当前账号无权访问|[DOM](live-admin/evidence/system-log-dom.json)|
|M16|`config_sys_data.html` 数据备份恢复|`/admin/config/sys_data.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M17|`config_system.html` 系统参数配置|`/admin/config/system.html`|真实存在并可访问|[DOM](live-admin/evidence/settings-dom.json)|
|M18|`export_card.html` 导出电子卡券|`/admin/export/card.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M19|`export_code.html` 批量导出卡券|`/admin/export/code.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M20|`export_index.html` 导出文档管理|`/admin/export/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M21|`export_wxqrcode.html` 导出小程序码|`/admin/export/wxqrcode.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M22|`import_batch_one.html` 批量导入卡券|`/admin/import/batch_one.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M23|`inbound_add.html` 商品入库|`/admin/inbound/add.html`|真实存在并可访问|[DOM](live-admin/evidence/inbound-add-dom.json)|
|M24|`inbound_list.html` 入库审核|`/admin/inbound/index.html`|真实存在并可访问|[DOM](live-admin/evidence/inbound-list-dom.json)|
|M25|`message_list.html` 在线反馈|`/admin/message/index.html`|真实存在并可访问|[DOM](live-admin/evidence/feedback-dom.json)|
|M26|`order_df.html` 待发订单|`/admin/order/index_df.html`|真实存在并可访问|[DOM](live-admin/evidence/shipping-form-entry-dom.json)|
|M27|`order_export.html` 导出订单|`/admin/order/export.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M28|`order_kuaidi.html` 导入快递单|`/admin/order/kuaidi.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M29|`order_list.html` 全部订单|`/admin/order/index.html`|真实存在并可访问|[DOM](live-admin/evidence/order-list-dom.json)|
|M30|`order_log.html` 订单日志|`/admin/order/log.html`|真实存在但当前账号无权访问|[DOM](live-admin/evidence/order-log-dom.json)|
|M31|`order_sh.html` 售后订单|`/admin/order/index_sh.html`|真实存在并可访问|[DOM](live-admin/evidence/order-aftercare-dom.json)|
|M32|`order_tihuo.html` 到店提货|`/admin/order/tihuo.html`|真实存在并可访问|[DOM](live-admin/evidence/order-pickup-dom.json)|
|M33|`order_wc.html` 完成订单|`/admin/order/index_wc.html`|真实存在并可访问|[DOM](live-admin/evidence/order-complete-dom.json)|
|M34|`order_yf.html` 已发订单|`/admin/order/index_yf.html`|真实存在并可访问|[DOM](live-admin/evidence/order-shipped-dom.json)|
|M35|`order_zf.html` 回收站|`/admin/order/index_zf.html`|真实存在并可访问|[DOM](live-admin/evidence/order-trash-dom.json)|
|M36|`outbound_add.html` 商品出库|`/admin/outbound/add.html`|真实存在并可访问|[DOM](live-admin/evidence/outbound-add-dom.json)|
|M37|`outbound_list.html` 出库审核|`/admin/outbound/index.html`|真实存在并可访问|[DOM](live-admin/evidence/outbound-list-dom.json)|
|M38|`product_add.html` 添加商品|`/admin/product/add.html`|真实存在并可访问|[DOM](live-admin/evidence/product-add-dom.json)|
|M39|`product_list.html` 商品列表|`/admin/product/index.html`|真实存在并可访问|[DOM](live-admin/evidence/product-list-dom.json)|
|M40|`sms_list.html` 短信列表|`/admin/sms/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M41|`sms_send.html` 发送短信|`/admin/sms/send.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M42|`tools_decode.html` 二维码解码器|`/admin/tools/decode.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M43|`tools_makecode.html` 二维码生成器|`/admin/tools/makecode.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M44|`trace_add.html` 新增物流|`/admin/trace/add.html`|真实存在并可访问|[DOM](live-admin/evidence/shipping-add-dom.json)|
|M45|`trace_list.html` 物流管理|`/admin/trace/index.html`|真实存在并可访问|[DOM](live-admin/evidence/shipping-list-dom.json)|
|M46|`user_list.html` 用户列表|`/admin/user/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M47|`warehousing_list.html` 库存明细|`/admin/warehousing/index.html`|真实存在并可访问|[DOM](live-admin/evidence/inventory-dom.json)|
|M48|`wechat_menu.html` 自定义菜单|`/admin/wechat/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M49|`wechat_news.html` 图文素材|`/admin/response/news.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M50|`wechat_response.html` 回复资源管理|`/admin/response/index.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M51|`wechat_template.html` 消息模板|`/admin/wechat/template_msg.html`|尚未调查（菜单已观察）|[DOM](live-admin/evidence/initial-dom.json)|
|M52|`welcome.html` 后台首页|`/admin/index/welcome.html`|真实存在并可访问|[DOM](live-admin/evidence/welcome-deep-dom.json)|

## 附录C. 后台57项源码端点及最新证据状态

下表从历史源码清单整合；源码行号是捕获内联script内部行号。getcount为真实读取、add_check为真实失败提交，其余均STATIC_CANDIDATE，不代表成功调用。

|Method / URL字面量|调用页面|源码定位|当前状态|
|---|---|---|---|
|GET `/admin/login/quit.html`|`/admin/index/index.html`|[admins-source-excerpts.json](live-admin/evidence/admins-source-excerpts.json) inline 0 行12|STATIC_CANDIDATE|
|GET `/admin/admin/clearcache.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](live-admin/evidence/feedback-source-excerpts.json) inline 0 行247|STATIC_CANDIDATE|
|POST `/admin/message/messagestatus.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](live-admin/evidence/feedback-source-excerpts.json) inline 1 行35|STATIC_CANDIDATE|
|POST `/admin/message/emptymessage.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](live-admin/evidence/feedback-source-excerpts.json) inline 1 行55|STATIC_CANDIDATE|
|POST `/admin/message/delmessage.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](live-admin/evidence/feedback-source-excerpts.json) inline 1 行89|STATIC_CANDIDATE|
|POST `/admin/message/message_detail.html`|`/admin/message/index.html`|[feedback-source-excerpts.json](live-admin/evidence/feedback-source-excerpts.json) inline 1 行114|STATIC_CANDIDATE|
|POST `/admin/inbound/add_check.html`|`/admin/inbound/add.html`|[inbound-add-source-excerpts.json](live-admin/evidence/inbound-add-source-excerpts.json) inline 1 行101|STATIC_CANDIDATE|
|POST `/admin/inbound/del_choose.html`|`/admin/inbound/index.html`|[inbound-list-source-excerpts.json](live-admin/evidence/inbound-list-source-excerpts.json) inline 1 行66|STATIC_CANDIDATE|
|POST `/admin/warehousing/jjx_choose.html?e=1`|`/admin/warehousing/index.html`|[inventory-source-excerpts.json](live-admin/evidence/inventory-source-excerpts.json) inline 1 行62|STATIC_CANDIDATE|
|POST `/admin/order/order_cancel_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](live-admin/evidence/order-aftercare-source-excerpts.json) inline 1 行187|STATIC_CANDIDATE|
|POST `/admin/order/exportorder_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](live-admin/evidence/order-aftercare-source-excerpts.json) inline 1 行226|STATIC_CANDIDATE|
|POST `/admin/order/exportorder.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](live-admin/evidence/order-aftercare-source-excerpts.json) inline 1 行236|STATIC_CANDIDATE|
|POST `/admin/order/sh_choose.html`|`/admin/order/index_sh.html`|[order-aftercare-source-excerpts.json](live-admin/evidence/order-aftercare-source-excerpts.json) inline 1 行266|STATIC_CANDIDATE|
|POST `/admin/order/orderstatus.html`|`/admin/order/index_wc.html`|[order-complete-source-excerpts.json](live-admin/evidence/order-complete-source-excerpts.json) inline 1 行154|STATIC_CANDIDATE|
|POST `/admin/order/edit_choose.html?e=1`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](live-admin/evidence/order-detail-entry-final-source-excerpts.json) inline 1 行262|STATIC_CANDIDATE|
|POST `/admin/order/dzmd_batch_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](live-admin/evidence/order-detail-entry-final-source-excerpts.json) inline 1 行313|STATIC_CANDIDATE|
|POST `/admin/order/fahuo_batch_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](live-admin/evidence/order-detail-entry-final-source-excerpts.json) inline 1 行345|STATIC_CANDIDATE|
|POST `/admin/order/del_dzmd_choose.html`|`/admin/order/index_df.html`|[order-detail-entry-final-source-excerpts.json](live-admin/evidence/order-detail-entry-final-source-excerpts.json) inline 1 行409|STATIC_CANDIDATE|
|POST `/admin/order/edit_orderinfo.html`|`/admin/order/orderinfo.html`|[order-detail-fields-source-excerpts.json](live-admin/evidence/order-detail-fields-source-excerpts.json) inline 1 行4|STATIC_CANDIDATE|
|POST `/admin/order/edit_orderinfo.html?type=1`|`/admin/order/orderinfo.html`|[order-detail-fields-source-excerpts.json](live-admin/evidence/order-detail-fields-source-excerpts.json) inline 1 行25|STATIC_CANDIDATE|
|POST `/admin/order/editcheck.html`|`/admin/order/edit.html`|[order-edit-source-excerpts.json](live-admin/evidence/order-edit-source-excerpts.json) inline 1 行46|STATIC_CANDIDATE|
|POST `/admin/order/orderupload.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](live-admin/evidence/order-list-source-excerpts.json) inline 1 行194|STATIC_CANDIDATE|
|POST `/admin/order/orderdownload.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](live-admin/evidence/order-list-source-excerpts.json) inline 1 行232|STATIC_CANDIDATE|
|POST `/admin/order/wdt_del_choose.html`|`/admin/order/index.html`|[order-list-source-excerpts.json](live-admin/evidence/order-list-source-excerpts.json) inline 1 行272|STATIC_CANDIDATE|
|POST `/admin/order/bianma_check.html`|`/admin/order/tihuo.html`|[order-pickup-source-excerpts.json](live-admin/evidence/order-pickup-source-excerpts.json) inline 1 行17|STATIC_CANDIDATE|
|POST `/admin/order/fahuo_cancel.html`|`/admin/order/index_yf.html`|[order-shipped-source-excerpts.json](live-admin/evidence/order-shipped-source-excerpts.json) inline 1 行329|STATIC_CANDIDATE|
|POST `/admin/order/order_cancel.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](live-admin/evidence/order-trash-source-excerpts.json) inline 1 行138|STATIC_CANDIDATE|
|POST `/admin/order/del.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](live-admin/evidence/order-trash-source-excerpts.json) inline 1 行167|STATIC_CANDIDATE|
|POST `/admin/order/order_clear.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](live-admin/evidence/order-trash-source-excerpts.json) inline 1 行187|STATIC_CANDIDATE|
|POST `/admin/order/order_recovery.html`|`/admin/order/index_zf.html`|[order-trash-source-excerpts.json](live-admin/evidence/order-trash-source-excerpts.json) inline 1 行206|STATIC_CANDIDATE|
|POST `/admin/outbound/add_check.html`|`/admin/outbound/add.html`|[outbound-add-source-excerpts.json](live-admin/evidence/outbound-add-source-excerpts.json) inline 1 行101|STATIC_CANDIDATE|
|POST `/admin/outbound/del_choose.html`|`/admin/outbound/index.html`|[outbound-list-source-excerpts.json](live-admin/evidence/outbound-list-source-excerpts.json) inline 1 行65|STATIC_CANDIDATE|
|POST `/admin/product/add_check.html`|`/admin/product/add.html`|[product-add-deep-source-excerpts.json](live-admin/evidence/product-add-deep-source-excerpts.json) inline 1 行122|LIVE_OBSERVED：POST code=0|
|POST `/admin/product/onestatus.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](live-admin/evidence/product-edit-entry-source-excerpts.json) inline 1 行36|STATIC_CANDIDATE|
|POST `/admin/product/update_sort.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](live-admin/evidence/product-edit-entry-source-excerpts.json) inline 1 行130|STATIC_CANDIDATE|
|POST `/admin/product/del.html`|`/admin/product/index.html`|[product-edit-entry-source-excerpts.json](live-admin/evidence/product-edit-entry-source-excerpts.json) inline 1 行154|STATIC_CANDIDATE|
|POST `/admin/product/update.html`|`/admin/product/edit.html`|[product-edit-source-excerpts.json](live-admin/evidence/product-edit-source-excerpts.json) inline 1 行65|STATIC_CANDIDATE|
|POST `/admin/auth/group_edit2.html`|`/admin/auth/group_edit.html`|[role-edit-source-excerpts.json](live-admin/evidence/role-edit-source-excerpts.json) inline 1 行49|STATIC_CANDIDATE|
|POST `/admin/auth/group_status.html`|`/admin/auth/group.html`|[roles-deep-source-excerpts.json](live-admin/evidence/roles-deep-source-excerpts.json) inline 1 行30|STATIC_CANDIDATE|
|POST `/admin/auth/group_del.html`|`/admin/auth/group.html`|[roles-deep-source-excerpts.json](live-admin/evidence/roles-deep-source-excerpts.json) inline 1 行60|STATIC_CANDIDATE|
|POST `/admin/config/system_edit.html`|`/admin/config/system.html`|[settings-source-excerpts.json](live-admin/evidence/settings-source-excerpts.json) inline 3 行329|STATIC_CANDIDATE|
|GET `/admin/config/bak.html?bak=1`|`/admin/config/system.html`|[settings-source-excerpts.json](live-admin/evidence/settings-source-excerpts.json) inline 3 行348|STATIC_CANDIDATE|
|GET `/admin/config/recover_bak.html?bak=1`|`/admin/config/system.html`|[settings-source-excerpts.json](live-admin/evidence/settings-source-excerpts.json) inline 3 行365|STATIC_CANDIDATE|
|GET `/admin/config/getrandpass.html?n=`|`/admin/config/system.html`|[settings-source-excerpts.json](live-admin/evidence/settings-source-excerpts.json) inline 3 行511|STATIC_CANDIDATE|
|GET `/admin/wechat/get_template_msg_haotihuo.html?e=`|`/admin/config/system.html`|[settings-source-excerpts.json](live-admin/evidence/settings-source-excerpts.json) inline 3 行522|STATIC_CANDIDATE|
|POST `/admin/trace/addcheck.html`|`/admin/trace/add.html`|[shipping-add-source-excerpts.json](live-admin/evidence/shipping-add-source-excerpts.json) inline 1 行14|STATIC_CANDIDATE|
|POST `/admin/trace/onestatus.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](live-admin/evidence/shipping-list-source-excerpts.json) inline 1 行38|STATIC_CANDIDATE|
|POST `/admin/trace/del.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](live-admin/evidence/shipping-list-source-excerpts.json) inline 1 行70|STATIC_CANDIDATE|
|GET `/admin/trace/setdefault.html`|`/admin/trace/index.html`|[shipping-list-source-excerpts.json](live-admin/evidence/shipping-list-source-excerpts.json) inline 1 行86|STATIC_CANDIDATE|
|POST `/admin/order/fahuo_check.html`|`/admin/order/fahuo.html`|[shipping-manual-form-source-excerpts.json](live-admin/evidence/shipping-manual-form-source-excerpts.json) inline 1 行22|STATIC_CANDIDATE|
|POST `/admin/code/staupdate.html`|`/admin/code/status.html`|[voucher-activation-source-excerpts.json](live-admin/evidence/voucher-activation-source-excerpts.json) inline 1 行27|STATIC_CANDIDATE|
|POST `/admin/code/batch_code.html`|`/admin/code/batch_add.html`|[voucher-batch-deep-source-excerpts.json](live-admin/evidence/voucher-batch-deep-source-excerpts.json) inline 1 行60|STATIC_CANDIDATE|
|POST `/admin/code/onestatus.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](live-admin/evidence/voucher-list-source-excerpts.json) inline 1 行79|STATIC_CANDIDATE|
|POST `/admin/code/sndel.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](live-admin/evidence/voucher-list-source-excerpts.json) inline 1 行258|STATIC_CANDIDATE|
|POST `/admin/code/pass_choose.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](live-admin/evidence/voucher-list-source-excerpts.json) inline 1 行310|STATIC_CANDIDATE|
|POST `/admin/code/exportcode.html`|`/admin/code/index.html`|[voucher-list-source-excerpts.json](live-admin/evidence/voucher-list-source-excerpts.json) inline 1 行422|STATIC_CANDIDATE|
|GET `/admin/index/getcount.html`|`/admin/index/welcome.html`|[welcome-deep-source-excerpts.json](live-admin/evidence/welcome-deep-source-excerpts.json) inline 1 行5|LIVE_OBSERVED：GET JSON200|
