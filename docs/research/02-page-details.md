# 02 独立页面调查记录

每个P编号是独立访问记录。**OBSERVED**表示实际浏览器可见；**INFERRED**表示脱敏HTML/CSS分析；**UNKNOWN**表示未验证。桌面约1512×738 CSS像素，移动采用Chrome Responsive 390×844并缩放显示；无真实手机触摸/网络测试。截图裁除浏览器标签、地址栏和书签。长页仅截取当前视口，AX补充可访问文本，不能代替全页视觉验证。

浏览器所读顶层文档标题统一为“好提货卡券提货系统_卡券兑换系统_微信提货系统_大闸蟹提货系统”（含本次独立打开的公告页）；下列页面名指主体标题或用途。公共导航、服务承诺、页脚组件详见[03](03-ui-components.md)。

## P01 首页

**OBSERVED** 实际URL：https://demo.haotihuo.com/。入口：首页导航。

**OBSERVED** 页面功能及内容：首页轮播、六个快捷入口、四个推荐礼品、公告、服务承诺及须知。

证据：[浏览器AX](evidence/ax/01-home.txt) · [桌面截图](evidence/screenshots/01-home-desktop.png) · [移动截图](evidence/screenshots/01-home-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：首页有轮播、快捷入口、推荐礼品和公告。P01点击Next后切换到另一张营销图，移动菜单可展开；P26为别名访问，没有重复测试全部交互。公告可关闭，页脚也有公告入口。

**INFERRED** DOM：`#hth-slide.carousel > .carousel-inner > .item`、`.home-links`、`.gift-area .thumbnail`，公告为Layer iframe。**UNKNOWN**：轮播自动间隔的实际计时、键盘/手势全覆盖。无业务表单可做空输入测试。

静态交叉来源：[脱敏HTML](evidence/static/home.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P02 卡券鉴真

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/card_verify.html。入口：顶栏验证真伪。

**OBSERVED** 页面功能及内容：卡号/密码、眼睛图标、确定提交；文案说明可查真伪、有效期、对应商品。

证据：[浏览器AX](evidence/ax/02-verify-empty.txt) · [桌面截图](evidence/screenshots/02-verify-desktop.png) · [移动截图](evidence/screenshots/02-verify-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`bianma`卡号：email/max32，`pass`密码：password/max20；均由JS检查非空。按钮“确定提交”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：清空卡号、密码后点击“确定提交”，弹窗“请输入卡号”，关闭提示；未填写卡号格式错误用例，以免猜测凭证。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

静态交叉来源：[脱敏HTML](evidence/static/verify.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P03 预约提货

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/card_exchange.html。入口：顶栏预约提货。

**OBSERVED** 页面功能及内容：卡号/密码、眼睛图标、立即预约；这是预约前的卡券入口。

证据：[浏览器AX](evidence/ax/03-exchange-empty.txt) · [桌面截图](evidence/screenshots/03-exchange-desktop.png) · [移动截图](evidence/screenshots/03-exchange-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`bianma`卡号：email/max32，`pass`密码：password/max20；均由JS检查非空。按钮“立即预约”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：清空卡号、密码后点击“立即预约”，弹窗“请输入卡号”，关闭提示；未进入选品或预约日期。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

静态交叉来源：[脱敏HTML](evidence/static/exchange.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P04 订单查询

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/order_query.html。入口：顶栏订单查询。

**OBSERVED** 页面功能及内容：订单号/收货手机、立即查询；文案声称可查状态、物流及修改未发货订单，后两者未执行。

证据：[浏览器AX](evidence/ax/04-order-empty.txt) · [桌面截图](evidence/screenshots/04-order-desktop.png) · [移动截图](evidence/screenshots/04-order-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`bianma_jd`：email/max32，`phone`：tel/max11；JS非空。按钮“立即查询”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：清空订单号与手机号后点击“立即查询”，弹窗“请输入提货卡号或订单号”，关闭提示；未输入任何订单/手机号组合。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

静态交叉来源：[脱敏HTML](evidence/static/order.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P05 售后反馈

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/feedback.html。入口：顶栏售后反馈。

**OBSERVED** 页面功能及内容：订单号、联系人、手机号、邮箱、反馈内容和立即提交。

证据：[浏览器AX](evidence/ax/05-feedback-empty.txt) · [桌面截图](evidence/screenshots/05-feedback-desktop.png) · [移动截图](evidence/screenshots/05-feedback-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`order_sn` text/max30选填；`xm` text/max20必填；`phone` text/max12必填且旧手机号正则校验；`email` text/max100选填；`message` textarea/max210必填，文案写200字符。按钮“立即提交”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：空姓名提交显示“请填写联系人姓名”；姓名TEST、手机abc提交显示“手机号码有误，请重填”；随后清空测试值。未提交有效反馈。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

静态交叉来源：[脱敏HTML](evidence/static/feedback.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P06 礼品展示

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product.html。入口：顶栏礼品展示。

**OBSERVED** 页面功能及内容：七个礼品图文链接，无观察到筛选、搜索、分页、数量选择。

证据：[浏览器AX](evidence/ax/06-products.txt) · [桌面截图](evidence/screenshots/06-products-desktop.png) · [移动截图](evidence/screenshots/06-products-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：七个列表项对应详情ID 57、17、84、88、13、86、78；分别打开这些页面核验。桌面栅格、移动两列；没有观察到分页或筛选。

**INFERRED** DOM：`.container > .row > .col-md-3.col-xs-6 > .thumbnail > a + .caption`。注释中的“兑换”不是运行入口。**UNKNOWN**：礼品列表是否由卡券权限过滤、是否有登录后选品模式。无输入框及空输入测试。

静态交叉来源：[脱敏HTML](evidence/static/products.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P07 礼品列表入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_order。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：与礼品展示相同的七个礼品；不能把路径当成下单页面。

证据：[浏览器AX](evidence/ax/07-product-order.txt) · [桌面截图](evidence/screenshots/07-product-order-desktop.png) · [移动截图](evidence/screenshots/07-product-order-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：七个列表项对应详情ID 57、17、84、88、13、86、78；分别打开这些页面核验。桌面栅格、移动两列；没有观察到分页或筛选。

**INFERRED** DOM：`.container > .row > .col-md-3.col-xs-6 > .thumbnail > a + .caption`。注释中的“兑换”不是运行入口。**UNKNOWN**：礼品列表是否由卡券权限过滤、是否有登录后选品模式。无输入框及空输入测试。

静态交叉来源：[脱敏HTML](evidence/static/product-order.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P08 关于我们

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/about_us。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：公司与系统介绍，出现多商品/多卡型等营销描述。

证据：[浏览器AX](evidence/ax/08-about.txt) · [桌面截图](evidence/screenshots/08-about-desktop.png) · [移动截图](evidence/screenshots/08-about-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：信息展示页，公共导航可见，无业务输入及提交按钮。移动内容纵向排布。未发起电话、邮箱或微信通信。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > .col-xs-12 > h3.title + .content`，图文及公司介绍。**UNKNOWN**：营销声明是否对应当前演示站真实启用功能；别名页面后端路由关系。无输入校验可测试。

静态交叉来源：[脱敏HTML](evidence/static/about.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P09 联系我们

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/contact_us。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：公司名称、公开商务联系方式、地址和公众号二维码；未拨号、扫码或联系。

证据：[浏览器AX](evidence/ax/09-contact.txt) · [桌面截图](evidence/screenshots/09-contact-desktop.png) · [移动截图](evidence/screenshots/09-contact-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：信息展示页，公共导航可见，无业务输入及提交按钮。移动内容纵向排布。未发起电话、邮箱或微信通信。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > .col-xs-12 > h3.title + .content`，图文及公司介绍。**UNKNOWN**：营销声明是否对应当前演示站真实启用功能；别名页面后端路由关系。无输入校验可测试。

静态交叉来源：[脱敏HTML](evidence/static/contact.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P10 大闸蟹288礼包

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=57。入口：首页推荐/礼品列表。

**OBSERVED** 页面功能及内容：市场价288元、提货测试提示、产地江苏；演示内容，不作为真实商品事实。

证据：[浏览器AX](evidence/ax/10-detail-57.txt) · [桌面截图](evidence/screenshots/10-detail-57-desktop.png) · [移动截图](evidence/screenshots/10-detail-57-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-57.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P11 598大礼包

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=17。入口：首页推荐/礼品列表。

**OBSERVED** 页面功能及内容：属性含净重3KG、产地陕西韩城、真空包装。

证据：[浏览器AX](evidence/ax/11-detail-17.txt) · [桌面截图](evidence/screenshots/11-detail-17-desktop.png) · [移动截图](evidence/screenshots/11-detail-17-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-17.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P12 延安苹果9枚装

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=84。入口：首页推荐/礼品列表。

**OBSERVED** 页面功能及内容：属性含净重3KG、产地陕西韩城、真空包装，保留演示页面原意。

证据：[浏览器AX](evidence/ax/12-detail-84.txt) · [桌面截图](evidence/screenshots/12-detail-84-desktop.png) · [移动截图](evidence/screenshots/12-detail-84-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-84.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P13 花椒礼盒

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=88。入口：首页推荐/礼品列表。

**OBSERVED** 页面功能及内容：市场价999元、产地奥地利；演示数据可能不合常理，不能擅自修正为业务规则。

证据：[浏览器AX](evidence/ax/13-detail-88.txt) · [桌面截图](evidence/screenshots/13-detail-88-desktop.png) · [移动截图](evidence/screenshots/13-detail-88-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-88.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P14 有机胚芽米

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=13。入口：礼品列表。

**OBSERVED** 页面功能及内容：净重10KG、产地吉林。

证据：[浏览器AX](evidence/ax/14-detail-13.txt) · [桌面截图](evidence/screenshots/14-detail-13-desktop.png) · [移动截图](evidence/screenshots/14-detail-13-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-13.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P15 套餐一

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=86。入口：礼品列表。

**OBSERVED** 页面功能及内容：可见价格300。

证据：[浏览器AX](evidence/ax/15-detail-86.txt) · [桌面截图](evidence/screenshots/15-detail-86-desktop.png) · [移动截图](evidence/screenshots/15-detail-86-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-86.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P16 1688大礼包套餐

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=78。入口：礼品列表。

**OBSERVED** 页面功能及内容：图片和详情内容；AX没有读取到与前几页相同的属性表。

证据：[浏览器AX](evidence/ax/16-detail-78.txt) · [桌面截图](evidence/screenshots/16-detail-78-desktop.png) · [移动截图](evidence/screenshots/16-detail-78-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-78.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P17 微信小程序开发 各种平台小程序开发

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=92。入口：首页轮播。

**OBSERVED** 页面功能及内容：价格3888、规格定制、产地中国深圳，正文含可编辑详情的演示文案和图片。

证据：[浏览器AX](evidence/ax/17-detail-92.txt) · [桌面截图](evidence/screenshots/17-detail-92-desktop.png) · [移动截图](evidence/screenshots/17-detail-92-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-92.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P18 各种礼品卡提货系统开发定制

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=93。入口：首页轮播。

**OBSERVED** 页面功能及内容：价格3000、规格定制及大幅富文本图片；执行向下滚动观察。

证据：[浏览器AX](evidence/ax/18-detail-93.txt) · [桌面截图](evidence/screenshots/18-detail-93-desktop.png) · [移动截图](evidence/screenshots/18-detail-93-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-93.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P19 商品防伪防窜货追溯系统解决方案

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/product_detail.html?id=94。入口：首页轮播。

**OBSERVED** 页面功能及内容：价格6800、规格定制及图文；滚动后可见媒体区域，AX提示无法播放媒体，未启动播放。

证据：[浏览器AX](evidence/ax/19-detail-94.txt) · [桌面截图](evidence/screenshots/19-detail-94-desktop.png) · [移动截图](evidence/screenshots/19-detail-94-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED** 主要控件：公共导航及图片/富文本内容，未观察到商品专属数量框、选品勾选、加入购物车或兑换按钮。进入详情不是完成选品。移动为单列纵向内容；长图可超出当前视口。

**INFERRED** DOM：顶部图片区域与 `#inner-headline > .container.main-content` 中的标题、富文本/属性表，部分页面加载Swiper；见[03](03-ui-components.md)及[06](06-frontend-architecture.md)。无本页业务输入校验可测试。

**UNKNOWN**：该详情与卡券可兑换集合的关系、价格是否参与结算、套餐组成与库存。未执行任何兑换。

静态交叉来源：[脱敏HTML](evidence/static/detail-94.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P20 系统公告

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/notice.html。入口：首页自动弹窗/页脚系统公告。

**OBSERVED** 页面功能及内容：独立公告文档，也嵌在Layer弹窗中；2022年日期为历史文案，不能当作当前停发安排。

证据：[浏览器AX](evidence/ax/20-notice.txt) · [桌面截图](evidence/screenshots/20-notice-desktop.png) · [移动截图](evidence/screenshots/20-notice-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：既观察首页公告弹窗，也单独打开公告URL；内容为历史公告文本，没有业务输入。浏览器Network捕获GET 200，见[05-N01](05-api-inventory.md)。弹窗截图：[桌面](evidence/screenshots/20-notice-modal-desktop.png)、[移动](evidence/screenshots/20-notice-modal-mobile.png)。

**INFERRED** DOM：`.notice`正文容器和段落，独立加载v1.css。**UNKNOWN**：公告发布/关闭持久化策略及后台管理方式。

静态交叉来源：[脱敏HTML](evidence/static/notice.sanitized.html)（独立HTTP取回，非运行时DOM）。

## P21 关于我们 .html入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/about_us.html。入口：页脚关于我们。

**OBSERVED** 页面功能及内容：与P08观察内容相同；独立访问并截图。

证据：[浏览器AX](evidence/ax/21-about-html.txt) · [桌面截图](evidence/screenshots/21-about-html-desktop.png) · [移动截图](evidence/screenshots/21-about-html-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：信息展示页，公共导航可见，无业务输入及提交按钮。移动内容纵向排布。未发起电话、邮箱或微信通信。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > .col-xs-12 > h3.title + .content`，图文及公司介绍。**UNKNOWN**：营销声明是否对应当前演示站真实启用功能；别名页面后端路由关系。无输入校验可测试。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。

## P22 联系我们 .html入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/contact_us.html。入口：页脚联系方式。

**OBSERVED** 页面功能及内容：与P09观察内容相同；独立访问并截图。

证据：[浏览器AX](evidence/ax/22-contact-html.txt) · [桌面截图](evidence/screenshots/22-contact-html-desktop.png) · [移动截图](evidence/screenshots/22-contact-html-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：信息展示页，公共导航可见，无业务输入及提交按钮。移动内容纵向排布。未发起电话、邮箱或微信通信。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > .col-xs-12 > h3.title + .content`，图文及公司介绍。**UNKNOWN**：营销声明是否对应当前演示站真实启用功能；别名页面后端路由关系。无输入校验可测试。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。

## P23 预约无后缀入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/card_exchange。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：与P03观察内容相同，清空预填字段后截图；没有再次提交。

证据：[浏览器AX](evidence/ax/23-exchange-noext.txt) · [桌面截图](evidence/screenshots/23-exchange-noext-desktop.png) · [移动截图](evidence/screenshots/23-exchange-noext-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`bianma`卡号：email/max32，`pass`密码：password/max20；均由JS检查非空。按钮“立即预约”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：清空页面预填字段并截图；没有重复执行主入口的提交测试。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。

## P24 查询无后缀入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/order_query。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：与P04观察内容相同，清空预填字段后截图；没有再次提交。

证据：[浏览器AX](evidence/ax/24-order-noext.txt) · [桌面截图](evidence/screenshots/24-order-noext-desktop.png) · [移动截图](evidence/screenshots/24-order-noext-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`bianma_jd`：email/max32，`phone`：tel/max11；JS非空。按钮“立即查询”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：清空页面预填字段并截图；没有重复执行主入口的提交测试。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。

## P25 反馈无后缀入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/feedback。入口：首页快捷入口。

**OBSERVED** 页面功能及内容：与P05观察内容相同，未提交。

证据：[浏览器AX](evidence/ax/25-feedback-noext.txt) · [桌面截图](evidence/screenshots/25-feedback-noext-desktop.png) · [移动截图](evidence/screenshots/25-feedback-noext-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**INFERRED** DOM：`#inner-headline > .container.main-content > .row > … > form#order-form`，反馈使用fieldset，其余使用居中login-form/input-group。字段：`order_sn` text/max30选填；`xm` text/max20必填；`phone` text/max12必填且旧手机号正则校验；`email` text/max100选填；`message` textarea/max210必填，文案写200字符。按钮“立即提交”。 未把HTML类型email当成卡号业务格式。详细规则及行号见[字段契约](03-ui-components.md)。

**OBSERVED** 交互：观察相同反馈表单并截图，未再次提交。

**UNKNOWN**：服务端响应、验证码挑战及通过后流程、认证要求。没有执行密码显隐测试或成功提交。移动观察为纵向表单和固定底栏；别名页后端等价性未确认。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。

## P26 首页 index入口

**OBSERVED** 实际URL：https://demo.haotihuo.com/home/index/index.html。入口：移动底栏/静态成功回跳目标。

**OBSERVED** 页面功能及内容：观察到首页及自动公告；本次直接访问此公开链接，没有执行反馈成功回跳。

证据：[浏览器AX](evidence/ax/26-home-index.txt) · [桌面截图](evidence/screenshots/26-home-index-desktop.png) · [移动截图](evidence/screenshots/26-home-index-mobile.png)。操作：打开该公开链接，读取页面，在桌面与移动模拟视口观察；以下另列实际交互。

**OBSERVED**：首页有轮播、快捷入口、推荐礼品和公告。P01点击Next后切换到另一张营销图，移动菜单可展开；P26为别名访问，没有重复测试全部交互。公告可关闭，页脚也有公告入口。

**INFERRED** DOM：`#hth-slide.carousel > .carousel-inner > .item`、`.home-links`、`.gift-area .thumbnail`，公告为Layer iframe。**UNKNOWN**：轮播自动间隔的实际计时、键盘/手势全覆盖。无业务表单可做空输入测试。

此别名未另存独立HTTP源码；结构判断仅与同内容主入口对照，不宣称源码完全相同。


P26补充 **OBSERVED**：桌面公告已打开后切换移动视口，弹窗宽度未随之适配，正文被横向截断；这是该操作顺序下的表现，不能等同移动初次加载。见[证据方法](08-evidence-index.md)。
