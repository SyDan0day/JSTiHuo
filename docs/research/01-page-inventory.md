# 01 页面清单与调查范围

调查日期：2026-10-11（Asia/Shanghai；采集记录另含UTC）。目标：https://demo.haotihuo.com/ 。只进行了公开用户端调查与文档整理。

**OBSERVED**：共访问26个实际URL，覆盖用户要求的8类入口、10个商品/营销详情和公告。把7个重复入口合并后为19个内容页面。每个URL均有桌面与390×844移动模拟视口截图；截图为视口内容，不是整页拼接。逐页操作与证据见[02](02-page-details.md)，机器可读映射见[页面记录](evidence/page-observations.json)。该计数不包括未进入的选品、地址、订单详情和物流页面。

**OBSERVED**：执行了四个空输入校验、一个反馈手机格式错误校验、首页轮播下一项、移动菜单展开、公告展示/关闭及详情滚动。没有使用预填卡密或订单值完成业务。**UNKNOWN**：真实卡券后的业务页面、成功/服务端失败状态与一卡多商品规则。

## 实际可达URL

以下条目均为 **OBSERVED**；来源为各行对应的浏览器AX和截图，非仅枚举猜测路径。入口列表示链接发现位置；部分链接通过地址栏打开核验，不代表每条边都实际点击过。

