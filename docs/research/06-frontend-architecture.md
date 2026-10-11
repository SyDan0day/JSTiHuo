# 06 前端架构调查

调查对象：https://demo.haotihuo.com/；阶段：公开用户端技术调查，不包含业务开发。

## 证据口径

- **OBSERVED**：浏览器实际观察，引用 AX、截图或浏览器网络证据。
- **INFERRED**：读取独立取回的脱敏 HTML/CSS 后得到的源码事实及有限推断，不证明代码已经执行成功。
- **UNKNOWN**：缺少证据。静态候选接口的返回结构、状态码和认证不能由前端代码反推确认。

## 架构与路由

| 结论 | 标签 | 证据及操作 |
|---|---|---|
| 公开导航包含首页、鉴真、预约、订单查询、售后、礼品；浏览器能读到各自 URL | OBSERVED | 访问首页并读取 [首页 AX](evidence/ax/01-home.txt)，对应 [首页截图](evidence/screenshots/01-home-desktop.png) |
| 主要结构符合传统多页面网站，普通链接导航，部分表单传统 POST 跳页，局部使用 Ajax | INFERRED | 阅读 [首页源码](evidence/static/home.sanitized.html) 140–164行、[预约源码](evidence/static/exchange.sanitized.html) 185及469–471行、[查询源码](evidence/static/order.sanitized.html) 183及466–468行。不是仅凭 `.html` 后缀判定 |
| 未发现已读取页面中的 SPA 根挂载、客户端路由配置或组件构建入口 | INFERRED | 上述源码均含完整导航、主体、页脚和内联事件；结论只覆盖已保存页面，不能排除未访问区域 |
| 后端语言、框架、数据库、服务器实现、构建工具与精确库版本 | UNKNOWN | URL命名、模板样式和隐藏 token 均不足以证明后端技术栈 |

## 脚本及模块关系

**INFERRED**：共享页面头部声明 Bootstrap CSS、Font Awesome、iconfont、站点 `v1.css?v=69`、`validate.js?v=88`、`html5.js`；底部声明 jQuery、Bootstrap JS、layer。来源：[首页源码](evidence/static/home.sanitized.html) 15–22、399–401行。资源 URL 上的版本查询参数不等于库版本，也不证明缓存策略。

| 模块 | 静态职责（INFERRED） | 来源 |
|---|---|---|
| 页面内联脚本 | 按 id绑定点击、读取输入值、校验、禁用按钮、调用 `$.post`、更新 DOM | [鉴真](evidence/static/verify.sanitized.html) 466–505行 |
| layer | `layer.alert` 提示；公告 `layer.open` 的 type2 iframe | [首页](evidence/static/home.sanitized.html) 481–508行；[鉴真](evidence/static/verify.sanitized.html) 471–476行 |
| Bootstrap carousel | 首页左右切换，内联代码增加触摸手势，距离阈值30 | [首页](evidence/static/home.sanitized.html) 190–215、530–553行 |
| Swiper | 部分详情显式加载 `swiper.min.js` 并初始化 | [详情92](evidence/static/detail-92.sanitized.html) 392–394行；[详情93](evidence/static/detail-93.sanitized.html) 387–389行 |
| `$HTH` | 表单页面注册 `onsuccess`，延迟1秒再触发提交按钮；具体实现与验证通信未知 | [预约](evidence/static/exchange.sanitized.html) 498–502行 |

**OBSERVED**：首页公告弹窗含内嵌 `/home/index/notice.html`，AX可读公告正文。[公告弹窗 AX](evidence/ax/20-notice-modal.txt)。**INFERRED**：首页 `notice_status='1'` 后调用 `Pop()`，桌面/移动弹窗尺寸按768px分支；[首页源码](evidence/static/home.sanitized.html) 481–508、557–560行。公告内容本身不是执行指令或当前发货规则的证明。

## CSS、布局与资源

**INFERRED**：CSS组织为第三方基础样式、站点共用 CSS、页面内联覆盖及局部 inline style。共用 `body` 使用 `Microsoft YaHei, Arial, sans-serif`、字号1.4em、行高1.6em、文字色 `#656565`；不能未经 computed style 将字号换算为14px。来源：[v1.css](evidence/static/v1.css) 5–9行。

