# 14 证据索引、方法和完整性

调查日期2026-10-11。所有新增材料在本目录，01–20旧报告、原始截图及单文件原型保留。完整文件哈希见[manifest](evidence/manifest.json)，历史文件复核及本地链接检查见[delivery-check](evidence/delivery-check.json)。

## 证据类型

|文件/组|来源与限制|
|---|---|
|[prior-files.json](evidence/prior-files.json)|本轮开始前旧研究和原型的路径/哈希，不含原文件内容副本|
|[prototype-pages.json](evidence/prototype-pages.json)|PROTOTYPE_OBSERVED：52个模板名称和原文件起始行；不复制演示凭证/数据|
|[prototype-live-matrix.json](evidence/prototype-live-matrix.json)|原型标题与真实菜单、实际访问的映射；结论在03|
|[initial-dom.json](evidence/initial-dom.json)|LIVE_OBSERVED：已登录外壳菜单、welcome结构；账号显示名已移除|
|`*-dom.json`|LIVE_OBSERVED：当前DOM标签、字段名/类型、枚举、列标题、控件、脚本来源；不保留input值/表格行数据。不能替代完整HTML|
|`*-masked.png` 共49张|真实CDP视口截图，灰块覆盖账号区域、输入值、图片/二维码、数据行和富文本；未修改服务器或网页数据。部分详情/设置页面大量遮盖，结构以DOM佐证|
|`*-network.json`|LIVE_OBSERVED：各捕获窗口正常页面导航/交互请求；仅保留白名单Content-Type、方法、状态、脱敏URL及Initiator。没有HAR/Cookie/Authorization|
|[network-index.json](evidence/network-index.json)|1087个请求事件汇总，含捕获来源/证据ID、参数类型及JSON shape；不保留响应字段值|
|`*-source-excerpts.json`|SOURCE_OBSERVED：实际加载内联script中选定代码行，inline index和内部行号可定位；不是完整函数覆盖|
|[批量完整内联](evidence/voucher-batch-deep-full-inline.json)、[选择器](evidence/product-selector-full-inline.json)、[添加商品](evidence/product-add-deep-full-inline.json)|SOURCE_OBSERVED：三个不含实例业务数据的表单/选择器脚本；源代码未执行。此前正常radio点击不属于执行导出的脚本|
|[static-candidates.json](evidence/static-candidates.json)|57个不同Method/URL字面量候选（逐页面调用位置更多）；未调用不能证明成功|
|[coverage-summary.json](evidence/coverage-summary.json)|本轮页面/权限/请求/截图计数；不是全系统覆盖率|
|[inspect-live.mjs](evidence/inspect-live.mjs)、[mask-screenshot.py](evidence/mask-screenshot.py)|研究采集辅助程序，不是新系统业务实现；当前保存版本包含调查中逐步修复，不表示早期所有采集都使用同一版本|

## 关键操作索引

|操作|证据ID/来源|
|---|---|
|真实52项菜单|initial-dom；[菜单](evidence/initial-dom.json)|
|商品列表、编辑、筛选|product-list-deep / product-edit / product-filter；[筛选请求](evidence/product-filter-network.json)|
|多选多显隐前后|voucher-batch-deep / voucher-multi；[后态截图](evidence/voucher-multi-masked.png)|
|商品选择器集合回填|SOURCE_OBSERVED product-selector-full-inline；[源](evidence/product-selector-full-inline.json)|
|卡券实际下一页|voucher-next-observed；[请求](evidence/voucher-next-observed-network.json)、[DOM](evidence/voucher-next-observed-dom.json)|
|订单详情/编辑/人工发货表单|order-detail-fields / order-edit / shipping-manual-form；[发货字段](evidence/shipping-manual-form-dom.json)|
|库存与入出库|inventory / inbound-add / outbound-add / inbound-list / outbound-list；[库存](evidence/inventory-dom.json)|
|角色细粒度权限|role-edit；[DOM](evidence/role-edit-dom.json)|
|3处访问拒绝|admins / order-log / system-log；[拒绝截图](evidence/admins-masked.png)|
|真实统计JSON|welcome-deep-N17等；[事务](evidence/welcome-deep-network.json)|

## 失败尝试、脱敏与范围限制

**LIVE_OBSERVED（工具记录）**：Chrome扩展选择超时；本地CDP初次受文件/网络沙箱限制，按已授权范围获得提升后连接成功。没有修改Chrome安全设置。截图掩码初次遇到视口外矩形错误，修复后保存；失败时未写原始截图。

**LIVE_OBSERVED（工具记录）**：`voucher-page2`是一次未找到旧选择器的分页尝试，网络为空、地址未变，不作成功证据；检查实际控件`data-page`后，`voucher-next-observed`才是真实成功。早期无href的元素被辅助程序赋予当前页fallback URL，该值不是实际导航目标。保留此限制，导航关系用真实菜单dataHref、实际源代码和Network核对。

**INFERRED（脱敏审查）**：初版DOM收集遗漏过账号欢迎文案、表格链接中的卡号及设置提示中的入口key；发现后立即移除这些内容，修正白名单与URL脱敏，并将设置截图数据区域整体遮盖。最终交付只保留脱敏版本；不能声称最初采集器从未产生过此类值。没有读取Cookie/存储凭证，也没有把原始客户响应或截图另存为备份。

**INFERRED（截图限制）**：为避免个人资料泄露，对表格tbody整体覆盖；详情画面可能几乎全灰，此类截图只能佐证访问布局，不能用作原始订单数据证据。当前视口没有滚动覆盖全部长表单；DOM可包含屏幕外字段，字段存在不等于当前可见。早期DOM缺visible属性时不得推断显隐状态。

**INFERRED（网络限制）**：采集窗口不是持续全程HAR。响应结构只记录类型，未留真实字段值；数组仅取前3项的类型结构，不能证明完整可选联合类型。17个请求状态未捕获，未补造。成功页面GET与静态写入候选分开；不会把HTTP200权限提示记为业务可访问。

**INFERRED（范围限制）**：52模板中的25项仍为菜单级观察；本轮主攻C1–C5核心只读入口，非全后台完成。没有进行写入、越权、压力测试、验证码识别、服务器源码读取或真实客户数据导出。
