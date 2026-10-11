# 03 UI组件与交互规范调查

本文区分 **OBSERVED（浏览器实际观察）**、**INFERRED（静态源码事实及推断）**、**UNKNOWN（尚未确认）**。源码路径与行号用于定位，不代表相关成功业务已执行。独立开发应使用自有视觉资源，不把演示文案、旧校验或残留代码自动视为产品需求。

## 公共组件

| 组件 | 结论与证据 |
|---|---|
| 桌面导航 | **OBSERVED**：首页、验证真伪、预约提货、订单查询、售后反馈、礼品展示。访问首页：[AX](evidence/ax/01-home.txt)、[截图](evidence/screenshots/01-home-desktop.png)。**INFERRED**：`header > .navbar > .container > .navbar-header + .navbar-collapse > ul.nav.navbar-nav`；[源码](evidence/static/home.sanitized.html) 140–164行。 |
| 移动导航 | **OBSERVED**：390×844下出现折叠菜单和底栏，底栏为首页/鉴真/提货/进度/联系；[AX](evidence/ax/02-verify-mobile.txt)、[截图](evidence/screenshots/02-verify-mobile.png)、[展开菜单](evidence/screenshots/02-mobile-menu.png)。**INFERRED**：`.navbar-toggle` 的collapse目标为 `.navbar-collapse`；[首页源码](evidence/static/home.sanitized.html) 145–152行。 |
| 移动底栏 | **INFERRED**：`#mobile-footer > .footer-nav-wrapper > a.footer-nav-single > .menu-wrapper`，≤767px显示，固定底部、100%宽、z-index99、flex间隔排列；各项85×65px、字12px、图标22px。[CSS](evidence/static/v1.css) 543–590、620–629行。底部预留50px与栏高65px不同，是否遮挡末尾内容需另测。 |
| 服务承诺 | **OBSERVED**：正品保证、新鲜上市、快递包邮、售后无忧；[鉴真 AX](evidence/ax/02-verify-empty.txt)。**INFERRED**：`section#content > .container > .row > .info-blocks > .info-blocks-in`；[鉴真源码](evidence/static/verify.sanitized.html) 251–278行。 |
| 提货须知 | **OBSERVED**：鉴真/预约/查询显示静态须知，其中含卡密失效、有效期、发货日期和物流说明；[查询 AX](evidence/ax/04-order-empty.txt)。属于站点声明，未验证实际业务规则。**INFERRED**：`section#call-to-action-2 > .container > .row > h3 + ul`；[查询源码](evidence/static/order.sanitized.html) 262–290行。 |
| 提示弹窗 | **OBSERVED**：空输入与非法手机显示“提示信息”和“确定”；见下方验证表。**INFERRED**：由 `layer.alert` 生成；[反馈源码](evidence/static/feedback.sanitized.html) 416–435行。 |
| 系统公告 | **OBSERVED**：主页可见系统公告弹窗，内嵌公告页；[AX](evidence/ax/20-notice-modal.txt)。**INFERRED**：layer type2 iframe，来源 `/home/index/notice.html`；[首页源码](evidence/static/home.sanitized.html) 481–508行。 |
| 返回顶部 | **INFERRED**：滚动超过100时显示 `#top-back`，点击300ms动画返回顶部；[首页源码](evidence/static/home.sanitized.html) 511–524行。运行/键盘可达性未验证。 |

## 页面主体组件与DOM

下表结构均为 **INFERRED**，由已脱敏静态HTML读取；运行画面另以截图补证。

| 页面 | 主要层级 | 来源 |
|---|---|---|
| 首页 | `#hth-slide.carousel.slide > .carousel-inner > .item`；`.row.home-links > .col-md-2.col-xs-4 > .category-item__image + .category-item__title`；`.container.gift-area … .col-md-3.col-xs-6 > .row > .thumbnail` | [home](evidence/static/home.sanitized.html) 190–215、220–259、265–308行 |
| 鉴真 | `#inner-headline > .container.main-content > .row > .col-xs-12.col-lg-4.col-lg-offset-4 > .login-form > form#order-form`；结果区 `#tishi`、`.row#msg` | [verify](evidence/static/verify.sanitized.html) 197–248行 |
| 预约 | 同鉴真居中表单；`.tabs`、`.form-group > .input-group`、`.passhide`、`.pintu`、`#smt` | [exchange](evidence/static/exchange.sanitized.html) 180–226行 |
| 查询 | 同居中表单，字段 `#bianma/#phone`，提交 `#smt` | [order](evidence/static/order.sanitized.html) 178–224行 |
| 反馈 | `#inner-headline > .container.main-content > .row > .col-xs-12.col-lg-6.col-lg-offset-3.order-main > form#order-form > .tabs + fieldset > .form-group` | [feedback](evidence/static/feedback.sanitized.html) 177–221行 |
| 礼品展示/列表 | `#inner-headline > .container > .row > .col-md-3.col-xs-6 > .row > .thumbnail > a + .caption.text-center > h3` | [products](evidence/static/products.sanitized.html) 196–270行；[product-order](evidence/static/product-order.sanitized.html) 同范围 |
| 关于/联系 | `#inner-headline > .container.main-content > .row > .col-xs-12 > h3.title + .content` | [about](evidence/static/about.sanitized.html) 171–177行；[contact](evidence/static/contact.sanitized.html) 174–179行 |
| 商品/营销详情 | `.swiper-container` 图片区；随后采用关于页同类正文层级，正文含表格或富文本图片 | [详情57](evidence/static/detail-57.sanitized.html) 179–194行；[详情92](evidence/static/detail-92.sanitized.html) 179–193行 |