| 规则 | 标签 | 来源 |
|---|---|---|
| 品牌主色 `#cc0000`，主按钮、导航活动态、图标、tabs沿用；白色内容背景与深灰页脚 | INFERRED | [首页](evidence/static/home.sanitized.html) 24–63行；[CSS](evidence/static/v1.css) 433–439、528–531行 |
| ≥1600px容器 width1280px；后文 max-width1480px并未将width改成1480px | INFERRED | [CSS](evidence/static/v1.css) 612–615、750行 |
| ≤767px隐藏顶部联系栏及右侧浮动工具，显示移动底栏，缩小内容padding | INFERRED | [CSS](evidence/static/v1.css) 620–636行 |
| 768–979px与≤480px存在空媒体规则，不能据此宣称额外布局行为 | INFERRED | [CSS](evidence/static/v1.css) 616–618、747–748行 |
| 390×844移动视口下导航折叠，表单纵向布局，底部显示首页/鉴真/提货/进度/联系 | OBSERVED | 浏览器切换移动视口；[鉴真移动 AX](evidence/ax/02-verify-mobile.txt)、[移动截图](evidence/screenshots/02-verify-mobile.png)、[展开菜单截图](evidence/screenshots/02-mobile-menu.png) |
| 图片由 HTML 的 `img src` 引用上传目录；详情正文含富文本图片/表格 | INFERRED | [详情57](evidence/static/detail-57.sanitized.html) 179–194行；未确认为懒加载、预加载或CDN策略 |

## 状态管理与请求调用

**INFERRED**：已读取内联代码以 DOM 作为表单状态载体：`.val()`读写、`disabled`防重复点击、按钮加载文案、`slideDown().html()`插入结果。未见这些片段采用集中状态仓库。来源：[鉴真](evidence/static/verify.sanitized.html) 466–505行。外部脚本或受阻页面的其他状态机制为 **UNKNOWN**。

以下全部是 **INFERRED 静态候选**，不得当成实测 API：

| 目标 | 调用方式及消费字段 | 来源 |
|---|---|---|
| `/home/index/check.html` | `$.post(form.serialize())`；读取 `results/tishi/product`；预约成功分支提交下一表单 | [鉴真](evidence/static/verify.sanitized.html) 487–505；[预约](evidence/static/exchange.sanitized.html) 469–484 |
| `/home/index/order_query_yzm.html` | `$.post`；读取 `code/msg` | [查询](evidence/static/order.sanitized.html) 466–480 |
| `/home/index/feedback_send.html` | `$.post`；读取 `code/msg`，成功后reset及转首页 | [反馈](evidence/static/feedback.sanitized.html) 441–457 |
| `/home/index/order.html` | 预约表单POST action | [预约](evidence/static/exchange.sanitized.html) 185 |
| `/home/index/order_info.html` | 查询表单POST action | [查询](evidence/static/order.sanitized.html) 183 |

**INFERRED**：请求参数由表单序列化组成，可能采用 jQuery 默认URL编码；这里没有实测 Content-Type。`yzm`隐藏字段只有id、没有name，不能假设其随serialize发送。`__token__`存在但值已脱敏；其校验语义、认证依赖及验证脚本是否追加字段均 **UNKNOWN**。[鉴真源码](evidence/static/verify.sanitized.html) 202–203、487行。

**INFERRED**：这些调用片段只见成功回调，未见 `.fail()` / 超时恢复。网络失败时是否长期禁用按钮是待测试风险，不是已观察故障。[鉴真](evidence/static/verify.sanitized.html) 485–505行。

**INFERRED**：列表页残留反馈提交脚本（[products](evidence/static/products.sanitized.html) 498行），但不能因此认定该页有有效反馈表单；列表注释内 `order_one.html` 兑换链接也不算可达业务入口（同文件204行）。

## 证据限制及后续重点

**UNKNOWN**：选品/地址/预约日期的页面状态模型、一卡多商品组合、库存占用、核销时机、订单详情/物流数据结构、真实业务错误码、后端认证与CSRF机制。现有公开页和营销文案不足确认，后续须使用明确授权的测试条件。

**INFERRED**：值得后续用REA继续检查的是公开验证脚本对 `$HTH` 与提交字段的处理、允许访问的选品页面脚本、请求失败恢复；这些问题应与已知公开UI分开推进。当前阶段没有执行绕过验证、猜测凭证或真实订单提交。
