# 14 第二阶段网络证据

## 实际采集结果

**OBSERVED**：REA受控场景在用户指定CDP页执行普通导航，得到50个请求事务、24个去重URL，全部GET、全部状态200；其中用户端三页39次，后台登录11次。没有卡券、兑换、查询、反馈或后台登录POST。这个结果不能证明这些业务接口不存在。每个事务的URL/Method/Status/参数名/捕获步骤/事件序号已保存于[network-requests.json](evidence/phase2/network-requests.json)。

- 用户端场景：[输入](evidence/phase2/public-navigation-scenario.json)、[结果](evidence/phase2/rea/public-navigation-scenario-result.json)，关联ID `ev_c006a0007ad8fdf57943162bb98bc9db0a5e7293f2e48c10448eebf3be878aad`。先前页面URL含入口key，已脱敏，结果文件属于衍生证据，不可当原始REA摘要验证。
- 后台场景：[输入](evidence/phase2/admin-navigation-scenario.json)、[结果](evidence/phase2/rea/admin-scenario-result.json)，ID `ev_701d4ef3f7e83ed5dce88495600d02cd043901963c40c1f1d7c853746dce4db0`；使用环境变量secret引用，REA自动脱敏key，未将其值写入场景文件。

**OBSERVED**：受控场景动作仅包含公开页面goto与短暂等待；没有点击提交/验证码、填写字段或操作业务。REA负责网络记录，不能称这些导航由被动观察器自动执行。结果浏览器ownership为external，完成后只断开连接，没有关闭用户Chrome。

## 每条记录的字段口径

**UNKNOWN**：事务Request/Response Content-Type头值、JS Initiator及响应JSON结构。为避免保留凭证/个人信息，本次关闭header_values/request_body/response_body；场景事件提供头名称但不提供头值和initiator，不根据扩展名编造。GET查询参数在记录中按字符串编码标识，服务端类型未知。

**OBSERVED（另一次资源快照）**：`inspect_web_page.resources`提供MIME类型，已另存为`resource_mime_snapshot`并附来源文件；不是把另一采集窗口的MIME冒充当前事务响应头。特别是后台`yzm.html`被识别为`image/png`，因此它是验证码图片，不是JSON业务API。[admin-capture](evidence/phase2/rea/admin-capture.json)。

以下按去重URL汇总，**每行均OBSERVED**；逐次记录及事务ID见JSON。表内MIME来自独立资源快照，缺失记UNKNOWN。请求/响应正文结构与JS Initiator的未知口径适用于所有行。

|URL|Method|Status|次数|资源类型|资源快照MIME|参数名|
|---|---|---|---|---|---|---|
|`https://demo.haotihuo.com/home/index/card_verify.html`|GET|200|1|document|UNKNOWN|无|
|`https://demo.haotihuo.com/static/bootstrap/css/bootstrap.min.css`|GET|200|3|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/home/css/font-awesome/font-awesome.min.css`|GET|200|4|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/home/css/iconfont/iconfont.css`|GET|200|3|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/home/css/v1.css?v=69`|GET|200|3|stylesheet|text/css|v|
|`https://demo.haotihuo.com/static/home/js/validate.js?v=88`|GET|200|3|script|application/javascript|v|
|`https://demo.haotihuo.com/uploads/image/202306/20230601174608_65681.png`|GET|200|3|image|image/png|无|
|`https://demo.haotihuo.com/static/home/weiui/jquery.min.js`|GET|200|3|script|application/javascript|无|
|`https://demo.haotihuo.com/static/bootstrap/js/bootstrap.min.js`|GET|200|3|script|application/javascript|无|
|`https://demo.haotihuo.com/static/layer/layer.js`|GET|200|3|script|application/javascript|无|
|`https://demo.haotihuo.com/uploads/image/202204/20220418003923_65494.jpg`|GET|200|3|image|image/jpeg|无|
|`https://demo.haotihuo.com/static/home/css/fonts/fontawesome-webfont.woff2?v=4.7.0`|GET|200|4|font|UNKNOWN|v|
|`https://demo.haotihuo.com/static/layer/theme/default/layer.css?v=3.1.1`|GET|200|3|stylesheet|text/css|v|
|`https://demo.haotihuo.com/home/index/card_exchange.html`|GET|200|1|document|UNKNOWN|无|
|`https://demo.haotihuo.com/home/index/order_query.html`|GET|200|1|document|UNKNOWN|无|
|`https://demo.haotihuo.com/admin/login?key=[REDACTED:entry_key]`|GET|200|1|document|UNKNOWN|key|
|`https://demo.haotihuo.com/static/home/assets/css/bootstrap.min.css`|GET|200|1|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/layuiadmin/layui/css/layui.css`|GET|200|1|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/admin/css/login.css`|GET|200|1|stylesheet|text/css|无|
|`https://demo.haotihuo.com/admin/login/yzm.html`|GET|200|1|image|image/png|无|
|`https://demo.haotihuo.com/static/admin/js/jquery.js`|GET|200|1|script|application/javascript|无|
|`https://demo.haotihuo.com/static/layuiadmin/layui/layui.js`|GET|200|1|script|application/javascript|无|
|`https://demo.haotihuo.com/static/admin/js/keyboard/keyboard.css`|GET|200|1|stylesheet|text/css|无|
|`https://demo.haotihuo.com/static/admin/js/keyboard/keyboard.js`|GET|200|1|script|application/javascript|无|

