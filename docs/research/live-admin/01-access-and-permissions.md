# 01 访问权限与统一调查基线

调查日期：2026-10-11；范围为当前账号的真实后台只读调查及后续规格整合。沿用现有Chrome专用CDP会话，用户已完成登录；没有要求、输入或保留账号密码、Cookie、Authorization、会话Token。没有重新登录，没有取得服务器源码。

## 证据分类

|标签|本轮含义|
|---|---|
|LIVE_OBSERVED|真实会话中的页面、控件状态或网络请求；页面声称的规则仅证明文案存在|
|SOURCE_OBSERVED|真实页面已加载脚本中的代码；STATIC_CANDIDATE表示尚未调用的接口|
|PROTOTYPE_OBSERVED|52页单文件原型内容；不证明真实系统行为|
|INFERRED|从证据解释关系或评估覆盖；不代表端到端验证|
|DESIGN_DECISION|Go/Vue新系统自主设计；不能反写为原站内部模型|

UNKNOWN是“未确认”状态，WRITE_TEST_REQUIRED是验证条件，不是新的事实来源。每项重要结论附页面、操作和证据。

## 已有成果基线

**INFERRED（文档审核）**：已复核01–20现有调查文档、原始证据清单和新增原型。26个用户端URL对应19内容组、63张截图；一期三个Ajax和两个POST表单仍是静态候选。二期REA已定位拼图→预检→后续POST链，50次GET没有业务POST。历史审核的8项澄清仍有效，不覆盖原文。[一期审核](../11-first-stage-audit.md)、[二期结论](../20-development-readiness.md)

**INFERRED（模型基线）**：旧18中的SKU、Entitlement、Redemption、库存事务、订单状态等均为自主模型候选。现有证据没有赋予其“原站数据库”身份。本轮Product字段、card_type和后台状态筛选能收窄部分未知；并发、库存扣减和端到端兑换仍不能由页面推断。[旧模型](../18-business-contract.md)、[本轮模型](10-domain-model.md)

**PROTOTYPE_OBSERVED**：原型52模板完整存在，UI使用PAGES模板注入、Toast和非空模拟登录；没有将其模拟成功当作真实请求。原文件未改动。[模板索引](evidence/prototype-pages.json)、[52项矩阵](03-prototype-vs-live.md)

**INFERRED（本地完整性）**：本轮开始建立已有文件SHA-256基线，结束再次核对；结果见[交付检查](evidence/delivery-check.json)，清单见[原文件基线](evidence/prior-files.json)。没有重复用户端截图或重新执行已有空输入测试。

## 会话和权限结论

**LIVE_OBSERVED**：当前已登录外壳URL为`/admin/index/index.html`，加载`welcome.html`内容iframe，菜单含52个去重目标，与52模板名称全部可映射。[初始DOM](evidence/initial-dom.json)。实际打开33个内容路径，30个可读、3个权限受限；外壳另计1页。[清单](02-live-page-inventory.md)

**LIVE_OBSERVED**：管理员列表`/admin/auth/admin.html`、订单日志`/admin/order/log.html`、系统日志`/admin/config/log.html`显示“当前测试用户操作权限受限”，HTTP仍为200。没有尝试其他账号、路径变体或绕过。[管理员截图](evidence/admins-masked.png)、[订单日志截图](evidence/order-log-masked.png)、[系统日志DOM](evidence/system-log-dom.json)

**LIVE_OBSERVED**：角色列表和一个角色编辑表单可打开，包含`rule[]`细分操作权限。**INFERRED / UNKNOWN**：不能据此确认当前账号角色名称、全部权限、修改权限生效或服务端每个端点的授权完整性；未读取或保存角色绑定值，也没有更改复选框。[角色表单](evidence/role-edit-dom.json)

## 实际操作边界和工具

**LIVE_OBSERVED（工具操作）**：Chrome扩展列出连接但会话选择超时；转用已获授权的`127.0.0.1:9222` CDP。仅对目标域的现有标签读取DOM/正常加载脚本、观察Network，或根据已观察链接导航。没有扫描路径或独立携带凭证请求目标服务器。CDP已足够，本轮未重复REA分析。[采集辅助程序](evidence/inspect-live.mjs)

**LIVE_OBSERVED**：除普通导航外，只执行商品启用状态筛选、一次成功卡券下一页、多选多radio切换。打开新增/编辑/人工发货表单后未保存；没有生成、激活、停用、核销、发货、删除、导出、通知、同步旺店通、改权限或清缓存。实际1087个请求记录全为GET，0 POST；GET本身也不能当作无副作用保证，已知写入GET候选未调用。[网络清单](08-live-api-inventory.md)
