# 08 证据索引与采集方法

调查时间：2026-10-11，时区Asia/Shanghai。证据仅针对本次可见公开用户端，不能证明未来站点状态或未访问分支。

## 标签和来源层级

- **OBSERVED**：Chrome实际页面/交互/Network观察，引用截图、可访问性树（AX）或请求文本。
- **INFERRED**：公开HTML/CSS的静态事实与有限推断；即使源码明确存在，也不表示该代码已被触发或服务端支持该分支。
- **UNKNOWN**：缺少足够证据；不会使用路径、营销词、静态回调构造确定的后端或业务结论。

## 文件分组

|证据组|来源/保存方法|索引与适用范围|
|---|---|---|
|P01–P26|Chrome桌面界面访问；读取AX；切换Responsive390×844；截图|[页面映射JSON](evidence/page-observations.json)、[逐页链接](02-page-details.md)，各URL独立桌面/移动记录|
|AX文本|仅保留目标站页面子树；敏感输入值清空或脱敏|`evidence/ax/`；不是DOM快照，不包含完整标签/CSS/事件|
|截图63张|Chrome原生截图裁出页面内容；35张桌面/操作图3022×1474，28张移动600×1300|`evidence/screenshots/`；移动原模拟视口390×844，像素尺寸来自屏幕缩放，不代表设备分辨率|
|N01|Chrome Network手工选择公告，抄取白名单Headers、Response、Initiator|[Headers](evidence/network/N01-notice-headers-whitelist.txt)、[Response](evidence/network/N01-notice-response.html)、[Initiator](evidence/network/N01-notice-initiator.txt)、[截图](evidence/screenshots/N01-notice-initiator.png)|
|S-*共20条|独立公开HTTP GET，不显式提供凭证、不使用Cookie jar；取回后先脱敏再写文件|[元数据](evidence/network/public-http-metadata.json)、`evidence/static/*.sanitized.html`；**不是浏览器网络捕获**|
|v1.css|公开引用样式文件另行取回，静态分析|[CSS](evidence/static/v1.css)，不代表computed style|
|采集工具|仅研究用的公开GET/脱敏辅助脚本|[脚本](evidence/tools/collect_public_sources.py)；初始批次16页，补充3个详情与公告4页，实际20条以元数据为准；没有业务开发代码|
|校验清单|交付文件大小和SHA-256|[manifest](evidence/manifest.json)，哈希证明交付版本一致性，不是源站签名|

## 关键操作与证据

|操作|标签与结果|保存证据|
|---|---|---|
|访问八类公共入口、10详情、公告及重复入口|**OBSERVED** 26URL可达；19个合并内容页面|[页面清单](01-page-inventory.md)、[逐页记录](02-page-details.md)|
|空鉴真/预约/查询/反馈|**OBSERVED** 四个字段提示|[组件验证表](03-ui-components.md)|
|反馈姓名TEST、手机abc|**OBSERVED** 手机格式提示；测试后清空|[AX](evidence/ax/05-feedback-invalid-phone.txt)、[截图](evidence/screenshots/05-feedback-invalid-phone.png)|
|首页点击Next|**OBSERVED** 切换轮播图|[截图](evidence/screenshots/01-home-carousel-next.png)|
|移动菜单展开|**OBSERVED** 显示折叠菜单|[截图](evidence/screenshots/02-mobile-menu.png)|
|公告展示/关闭/独立打开|**OBSERVED** iframe内容可读|[弹窗AX](evidence/ax/20-notice-modal.txt)、[桌面](evidence/screenshots/20-notice-modal-desktop.png)、[移动](evidence/screenshots/20-notice-modal-mobile.png)|
|详情滚动|**OBSERVED** 富文本长图、属性及媒体区域|[93下部截图](evidence/screenshots/18-detail-93-bottom.png)、[94 AX](evidence/ax/19-detail-94.txt)|
|首页刷新观察Network|**OBSERVED** N01 GET200 HTML|[接口记录](05-api-inventory.md)|

## 截图的具体限制

**OBSERVED**：P26移动截图来自桌面公告打开后切换390px视口，公告仍保持较宽尺寸，正文横向被截；这证明该操作顺序的显示结果，不能直接泛化为移动端初次加载一定有此问题。[P26移动](evidence/screenshots/26-home-index-mobile.png)。在390px下刷新首页时另有适配的公告画面，见[N01截图](evidence/screenshots/N01-notice-initiator.png)。独立公告页与弹窗是不同上下文。

**OBSERVED**：截图含普通指针高亮或站内URL悬停条；不影响主要页面证据。长详情截图可能在滚动后位置；没有将其命名或描述为全页截图。页面公开公司商务联系方式/公众号属于站点展示内容，保留用于页面结构证据；未采集真实用户的姓名、收货地址、订单或联系方式。

截图已裁除无关Chrome标签、地址栏、书签及浏览器用户头像。表单预填卡密/订单信息在保存最终证据前清空；保留的测试文本仅TEST/abc。静态HTML中所有input value均被替换为`[REDACTED]`，并进一步处理敏感赋值与长数字；这会使部分资源文件名失真，因此不能把脱敏后的文件当可运行镜像或精确资源URL清单。

## 网络与环境限制

**OBSERVED（操作记录）**：优先Chrome插件后连接失败，改用Chrome原生UI；没有完整CDP记录器、自动DOM快照或持续网络捕获。Network列表含浏览器扩展资源，截图显示的38请求/传输总量不能归为站点性能数据。只对N01保存明确请求归属，不把其他行补写为业务API。

**OBSERVED（操作记录）**：Console粘贴保护未绕过；不将Agent输入造成的SyntaxError算作站点故障。未记录Cookie/会话头/令牌值；没有导出原始HAR。工具限制详见[09](09-unknowns.md)。

**INFERRED**：静态源码可能随时间、会话与服务端配置改变；行号只对应当前交付文件。请先依据manifest确认文件未变化，再复核行号。独立HTTP的200不证明浏览器全部导航状态码，也不能证明业务POST不需要认证。
