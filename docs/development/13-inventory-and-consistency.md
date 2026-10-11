# 13 库存与事务一致性

**DESIGN_DECISION**，非原站实测。定义on_hand为仓库账面实存，reserved为已接受未发货订单预占，available=on_hand-reserved；均>=0，reserved<=on_hand。入库累计与已用累计从账本计算展示，不能把on_hand称累计历史入库。所有库存变动必须产生inventory_transactions唯一event_key。

## 通用幂等协议

所有业务写请求Idempotency-Key16..64随机ASCII，同主体+operationId命名空间；请求规范化后HMAC摘要。事务首先插入/锁idempotency_keys唯一行；同键同hash已有成功result_ref即读取对象返回，版本/卡状态检查在重放之后；不同hash409。事务中间不单独提交IN_PROGRESS占位，失败全回滚；成功和result_ref同事务提交，保留24h（过期删除也不移除领域唯一约束）。不保存PII响应或原始token；重放可重新签发同对象scope会话。跨节点唯一约束和行锁保证串行，不能只用进程Mutex。

## 全局锁序

相同用例使用：幂等键→批次/卡券→订单（有则）→按DATE升序日历→按ID升序商品→按product_id升序库存→reservation/账本→审计/Outbox。审核类幂等→库存单→商品/库存升序；发货幂等→订单→售后（如需）→商品/库存；补发幂等→订单→售后→库存；配置更新先config→日历，兑换不反向请求config行锁（用事务开始前不可变配置快照，日历权威再次校验）。密码hash在事务前验证，锁卡后比较credential_version确保未被重置。任何变更不得反向取得已绕过的高级别锁。

MySQL READ COMMITTED，SELECT ... FOR UPDATE只用于事务中的精确PK/唯一键匹配；余额行在商品创建时初始化，日历行提前生成31天，缺失时以唯一ship_date插入后重读锁。避免依赖未存在行的间隙锁。[MySQL锁读官方说明](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html)。

## T-REDEEM：提交与核销

1. BeginTx；处理幂等成功重放；先依据不可变batch_id锁批次，再锁Voucher，核对credential_version、active/SEALED、有效期、未消费未删除。无需再次传secret。
2. 从冻结policy_snapshot检查规则版本、商品集合、恰选N、去重和quantity=1；锁日历并检查closed/capacity；锁产品/库存升序，确认启用/期限/地区和available。
3. 插入voucher_redemptions（voucher_id唯一）、orders（redemption_id唯一）、order_items快照；密文地址/HMAC在事务前准备，结果日志不留原文。
4. 每项执行条件UPDATE inventory_balances SET reserved=reserved+q,version=version+1 WHERE product_id=? AND on_hand-reserved>=q；必须RowsAffected=1，否则全回滚OUT_OF_STOCK。写ACTIVE reservation及reserve:{order}:{product}账本。
5. calendar.used_slots+=1条件used<capacity；Voucher.redeemed_at=now；写order.created Outbox、审计、幂等result_ref；commit。签发ORDER WRITE会话若单独失败不回滚已提交订单，同键重试可补发授权。

同一卡不同键并发：仅一单成功，其余409 ALREADY_REDEEMED。相同键同请求100次只得到同一订单。不向外部系统发网络请求、不在预览时占库存。

## 其他原子事务

|事务|前提/锁|更新及唯一事件|
|---|---|---|
|T-EDIT|订单PENDING/version/截止时间；旧新日期升序；新日期校验|地址密文/phone HMAC/date/备注更新；旧used-1新used+1，全部成功才commit；库存不变；同一天不改容量|
|T-CANCEL|订单PENDING/version；无LABEL RUNNING/READY/UNKNOWN|reservation ACTIVE→RELEASED，reserved-=q，release:{order}:{product}；calendar.used-1；订单CANCELLED，卡不恢复|
|T-SHIP|PENDING到预约日，version，原发唯一键，无开放售后；所有reservation ACTIVE|on_hand-=q,reserved-=q；reservation CONSUMED；ship:{order}:{product}；唯一shipment+SHIPPED+Outbox；calendar不减|
|T-PICKUP|卡密正确、未兑且启用封存、商品选择合法、available够；管理员权限|创建redemption+COMPLETED/PICKUP订单，calendar=NULL，无配送地址（加密空对象，phone_lookup为不可查询随机值）；on_hand-=q；pickup:{order}:{product}；无reservation|
|T-APPROVE|单据PENDING/version；余额行升序|入库+q；出库须available>=q才-q；document:{id}:{product}；审核人/时间同事务；重复返回原结果|
|T-REPLACE|订单已发/完成；REPLACEMENT售后APPROVED，未有唯一补发运单；available>=q|on_hand-=q（reserved不变）；replacement:{case}:{product}；一条补发shipment及Outbox；原单状态/卡不变|
|T-EVENT|验签/时效、唯一provider_event_id；订单→shipment锁|只追加未见事件；可信DELIVERED使原发订单SHIPPED→COMPLETED；补发DELIVERED关闭售后，不重复扣库存|

## 对账与故障

每日worker读取一致快照比较：每product reserved等于ACTIVE reservations之和；on_hand等于库存账on_hand_delta之和；reserved等于reserved_delta之和；每PENDING配送订单项有ACTIVE预占；SHIPPED/COMPLETED配送项CONSUMED，CANCELLED项RELEASED；每redemption恰一order且voucher消费时间存在。检测差异只告警和展示，不自动改余额。修复需停相关商品、审计纠错单或人工维护流程，不能直接UPDATE掩盖根因。

故障注入逐步在insert order、第二件预占、日历更新、Outbox写入及commit前断连接，验证所有表同回滚。commit结果不明用同键查询，不“补偿性取消”猜测结果。进程崩溃可由DB回滚；外部任务补偿见14。审计与交易强一致失败则业务回滚，不能无审计成功。

## 警戒值

DESIGN_DECISION：库存警戒值默认0；setInventoryWarning以product_id定位余额，inventory.write权限、version比较和审计同事务，仅更新warning_level。available<=warning_level显示低库存标记，不自动补货、不修改数量。T08补充：警戒值修改不生成数量流水，不改变on_hand/reserved，旧version409。
