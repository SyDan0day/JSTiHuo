# 01 统一调查基线

## 当前项目审阅

2026-10-11本地检查：backend/frontend/deployments/tests为空目录；research仅空分类目录；业务材料位于docs/research。没有go.mod/package.json/Compose或现存业务代码，git status报告“not a git repository”。因此本轮不声称有可运行项目、迁移或测试通过。实际读取资料为研究01–21、live-admin 01–14、write-tests计划/结果；52模板逐个提取表单标签，检查真实源码和网络JSON；查看鉴真移动截图、真实多选多截图、商品编辑截图。完整清单及hash见[审阅清单](reviewed-sources.json)。

## 证据等级及时间

LIVE_OBSERVED=真实浏览器观察；NETWORK_OBSERVED=真实请求/响应，是LIVE的网络子类；SOURCE_OBSERVED=捕获的实际加载代码；PROTOTYPE_OBSERVED=原型；INFERRED=解释或独立下载源码分析；DESIGN_DECISION=新系统。旧报告OBSERVED按具体来源细分，不改变历史文件。未知原站事实仍未知，但新系统用ADR给出确定规则。

|证据ID|来源|可确认内容|不能确认|
|---|---|---|---|
|E01|[用户端逐页](../research/02-page-details.md)、[组件](../research/03-ui-components.md)|LIVE：26URL/19内容组，63截图，入口、菜单、公开商品、空输入提示|有效卡后的选品、订单成功|
|E02|[前台调用图](../research/13-api-call-graph.md)、[捕获JS](../research/evidence/phase2/focused-provenance.json)|SOURCE：鉴真/预约check；查询预检；两个原生POST跳页；拼图回调|成功响应、CSRF语义、原站服务器算法|
|E03|[一期网络](../research/05-api-inventory.md)、[二期网络](../research/14-network-evidence.md)|NETWORK：公告GET200、二期50GET；无前台业务POST|不能说原站没有API|
|E04|[后台访问](../research/live-admin/02-live-page-inventory.md)|LIVE：33内容路径30可读3拒绝，外壳另计|菜单可见不证明保存权限|
|E05|[商品](../research/live-admin/04-product-workflow.md)|LIVE/SOURCE：编码、属性、媒体、有效期、区域、外部规格、状态|SKU/编码唯一约束|
|E06|[卡券](../research/live-admin/05-voucher-workflow.md)|LIVE：多选一/多选多显隐；SOURCE：ID集合、生成和加密候选|数量口径、加密算法、核销时点|
|E07|[订单](../research/live-admin/06-order-workflow.md)|LIVE：待发0/已发1/完成2筛选；详情/编辑字段；售后/回收站|完整状态迁移、拆单|
|E08|[库存物流](../research/live-admin/07-inventory-and-shipping.md)|LIVE：库存、审核、单号/公司、人工发货表单|预占/扣减时点、供应商副作用|
|E09|[后台网络](../research/live-admin/08-live-api-inventory.md)|NETWORK：1087GET窗口事件，getcount15个number字段，17状态缺失|不是1087个API；57源码字面量含已实测读端点|
|E10|[原型对照](../research/live-admin/03-prototype-vs-live.md)|PROTOTYPE：52模板，全部有同名真实菜单，24可访问3拒绝25仅菜单|模拟登录/Toast不证明成功|
|E11|[最新写测](../research/write-tests/2026-10-11/02-results.md)|NETWORK：一次add_check POST200/code0；LIVE：权限提示及0匹配|不是新增成功，未生成卡/订单|
|E12|[旧设计](../research/live-admin/10-domain-model.md)|DESIGN_DECISION：领域候选及事务建议|不是原站表结构或最终契约|

最新事实采用E11>E04–E09>前期阶段结论。原“未登录/0POST”仅适用于当时。已获写测授权但账号新增受限，不阻塞本次自有系统规格。不访问目标站、不新增真实业务数据。

## 关键字段映射

|原站字段/标签|新系统API/数据库|来源/调整|
|---|---|---|
|pro_name/spbm/pro_img/pro_content|name/code/media_ids/description_html；products/product_media|E05；表名/唯一约束DESIGN|
|spec_no/spec_name|external_spec_code/external_spec_name|E05；不冒充SKU|
|s_time/e_time/no_area/status|redeem_from/redeem_until/excluded_region_codes/status|E05；状态改稳定字符串|
|pici/bname|batch_no/label|E06；内部ID独立|
|card_type1/2/product_num|ONE_OF/N_OF/selection_count|E06；恰选N种各一件为ADR01|
|product/productlist|product_ids / voucher_products|E06；去重、快照、FK为DESIGN|
|code_num/code_pre/pass_len/pass_type|quantity/code_prefix/secret_length/secret_format|E06；安全生成/导出为ADR09|
|bianma/pass|code/secret|E02；不复制email类型，secret不回显|
|status激活、sale提货|active/redeemed_at|E06；加密与会话另分维度|
|订单status0/1/2|PENDING/SHIPPED/COMPLETED|E07；CANCELLED为自主补足|
|xm/phone/address/order_date|address对象/ship_date；recipient_cipher/calendar_id|E07；加密存储设计|
|t_no/t_trace|tracking_no/carrier_id|E08|
|累计/已用/可用|on_hand/reserved/available及账本|E08只有概念，公式按ADR04，不等同原站原公式|

## 明确冲突裁决

真实卡型多选一/多选多覆盖原型普通/电子卡；原型到店额外验证码不作原站事实；后台真实旺店通字段保留外部映射但不实现未经定义的集成。前台手机号旧正则/反馈200与210冲突改统一服务端规则；具体见05。前台目录与卡券授权选品分路由，公开商品可见不等于可兑换。

原站加密文案在现有截图中明确提示生成并导出后立即加密；新系统保留生成→短期导出→封存（界面别名加密）→可提货操作顺序，但不永久存明文、不模仿未知算法。封存不等于激活，见ADR09。