**OBSERVED**：礼品列表显示图片、名称及链接：[列表 AX](evidence/ax/06-products.txt)、[桌面截图](evidence/screenshots/06-products-desktop.png)、[移动截图](evidence/screenshots/06-products-mobile.png)。**INFERRED**：列表栅格声明桌面4列、窄屏2列；实际断点由Bootstrap定义，不能仅由此确认像素阈值。

**INFERRED**：已读取7份详情HTML（17/57/84/88/92/93/94）移除注释后，均无form/input及商品专属兑换控件；仍有通用预约导航。例：[详情57](evidence/static/detail-57.sanitized.html) 182–198行正文、156/264行公共预约链接。列表中的 `order_one.html` “兑换”按钮处于注释中（[列表源码](evidence/static/products.sanitized.html) 204、214等行），不能列为可点击兑换组件。

## 字段契约（静态）

下表均为 **INFERRED**；“必填”指前端JS校验或文案，不代表服务端规则。隐藏 `__token__` 已脱敏；`yzm`仅id无name，是否有验证脚本追加字段为 **UNKNOWN**。

| 表单 | 字段 | HTML类型/长度 | 必填及校验 | 来源 |
|---|---|---|---|---|
| 鉴真/预约 | `bianma` 卡号 | email；max32 | JS非空；不能因此要求用户输入邮箱 | [鉴真](evidence/static/verify.sanitized.html) 217、471–474；[预约](evidence/static/exchange.sanitized.html) 199、452–455 |
| 鉴真/预约 | `pass` 密码 | password；max20 | JS非空；有显隐切换脚本 | [鉴真](evidence/static/verify.sanitized.html) 225、475–478、510–518 |
| 查询 | `bianma_jd` 订单号 | email；max32 | JS非空，提示同时接受提货卡号或订单号 | [查询](evidence/static/order.sanitized.html) 200、449–452 |
| 查询 | `phone` 收货手机 | tel；max11 | JS非空；已读内联代码没有格式正则 | [查询](evidence/static/order.sanitized.html) 208、453–456 |
| 反馈 | `order_sn` | text；max30 | 选填 | [反馈](evidence/static/feedback.sanitized.html) 192 |
| 反馈 | `xm` 联系人 | text；max20 | 非空 | 同上196、416–419 |
| 反馈 | `phone` | text；max12 | 非空且匹配 `/^1[34578]\d{9}$/` | 同上200、420–427 |
| 反馈 | `email` | text；max100 | 选填，未见内联邮箱格式校验 | 同上205、408–437 |
| 反馈 | `message` | textarea；max210 | 非空；提示“200字符以内”，与max不一致 | 同上210、429–432 |

**UNKNOWN**：空格trim策略、服务端长度、Unicode计数、真实卡号格式、密码格式、现代手机号码范围与反馈200/210最终契约。后续开发不应机械继承旧手机号正则与类型误用。

## 已执行的安全校验

| 操作 | OBSERVED结果 | 浏览器证据 |
|---|---|---|
| 鉴真保持空字段点击确定提交 | “请输入卡号” | [AX](evidence/ax/02-verify-empty.txt)、[截图](evidence/screenshots/02-verify-empty-desktop.png) |
| 预约保持空字段点击立即预约 | “请输入卡号” | [AX](evidence/ax/03-exchange-empty.txt)、[截图](evidence/screenshots/03-exchange-empty.png) |
| 查询保持空字段点击立即查询 | “请输入提货卡号或订单号” | [AX](evidence/ax/04-order-empty.txt)、[截图](evidence/screenshots/04-order-empty.png) |
| 反馈空姓名点击立即提交 | “请填写联系人姓名” | [AX](evidence/ax/05-feedback-empty.txt)、[截图](evidence/screenshots/05-feedback-empty.png) |
| 反馈联系人填测试值TEST，手机填abc并点击 | “手机号码有误，请重填” | [AX](evidence/ax/05-feedback-invalid-phone.txt)、[截图](evidence/screenshots/05-feedback-invalid-phone.png) |

这些结果只证明本地交互提示，不证明业务POST曾发出或服务端校验成功。未用真实卡密、订单号或验证码继续测试。

## 动态状态与待验证项

**INFERRED**：鉴真脚本在提交时禁用按钮，显示“验证中，请稍候...”；成功分支清卡密、显示 `tishi/product`，失败分支弹窗。预约和查询成功分支触发表单跳转，反馈成功分支reset并转首页。这些均未实际进入成功状态，见 [鉴真](evidence/static/verify.sanitized.html) 485–505、[预约](evidence/static/exchange.sanitized.html) 467–484、[查询](evidence/static/order.sanitized.html) 464–480、[反馈](evidence/static/feedback.sanitized.html) 439–457行。

**INFERRED**：拼图入口容器初始 `display:none`，脚本在yzm为空时调用 `.pintu.click()`；验证机制、触发条件、可见性与回调有效性为 **UNKNOWN**，不得绕过或补造值。[预约源码](evidence/static/exchange.sanitized.html) 214–216、460–463、498–502行。

**UNKNOWN**：键盘焦点管理、屏幕阅读器标签完备性、密码显隐实际运行、触摸滑动完整行为、慢网/超时/断网恢复、长文本、错误接口返回、成功结果HTML结构、真实选品与地址组件。公开静态UI可以作为页面骨架依据；涉及核销与下单的组件须补齐业务证据后再定义。
