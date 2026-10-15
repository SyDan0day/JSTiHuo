# H1 性能验收

2026-10-15，本地真实 MySQL 8.4。测试入口 `backend/tests/integration/performance_test.go`，需显式 `-tags performance`，单独隔离库。数据为自有合成资料，不访问生产。

## 环境与范围

宿主机 Apple M5 Pro、15 核、24 GiB；Go 1.26.5 darwin/arm64。应用测试进程设置 GOMAXPROCS=2、GOMEMLIMIT=4GiB；独立 MySQL 容器限制 2 CPU/4 GiB（Docker VM 总配额大于单容器，不把宿主机全部资源算作应用配额）。应用使用真实事务与加密；GOMEMLIMIT 为 Go 运行时软目标，不是 OS 硬隔离。

建立 10 万条 CANCELLED 历史订单，每单有绑定业务号 AAD 的真实 PII 密文、冻结商品、已释放预占、预占/释放双向流水与 Outbox；100 个历史批次、10 万张卡。另建 100 个真实有效 WRITE 会话并发兑换；读样本为后台取消订单游标列表，每页 20 条。全部样本均计入，没有剔除失败请求。

普通读经真实 HTTP Handler 和权限读取；兑换测真实业务事务，不含验证码、Argon2 鉴真、TLS 和外部系统。门槛为规范 06 的读 P95 <300ms、兑换 P95 <1s，不把本测试等同于公网全链耗时。

## 实测结果

|样本|n|P50|P95|最大值|门槛|
|---|---:|---:|---:|---:|---|
|普通列表读|100|179.571333ms|213.411291ms|222.221916ms|通过|
|兑换事务|100|615.763209ms|812.488958ms|855.870417ms|通过|

测试耗时 15.15s，进程墙钟 15.55s，最大 RSS 220,758,016 字节（约 210.5 MiB），peak footprint 209,699,560 字节。执行后全局余额、活动预占与流水一致，无负库存、重复消费或重复主运单。

首次未优化测量读 P95 326.551ms、兑换 1.4697s，未达标。定位后将订单商品读取改为同一只读快照内批读，消除每单查询；兑换使用批次共享锁、仅对目标卡加排他锁，避免 JOIN 导致同批不同卡串行。没有取消事务、授权、审计或唯一约束，也没有新建业务索引。并发/配置/发货受影响回归随后通过。

EXPLAIN：订单列表使用 PRIMARY 倒序范围扫描与 LIMIT，无 filesort 或无界 OFFSET；日历按日期唯一索引为 const，余额按 product_id 唯一键为 const。只读库存对账在同一一致性快照中比较流水和活动预占。

## 复现

在受控测试终端设置 TEST_DB_DSN，必须使用允许创建/删除临时子库的隔离 MySQL 账号。停止其他压测，记录 MySQL 配额与主机资源后运行：

```sh
cd backend
GOMAXPROCS=2 GOMEMLIMIT=4GiB go test -tags performance -v ./tests/integration -run '^TestPerformanceAcceptance$' -count=1
```

不要同时启用 CONTRACT_VALIDATE；其同步 Schema 观察器用于正确性验收，不用于业务延迟测量。报告不代表大于此样本的容量、真实供应商或公网 SLA；部署前按实际数据量和硬件复测。
