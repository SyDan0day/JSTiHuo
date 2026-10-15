# F2 用户业务界面与反馈

2026-10-15 实施。公开反馈使用一次性 FEEDBACK 挑战；姓名、手机、邮箱和参考订单号以独立 PII 密钥环加密。参考订单号不会授予访问权限。处理接口比较版本，审计与幂等记录不缓存反馈正文或联系人。

用户交易页面统一公开布局、错误恢复与限流等待提示；地区联动拒绝过期父级响应。提交期间阻止改选与双击。反馈挑战不能重放，网络结果不确定时冻结表单并提示查询处理，不自动换挑战重复提交。

验证：`make test` 通过；真实 MySQL `TestFeedback` 通过。与 G1 隐私修正一并运行的 race 回归为 36.284 秒（TestFeedback、TestFileJobs、TestPrivateFileBoundaries）。`tests/browser/feedback.cjs` 在真实 API 上通过，保存 11 张截图于 `evidence/f2`；原卡密复核后续复用于售后申请。

完整兑换、查询、弱网和页面三档验收已随 H1 完成，见 H1-delivery.md；反馈专项与主链结果分别记录。未发送真实通知。

## 2026-10-15 本地前后端联调

启动 Compose 的 MySQL、API、worker 和 nginx 前端，入口为 `http://localhost:8080`。
本次新建本地库使用 4 个合成商品、已审核库存及 6 张激活测试卡，未使用真实顾客数据。
本机测试管理员及卡券凭证保存于忽略提交的 `deployments/secrets/local-test-data.json`，文件权限为 0600。

浏览器完成真实商品与图片加载、图形挑战、卡券鉴真、4 选 2、地区联动、预约及确认下单。
订单 `O_01M4Z3N9T3D8V5NH70SNQ089TR` 为待发货，刷新页面后可读取脱敏结果。
双击提交后只创建 1 张订单并消费 1 张卡，两个商品分别预占 1 件，预约名额占用 1 个。

本轮修复：

- `useChallenge.ts`、`Verify.vue`、`Query.vue`：换图期间禁用刷新和提交，立即清空旧挑战与答案；失败保留重试入口，恢复成功清除加载错误，自动换图保留业务失败提示。
- `AddressFields.vue`、`address-region.ts`、兑换 store、`Address.vue`、`Confirm.vue`：确认页展示完整省市区与详细地址；地区名称与 API 地址字段分离，仅存内存，切换地区时清除过期名称，流程结束同时清理。
- `public.css`：修复 flex 公共布局中页头宽度收缩。

验证结果：

- `cd frontend && node --test tests/public-flow.test.mjs`：7 项通过，覆盖挑战失败恢复、重复刷新、业务错误保留、地区一致性、状态清理和确认页渲染。
- `cd frontend && npm run build`：类型检查及生产构建通过。
- 设置本地测试 `TEST_DB_DSN` 后执行 `go -C backend test -race ./tests/integration -run '^(TestCatalog|TestContent|TestRedemption|TestMockFulfillment|TestConfigurationRace|TestInventoryReconciliation)$' -count=1 -v`：6 项通过，64.013 秒。测试使用独立临时数据库。
- 修复后的鉴真页和查询页均通过浏览器断网恢复验证：失败时无旧图、提交禁用，恢复后错误清除；订单结果页在桌面与 390px 断点无水平溢出。
- `git diff --check` 通过。前端容器已重新构建并启动。

真实承运商调用和实际发货不在本次商品加载、鉴真、下单联调范围内；测试订单保留待发货。

## 2026-10-15 查询、改址与模拟发货续验

使用本地合成订单，通过运行中的 nginx/API 验证订单号查询、READ 会话写入拒绝、原卡密复核升级 WRITE、完整地址替换及改期。原手机号查询被拒绝，新手机号可按卡号查询脱敏结果，旧版本修改返回冲突。

本地订单 `O_01M4Z3N9T3D8V5NH70SNQ089TR` 更新至西城区合成地址，预约日期为 2026-10-17；当前测试手机号见本地凭证文件的 `orders` 字段。数据库核对原日名额为 0、新日名额为 1，两件商品仍各预占 1 件，订单保持 PENDING。创建的 `LOCALMOCK` 承运商 provider 为 MOCK；提前发货返回 409，未创建运单。

实际发货事务在独立测试库的固定时钟下验证：MOCK 全单发货、库存实存与预占双减、模拟轨迹及签收完成。未修改运行中服务的业务时钟，未调用真实物流。

修复文件：`frontend/src/features/orders/Edit.vue` 在订单和日历完整加载前禁用提交，避免读取失败后沿用旧选项；`frontend/src/features/admin/shipping/Ship.vue` 增加刷新权威版本与承运商入口，刷新前清空目录与旧错误，读取失败时禁止提交，并将发货失败转换为中文提示。结果未知时仍冻结表单，重试保留原正文和幂等键。

验证：

- `cd frontend && node --test tests/order-flows.test.mjs tests/public-flow.test.mjs`：11 项通过，其中 4 项新增回归使用真实 Vue 页面、内存渲染器和模拟 HTTP 响应验证查询表单、版本冲突恢复、日历失败恢复与发货同键重试。替换回修复前代码时能复现失败，随后已恢复修复版。
- 设置隔离测试 `TEST_DB_DSN` 后执行 `go -C backend test -race ./tests/integration -run '^(TestRedemption|TestShippingTransactions|TestMockFulfillment)$' -count=1 -v`：3 项通过，49.380 秒。
- 临时 `localfollowup` 标签测试通过真实本地 HTTP 完成上述查询与改址验证，测试脚本及临时 DSN 已清理。固定时钟发货测试与本地运行订单分离，本轮未执行完整浏览器点击验收。
- `cd frontend && npm run build`、`git diff --check` 通过；修复版前端容器已重建启动，`/readyz` 返回 `ok`。
