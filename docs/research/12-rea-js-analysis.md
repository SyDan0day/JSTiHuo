# 12 REA JavaScript 深度分析

## 执行环境与方法

**OBSERVED（工具诊断）**：本会话没有可调用的REA MCP工具；已安装REA CLI 6.4.0，`doctor --client codex`报告Codex注册missing/config_drift。本阶段实际使用REA CLI的同名能力连接用户指定`http://127.0.0.1:9222`，未修改全局配置或安装分析引擎。证据：[doctor](evidence/phase2/rea/doctor.json)、[目标列表](evidence/phase2/rea/targets-filtered.json)，目标列表Evidence `ev_a94c664160059d10332bf75e600968e4cc9b1dd8d331720e9517ccfba8e7fcb5`。Chrome版本155.0.8059.39。

**OBSERVED**：实际完成`list_browser_targets`、三个业务页和公开后台登录页的`inspect_web_page`、预约页`analyze_web_bundle`、四组`export_web_scripts`、`analyze_javascript_application`及`trace_application_feature`。只捕获目标origin的公开脚本，不执行捕获脚本，不取服务器源码。未重复第一阶段全站页面截图。

|证据/操作|结果及边界|
|---|---|
|[verify-capture](evidence/phase2/rea/verify-capture.json)|`ev_23111de9f51f9aceda9091a485009a359be28cc536bf0bcd42d48be47fb1c0eb`；鉴真页6段脚本（4外链、2内联）|
|[exchange-capture](evidence/phase2/rea/exchange-capture.json)|预约脚本与无值DOM结构|
|[order-capture](evidence/phase2/rea/order-capture.json)|查询脚本与无值DOM结构|
|[exchange-bundle](evidence/phase2/rea/exchange-bundle.json)|`ev_48aaa3d2bf31c03bd09fd2fc74d2f32ac3eae434be3325f49107d5feebc3d5cd`；静态端点发现，不是接口调用|
|[verify导出](evidence/phase2/verify-scripts/manifest.json)、[exchange导出](evidence/phase2/exchange-scripts/manifest.json)、[order导出](evidence/phase2/order-scripts/manifest.json)|脚本URL、来源、字节数与导出SHA-256映射；不以V8/CDP hash代替导出文件哈希|
|[admin导出](evidence/phase2/admin-scripts/manifest.json)|后台登录页4段脚本；入口key已脱敏，相关envelope是衍生副本|
|[聚焦分析](evidence/phase2/rea/focused-application.json)|`ev_25a59e497ad2f6df89be9303418fc4c86d2ac1699b9bb9c1c1c67f03f81cf572`；7文件、19,626字节、3,552 AST节点、0解析失败|
|[分析摘要](evidence/phase2/rea/focused-summary.json)|application coverage complete（限选定7文件）；semantic coverage unknown/各类partial，不能声称全部动态调用已解析|
|[check追踪](evidence/phase2/rea/check-feature-trace.json)|`ev_76fc98622329ff4038de53b4e7efcf19dff63c942bf06787b9642fd1b6e376f5`；2个端点种子、7节点、6边；限源内追踪，不是运行时调用栈|

## 脚本身份与关系

下列“捕获到脚本”是 **OBSERVED**；其职责解释为 **INFERRED**。完整来源映射见[focused-provenance](evidence/phase2/focused-provenance.json)。副本仅用于分析，不是新系统业务代码。

