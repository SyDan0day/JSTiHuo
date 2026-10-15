# F1 公开页面与响应式交付

日期：2026-10-15。依赖现有 A1/A2/B1；延续 A1–E2 实现，不改研究原件，不调用目标站业务接口。

## 实现范围

- 六个真实内容操作：publicContent、publicContentPage、listContent、getContent、createContent、updateContent。复用 catalog 事务、管理员权限、CSRF、幂等、审计与 HTML 白名单，数据保存在现有 content_pages。
- 后台 `/admin/content` 列表、搜索、状态筛选、游标翻页、完整快照编辑、发布与撤下。网络结果不确定时冻结表单，原正文原幂等键重试；409 要求重新读取核对。
- 首页独立于目录，接入真实商品、推荐和公告；推荐可手动前后切换。统一公开头部、底部、移动五入口、安全区预留、跳过导航链接及焦点样式。
- `/pages/:slug` 支持须知、关于、联系、公告等自有信息；商品列表与详情共享布局。详情及信息页跟随路由参数重读，并拒绝旧响应覆盖。
- 公开旧 `.html` 路径的客户端重定向；Vite `/media` 代理。没有复制原站版权素材或映射旧业务 ID。

主要文件：backend/internal/app/catalog/content*.go、backend/internal/transport/http/content.go、frontend/src/features/public、frontend/src/features/admin/content、PublicLayout.vue、ProductGrid.vue、styles/public.css、router.ts 和 vite.config.ts。OpenAPI 补齐内容筛选枚举，生成类型同步；数据库结构不变。

## 契约与边界

公开只读取已发布内容，草稿与不存在统一 404。slug 唯一，只允许 1–64 位小写字母、数字、连字符。标题 1–120 个码点，正文最多 50000 个码点，清洗后再次检查长度；排序 0–1000000。正文不保留链接、图片、属性、脚本、样式；没有内容硬删除接口。

所有列表按 ID 降序；后台状态为 PUBLISHED/DRAFT/空，公开不接受 DRAFT。首页只从最新 100 条公开内容中按展示顺序呈现推荐和至多四条公告；其余内容仍可通过 slug 访问。固定入口使用 guide/about/contact/notice。新安装默认为空，商家身份、联系方式和运营正文需管理员核实发布，未发布页面显示不可用。

## 已执行验证

- `GOCACHE=/private/tmp/jst-f1-go-cache make test`：Go race 单元、OpenAPI 校验/类型生成、37 表迁移一致性、Go 构建及 Vue strict 类型/生产构建通过。
- `GOCACHE=/private/tmp/jst-f1-go-cache go vet -tags browserfixture ./...`：通过，包含浏览器夹具。
- `TEST_DB_DSN=隔离MySQL go test -race -v ./tests/integration -run '^TestContent$' -count=1`：通过；内容发布、草稿隔离、分页、同键重放、错误筛选、完整快照、版本竞争、权限与 CSRF。
- `TestContentAuditRollback`：注入真实 MySQL 审计触发器错误，断言内容与幂等记录均回滚；通过。
- `TestContentConcurrentReplay`：八个协程同键创建，返回同一 ID，只有一条成功审计；通过。
- `TEST_DB_DSN=隔离MySQL go test -race -v ./tests/integration -count=1`：全量通过，用时 270.407 秒，包含原有发行、兑换、订单、库存与物流回归。
- `tests/browser/content.cjs` + `browserfixture`：在真实 Go/MySQL 上通过。覆盖后台创建草稿→公开 404→发布→公开读取→撤下→公开 404；注入提交后丢响应并验证相同幂等键恢复；清洗 XSS、推荐切换、部分接口失败及重试、真实下架空目录、路由别名、页内路由切换、匿名后台拒绝、键盘跳过导航、无横向溢出、移动页尾不被底栏遮挡。浏览器无 pageerror。

15 张实际截图位于 [evidence/f1](evidence/f1)：首页、目录、须知、详情各三种宽度，以及后台编辑桌面/移动和移动列表。示例：[390 首页](evidence/f1/home-390.png)、[1440 首页](evidence/f1/home-1440.png)、[移动编辑](evidence/f1/content-editor-390.png)。图片、商品与内容均由隔离夹具生成。

## 重复浏览器验收

需要本机 Chrome、Playwright、Node 24+、已安装前端依赖，以及允许 CREATE/DROP DATABASE 的隔离 MySQL 测试账号。browserfixture 不随默认测试启动，不连接业务库。会话文件仅写入权限为 0700 的临时目录，文件为 0600；不记录 cookie、CSRF 或密码到截图与日志。

1. 启动 `npm --prefix frontend run dev`，默认 5173。
2. 准备一个新临时目录，导出 `F1_BROWSER_DIR` 为该绝对路径；导出 `TEST_DB_DSN` 为隔离空库。
3. 在 backend 执行 `go test -tags browserfixture -v ./tests/integration -run '^TestContentBrowser$' -count=1 -timeout=15m`。夹具在 localhost:8080 提供同源页面/API，创建临时子库、自有内容和四个测试商品。
4. 日志出现 ready 后，在仓库根目录执行 `F1_BROWSER_DIR=同一临时目录 PLAYWRIGHT_MODULE=本机playwright模块路径 node tests/browser/content.cjs`。也可通过 NODE_PATH 提供标准 Playwright。默认截图覆盖本包 evidence/f1；设置 F1_BROWSER_OUTPUT 可改为临时输出目录。
5. 浏览器测试会撤下夹具商品以验证空态，因此重复运行应重启夹具。完成后创建 `$F1_BROWSER_DIR/stop`，等待 Go 用例退出，再停止 Vite 并清理临时目录。夹具会删除会话文件和隔离子库。

## 未包含

F2 用户业务表单与反馈、G1–G3、H1/H2 全局验收仍待实施。此次验证只覆盖 F1 路由及其公开入口，不代表 14 步完整交易浏览器验收、真机软键盘、真实供应商、生产发布或恢复演练通过。新建商家运营内容尚需运营发布；本轮没有自动创建正式商家资料。

本轮未创建 Git commit，未 push。临时数据库、浏览器会话与验收服务在完成后清理，不作为运行环境交付。
