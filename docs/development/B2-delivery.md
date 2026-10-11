# B2 库存单据与审核（2026-10-12）

已实现 inventory / inventoryLedger / stockDocuments / createStockDocument / approveStockDocument / rejectStockDocument / setInventoryWarning。后台入口 /admin/inventory 与 /admin/inventory/documents：余额、可用量、低库存筛选、商品流水、警戒值版本编辑、入出库单创建、逐项数量、可选图片凭证 ID、状态筛选、审核原因及批准/拒绝。商品列表增加商品 ID 列供建单引用。PHYSICAL/VIRTUAL 是单据分类，共用规格定义的单商品余额，不新增仓库或独立库存池。

事务使用 READ COMMITTED。首先插入或锁定未提交的幂等占位行，随后重查管理员状态、凭证版本及权限，再锁单据、全部商品升序、全部余额升序。批准前检查所有商品；出库只能消耗 on_hand-reserved，库存不足422 OUT_OF_STOCK；单项失败整单回滚。入库实际库存上限1000000，契合响应 Schema。唯一 document:{id}:{product} 流水、余额、审核人/时间、版本、审计与幂等引用同事务提交。单据号为 I_ 加密码学随机 ULID，唯一冲突最多重试3次。

同主体/操作/键同正文重放原对象 ID 的当前投影，不重复业务效果；正文或目标 ID 变更409 IDEMPOTENCY_CONFLICT。成功记录保存24小时，失败不留下占位行。不同键竞争已审核单据返回409版本或状态冲突。警戒值仅修改余额警戒值与版本，拒绝仅修改单据；两者均不写数量流水。前端遇到网络异常或5xx保留同键同正文供用户手动重试，未知结果时锁定编辑。

验证：Go race、OpenAPI 校验及类型生成、37表迁移一致性、前后端构建通过。MySQL8.4隔离库集成测试覆盖入库/出库、保护reserved、多商品第二项不足全回滚、两个出库单抢最后可用量、同单不同键并发批准、8个同键并发重放、重复正文冲突、拒绝无流水、警戒值过期版本、权限撤销、CSRF、账本余额一致。通过触发器让审计 INSERT 失败，证明余额、流水、审核状态和幂等占位全部回滚。测试子库自动删除，不向业务库添加夹具。

后台浏览器完整点击验收尚未执行；当前验证为API真实事务、前端类型检查及构建。库存对账属于H1，卡券兑换预占属于D2，未显示这些未实现操作。下一工作包C1卡券批次。

部署：Docker API/proxy重建成功，MySQL/API/proxy均healthy；healthz/readyz为200，两个后台页面路由为200，三个库存读取接口未登录为401。临时13306测试端口已关闭，正常服务仅localhost:8080。