|公开URL或内联位置|角色与证据|
|---|---|
|`https://demo.haotihuo.com/static/home/js/validate.js?v=88`|自定义`pintu`验证组件、`_ajax`封装、`$HTH`别名；[导出副本](evidence/phase2/focused-sources/validate.js)|
|`https://demo.haotihuo.com/static/home/weiui/jquery.min.js`|DOM/event/serialize/POST公共依赖；三个捕获manifest均有记录|
|`https://demo.haotihuo.com/static/bootstrap/js/bootstrap.min.js`|公共Bootstrap交互依赖；不能据加载推断全部组件执行|
|`https://demo.haotihuo.com/static/layer/layer.js`|弹窗、公告iframe依赖|
|`card_verify.html`内联|匿名`#smt.click`、密码显隐、验证成功回调；[verify-1](evidence/phase2/focused-sources/verify-1.js)|
|`card_exchange.html`内联|匿名`#smt.click`、成功后表单submit；[exchange-1](evidence/phase2/focused-sources/exchange-1.js)|
|`order_query.html`内联|匿名`#smt.click`、结果码分支；[order-2](evidence/phase2/focused-sources/order-2.js)|
|三页公共内联|`Pop`、`topBack`；其余3个聚焦文件，不参与卡资格判定|
|后台`/static/admin/js/jquery.js`、`/static/admin/js/keyboard/keyboard.js`、`/static/layuiadmin/layui/layui.js`|登录表单依赖、软键盘、提示；[admin manifest](evidence/phase2/admin-scripts/manifest.json)|

**INFERRED**：代码组织为普通脚本+页面内联事件，`$HTH=pintu`把全局验证对象暴露给页面回调。未发现这些业务入口存在现代SPA模块/路由管理；此结论限采集范围。第一阶段`html5.js`为IE条件注释、`jqthumb.js`为禁用注释，不能混入当前Chrome加载清单，见[审核](11-first-stage-audit.md)。

## 本阶段新解决的问题

**INFERRED**：`window.onload`保留旧handler后调用`pintu.init()`；init创建验证DOM并绑定鼠标/触摸、关闭、刷新和`.pintu`点击。`show → refresh → Image.src`构造`/home/index/makecode?t=…`，不支持WebP时追加`nowebp=1`。拖动结束通过`_block_on_end → _send_result → _ajax.request`请求`/home/index/validate_code?xcode=…`。这是验证码子流程，不是商品/订单接口。[validate.js](evidence/phase2/focused-sources/validate.js)1–3行（压缩源，函数名定位）。

**INFERRED**：验证响应由文本成功条件判断；成功分支写`#yzm`本地标记，调用注册的`_onsuccess`，页面等待1秒再次触发`#smt.click()`。`#yzm`无name，组件代码未给它追加name，也未直接附加到业务serialize中；服务端如何绑定验证结果仍 **UNKNOWN**。本阶段没有拼图操作、手工设置标记或调用成功回调。

**INFERRED**：业务POST使用jQuery的serialize，发送表单实际值；验证码组件自己的`_ajax`只用于校验请求，不能把两套Ajax封装混成一条JSON POST协议。请求字段、失败分支及两个后续表单详见[13](13-api-call-graph.md)、[15](15-voucher-workflow.md)、[16](16-order-workflow.md)。

## 分析失败与覆盖限制

**OBSERVED（工具）**：首次静态分析因本地Swift辅助程序无法写入编译缓存失败：[失败证据](evidence/phase2/rea/verify-application.json)。按授权权限重试后解析6文件成功，但jQuery语义分析耗尽默认堆，整体图partial：[结果](evidence/phase2/rea/verify-application-complete.json)、[摘要](evidence/phase2/rea/verify-application-summary.json)。文件名complete只表示该次命令正常结束，不能代表图覆盖完整。

**OBSERVED（工具）**：随后聚焦三页内联+验证脚本7文件，解决第三方库引入的资源问题；保留全部原失败及partial输出。**UNKNOWN**：动态jQuery分派、服务端规则、卡券后续页面；REA图的0个module或request语义关系不代表没有函数或请求，手工源代码定位仍必要。

**OBSERVED（工具）**：`inspect_web_page`跨导航失败、刷新导致target_changed等见[14](14-network-evidence.md)；正常导航网络另由REA受控场景采集。URL入口key脱敏修改过的REA JSON不再是可验证原始envelope；原Evidence ID仅用于关联，详情见[脱敏记录](evidence/phase2/redaction-ledger.json)。