## API分类

**OBSERVED**：新增的动态生成资源是后台`GET /admin/login/yzm.html → 200`，资源快照MIME为image/png；没有识别/保存验证码图片内容。三个用户端文档和后台登录文档GET已观察。第一阶段公告N01仍为有效既有证据：[05](05-api-inventory.md)，本阶段不算重复新增。

**INFERRED / STATIC_CANDIDATE**：C01 check、C02 order_query_yzm、C03 feedback_send；F01 order、F02 order_info；V01 makecode、V02 validate_code；A01 login_check与A02找回密码iframe目标。完整构造与字段见[13](13-api-call-graph.md)。A04后台首页只是成功跳转候选，不当成已请求API。登录yzm图片GET不能证明登录成功。

## 被动观察的实际限制与失败记录

|尝试|实际结果|证据|
|---|---|---|
|加载完页面后inspect|捕获DOM结构、已加载脚本/资源；网络空，因为没有追溯此前活动|[verify](evidence/phase2/rea/verify-capture.json)、[order](evidence/phase2/rea/order-capture.json)|
|跨导航且请求脚本源|旧scriptId失效，No script for id，整次失败；不算网络证据|[失败输出](evidence/phase2/rea/exchange-navigation-capture.json)|
|observe_web_session|记录预约→查询的navigation_committed及lifecycle，未提供请求Method/Status|[会话](evidence/phase2/rea/order-navigation.json)|
|inspect跨URL/同步刷新|target_changed，未取得可用事务；另外一次观察窗口未覆盖刷新而为0请求|[跨URL](evidence/phase2/rea/admin-navigation-capture.json)、[刷新失败](evidence/phase2/rea/admin-network-observed.json)、[空窗口](evidence/phase2/rea/admin-reload-network.json)|
|capture_browser_scenario正常导航|成功取得39+11事务、共50GET；原始body和头值关闭|本文件上方两个scenario结果|

这些失败单独保留，没有通过重复宣称“已捕获”填补缺口。没有用伪造卡密、订单号或修改客户端验证标记触发POST。

**OBSERVED（审批记录）**：独立HTTP首次保存计划因URL含key被自动审批拒绝；改为保存脱敏URL后获准。后台HTTP副本仅作结构补证：[metadata](evidence/phase2/admin-http-metadata.json)、[HTML](evidence/phase2/admin-login.sanitized.html)，不混入50条浏览器事务。其他新增文件中入口key也已移除；变更清单见[redaction-ledger](evidence/phase2/redaction-ledger.json)。第一阶段文件保持原样。