|编号|页面|实际URL|入口|独立记录|
|---|---|---|---|---|
|P01|首页|[https://demo.haotihuo.com/](https://demo.haotihuo.com/)|首页导航|[AX](evidence/ax/01-home.txt) / [桌面](evidence/screenshots/01-home-desktop.png) / [移动](evidence/screenshots/01-home-mobile.png)|
|P02|卡券鉴真|[https://demo.haotihuo.com/home/index/card_verify.html](https://demo.haotihuo.com/home/index/card_verify.html)|顶栏验证真伪|[AX](evidence/ax/02-verify-empty.txt) / [桌面](evidence/screenshots/02-verify-desktop.png) / [移动](evidence/screenshots/02-verify-mobile.png)|
|P03|预约提货|[https://demo.haotihuo.com/home/index/card_exchange.html](https://demo.haotihuo.com/home/index/card_exchange.html)|顶栏预约提货|[AX](evidence/ax/03-exchange-empty.txt) / [桌面](evidence/screenshots/03-exchange-desktop.png) / [移动](evidence/screenshots/03-exchange-mobile.png)|
|P04|订单查询|[https://demo.haotihuo.com/home/index/order_query.html](https://demo.haotihuo.com/home/index/order_query.html)|顶栏订单查询|[AX](evidence/ax/04-order-empty.txt) / [桌面](evidence/screenshots/04-order-desktop.png) / [移动](evidence/screenshots/04-order-mobile.png)|
|P05|售后反馈|[https://demo.haotihuo.com/home/index/feedback.html](https://demo.haotihuo.com/home/index/feedback.html)|顶栏售后反馈|[AX](evidence/ax/05-feedback-empty.txt) / [桌面](evidence/screenshots/05-feedback-desktop.png) / [移动](evidence/screenshots/05-feedback-mobile.png)|
|P06|礼品展示|[https://demo.haotihuo.com/home/index/product.html](https://demo.haotihuo.com/home/index/product.html)|顶栏礼品展示|[AX](evidence/ax/06-products.txt) / [桌面](evidence/screenshots/06-products-desktop.png) / [移动](evidence/screenshots/06-products-mobile.png)|
|P07|礼品列表入口|[https://demo.haotihuo.com/home/index/product_order](https://demo.haotihuo.com/home/index/product_order)|首页快捷入口|[AX](evidence/ax/07-product-order.txt) / [桌面](evidence/screenshots/07-product-order-desktop.png) / [移动](evidence/screenshots/07-product-order-mobile.png)|
|P08|关于我们|[https://demo.haotihuo.com/home/index/about_us](https://demo.haotihuo.com/home/index/about_us)|首页快捷入口|[AX](evidence/ax/08-about.txt) / [桌面](evidence/screenshots/08-about-desktop.png) / [移动](evidence/screenshots/08-about-mobile.png)|
|P09|联系我们|[https://demo.haotihuo.com/home/index/contact_us](https://demo.haotihuo.com/home/index/contact_us)|首页快捷入口|[AX](evidence/ax/09-contact.txt) / [桌面](evidence/screenshots/09-contact-desktop.png) / [移动](evidence/screenshots/09-contact-mobile.png)|
|P10|大闸蟹288礼包|[https://demo.haotihuo.com/home/index/product_detail.html?id=57](https://demo.haotihuo.com/home/index/product_detail.html?id=57)|首页推荐/礼品列表|[AX](evidence/ax/10-detail-57.txt) / [桌面](evidence/screenshots/10-detail-57-desktop.png) / [移动](evidence/screenshots/10-detail-57-mobile.png)|
|P11|598大礼包|[https://demo.haotihuo.com/home/index/product_detail.html?id=17](https://demo.haotihuo.com/home/index/product_detail.html?id=17)|首页推荐/礼品列表|[AX](evidence/ax/11-detail-17.txt) / [桌面](evidence/screenshots/11-detail-17-desktop.png) / [移动](evidence/screenshots/11-detail-17-mobile.png)|
|P12|延安苹果9枚装|[https://demo.haotihuo.com/home/index/product_detail.html?id=84](https://demo.haotihuo.com/home/index/product_detail.html?id=84)|首页推荐/礼品列表|[AX](evidence/ax/12-detail-84.txt) / [桌面](evidence/screenshots/12-detail-84-desktop.png) / [移动](evidence/screenshots/12-detail-84-mobile.png)|
|P13|花椒礼盒|[https://demo.haotihuo.com/home/index/product_detail.html?id=88](https://demo.haotihuo.com/home/index/product_detail.html?id=88)|首页推荐/礼品列表|[AX](evidence/ax/13-detail-88.txt) / [桌面](evidence/screenshots/13-detail-88-desktop.png) / [移动](evidence/screenshots/13-detail-88-mobile.png)|
|P14|有机胚芽米|[https://demo.haotihuo.com/home/index/product_detail.html?id=13](https://demo.haotihuo.com/home/index/product_detail.html?id=13)|礼品列表|[AX](evidence/ax/14-detail-13.txt) / [桌面](evidence/screenshots/14-detail-13-desktop.png) / [移动](evidence/screenshots/14-detail-13-mobile.png)|
|P15|套餐一|[https://demo.haotihuo.com/home/index/product_detail.html?id=86](https://demo.haotihuo.com/home/index/product_detail.html?id=86)|礼品列表|[AX](evidence/ax/15-detail-86.txt) / [桌面](evidence/screenshots/15-detail-86-desktop.png) / [移动](evidence/screenshots/15-detail-86-mobile.png)|
|P16|1688大礼包套餐|[https://demo.haotihuo.com/home/index/product_detail.html?id=78](https://demo.haotihuo.com/home/index/product_detail.html?id=78)|礼品列表|[AX](evidence/ax/16-detail-78.txt) / [桌面](evidence/screenshots/16-detail-78-desktop.png) / [移动](evidence/screenshots/16-detail-78-mobile.png)|
|P17|微信小程序开发 各种平台小程序开发|[https://demo.haotihuo.com/home/index/product_detail.html?id=92](https://demo.haotihuo.com/home/index/product_detail.html?id=92)|首页轮播|[AX](evidence/ax/17-detail-92.txt) / [桌面](evidence/screenshots/17-detail-92-desktop.png) / [移动](evidence/screenshots/17-detail-92-mobile.png)|
|P18|各种礼品卡提货系统开发定制|[https://demo.haotihuo.com/home/index/product_detail.html?id=93](https://demo.haotihuo.com/home/index/product_detail.html?id=93)|首页轮播|[AX](evidence/ax/18-detail-93.txt) / [桌面](evidence/screenshots/18-detail-93-desktop.png) / [移动](evidence/screenshots/18-detail-93-mobile.png)|
|P19|商品防伪防窜货追溯系统解决方案|[https://demo.haotihuo.com/home/index/product_detail.html?id=94](https://demo.haotihuo.com/home/index/product_detail.html?id=94)|首页轮播|[AX](evidence/ax/19-detail-94.txt) / [桌面](evidence/screenshots/19-detail-94-desktop.png) / [移动](evidence/screenshots/19-detail-94-mobile.png)|
|P20|系统公告|[https://demo.haotihuo.com/home/index/notice.html](https://demo.haotihuo.com/home/index/notice.html)|首页自动弹窗/页脚系统公告|[AX](evidence/ax/20-notice.txt) / [桌面](evidence/screenshots/20-notice-desktop.png) / [移动](evidence/screenshots/20-notice-mobile.png)|
|P21|关于我们 .html入口|[https://demo.haotihuo.com/home/index/about_us.html](https://demo.haotihuo.com/home/index/about_us.html)|页脚关于我们|[AX](evidence/ax/21-about-html.txt) / [桌面](evidence/screenshots/21-about-html-desktop.png) / [移动](evidence/screenshots/21-about-html-mobile.png)|
|P22|联系我们 .html入口|[https://demo.haotihuo.com/home/index/contact_us.html](https://demo.haotihuo.com/home/index/contact_us.html)|页脚联系方式|[AX](evidence/ax/22-contact-html.txt) / [桌面](evidence/screenshots/22-contact-html-desktop.png) / [移动](evidence/screenshots/22-contact-html-mobile.png)|
|P23|预约无后缀入口|[https://demo.haotihuo.com/home/index/card_exchange](https://demo.haotihuo.com/home/index/card_exchange)|首页快捷入口|[AX](evidence/ax/23-exchange-noext.txt) / [桌面](evidence/screenshots/23-exchange-noext-desktop.png) / [移动](evidence/screenshots/23-exchange-noext-mobile.png)|
|P24|查询无后缀入口|[https://demo.haotihuo.com/home/index/order_query](https://demo.haotihuo.com/home/index/order_query)|首页快捷入口|[AX](evidence/ax/24-order-noext.txt) / [桌面](evidence/screenshots/24-order-noext-desktop.png) / [移动](evidence/screenshots/24-order-noext-mobile.png)|
|P25|反馈无后缀入口|[https://demo.haotihuo.com/home/index/feedback](https://demo.haotihuo.com/home/index/feedback)|首页快捷入口|[AX](evidence/ax/25-feedback-noext.txt) / [桌面](evidence/screenshots/25-feedback-noext-desktop.png) / [移动](evidence/screenshots/25-feedback-noext-mobile.png)|
|P26|首页 index入口|[https://demo.haotihuo.com/home/index/index.html](https://demo.haotihuo.com/home/index/index.html)|移动底栏/静态成功回跳目标|[AX](evidence/ax/26-home-index.txt) / [桌面](evidence/screenshots/26-home-index-desktop.png) / [移动](evidence/screenshots/26-home-index-mobile.png)|

## 阅读顺序与限制

[逐页记录](02-page-details.md) → [组件](03-ui-components.md) → [导航](04-navigation.md) → [请求](05-api-inventory.md) → [架构](06-frontend-architecture.md) → [业务流程](07-business-workflows.md) → [证据索引](08-evidence-index.md) → [未知项](09-unknowns.md) → [开发前需求](10-implementation-requirements.md)。

**OBSERVED**：Chrome插件连接失败后，使用Chrome原生界面及DevTools人工操作完成观察；未获得可直接调用的完整CDP能力。**UNKNOWN**：完整运行时DOM/CSS计算值、连续网络瀑布和Console无错状态。HTML/CSS结构来自另行公开HTTP取回的静态文件，均标为INFERRED，不能冒充实时DOM或浏览器响应。详情见[证据方法](08-evidence-index.md)。

**OBSERVED**：捕获并保存1条浏览器公告文档请求，业务JSON API为0条。**INFERRED**：静态发现3个Ajax接口及2个表单提交目标，见[05](05-api-inventory.md)。没有完成端到端兑换调查，也不能声明覆盖受凭证限制的全部页面。
