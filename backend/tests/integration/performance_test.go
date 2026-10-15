//go:build performance

package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/voucher"
	crypt "jstihuo/internal/infra/crypto"
)

// TestPerformanceAcceptance 用途：在十万历史订单与一百并发下验证读写延迟目标。
// 参数：t 测试上下文；必须显式启用 performance 标签和隔离 MySQL。
// 返回值：无，记录样本、资源与 P95；超出读 300ms、兑换 1s 目标立即失败。
func TestPerformanceAcceptance(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	ctx := context.Background()
	seedPerformanceHistory(t, f, 100000)
	t.Logf("样本：100000 CANCELLED 历史单；并发 100；GOMAXPROCS=%d；Go=%s；平台=%s/%s", runtime.GOMAXPROCS(0), runtime.Version(), runtime.GOOS, runtime.GOARCH)
	readTimes := parallelTimings(t, 100, func(i int) error {
		r := httptest.NewRequest("GET", "/api/v1/admin/orders?status=CANCELLED&limit=20", nil)
		r.AddCookie(f.client.cookies["jst-admin"])
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != 200 {
			return fmt.Errorf("列表状态 %d", w.Code)
		}
		var page order.Page
		if json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Items) != 20 {
			return fmt.Errorf("列表数据不完整")
		}
		return nil
	})
	t.Logf("普通读：n=%d P50=%s P95=%s max=%s", len(readTimes), readTimes[49], readTimes[94], readTimes[99])
	actors := performanceActors(t, f, 100)
	input := f.input()
	input.Items = input.Items[:1]
	writeTimes := parallelTimings(t, 100, func(i int) error {
		_, err := f.orders.Redemption.Redeem(ctx, actors[i], input, fmt.Sprintf("performance-redeem-%04d", i), "performance")
		return err
	})
	t.Logf("兑换：n=%d P50=%s P95=%s max=%s", len(writeTimes), writeTimes[49], writeTimes[94], writeTimes[99])
	result, err := f.operations.Reconcile(ctx)
	if err != nil || !result.Balanced {
		t.Fatal("性能执行后账本漂移", err)
	}
	explainPerformanceQueries(t, f)
	if readTimes[94] >= 300*time.Millisecond {
		t.Errorf("普通读 P95 未达到 300ms 目标：%s", readTimes[94])
	}
	if writeTimes[94] >= time.Second {
		t.Errorf("兑换 P95 未达到 1s 目标：%s", writeTimes[94])
	}
}

// parallelTimings 用途：同时释放有界并发请求并记录完整调用延迟。
// 参数：t 测试上下文；count 并发数；fn 单次真实业务调用。
// 返回值：升序延迟集合，业务失败终止验收，不从统计剔除失败样本。
func parallelTimings(t *testing.T, count int, fn func(int) error) []time.Duration {
	t.Helper()
	start := make(chan struct{})
	times := make([]time.Duration, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			begin := time.Now()
			err := fn(index)
			times[index] = time.Since(begin)
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal("性能请求失败", err)
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times
}

// seedPerformanceHistory 用途：在独立夹具内批量建立有密文与完整取消账的历史规模。
// 参数：t 测试上下文；f 隔离夹具；count 十万样本数。
// 返回值：无，禁止连接业务库；相同合成密码仅用于离线性能数据，不作为生产发行逻辑。
func seedPerformanceHistory(t *testing.T, f *fulfillmentFixture, count int) {
	t.Helper()
	ctx := context.Background()
	conn, err := f.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	typ, err := f.vouchers.CreateType(ctx, f.admin, voucher.TypeInput{Name: "历史性能样本", Mode: "ONE_OF", Count: 1}, testRequestKey(t), "performance")
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := json.Marshal(voucher.Policy{TypeID: typ.ID, Revision: 1, Mode: "ONE_OF", Count: 1, Products: []string{f.products[0]}})
	digits := "(SELECT 0 n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9)"
	if _, err = conn.ExecContext(ctx, "CREATE TEMPORARY TABLE performance_numbers(n INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "INSERT INTO performance_numbers SELECT a.n+10*b.n+100*c.n+1000*d.n+10000*e.n FROM "+digits+" a CROSS JOIN "+digits+" b CROSS JOIN "+digits+" c CROSS JOIN "+digits+" d CROSS JOIN "+digits+" e"); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, `INSERT INTO voucher_batches(id,batch_no,type_id,label,policy_snapshot,quantity,code_prefix,valid_from,valid_until,state,created_by)
		SELECT 1000+n,CONCAT('PERF-B-',n),?,'性能历史',?,1000,'PF',?,?,'SEALED',? FROM performance_numbers WHERE n<?`, typ.ID, policy, f.now.Add(-time.Hour), f.now.Add(365*24*time.Hour), f.admin.AdminID, count/1000); err != nil {
		t.Fatal(err)
	}
	hash, err := crypt.Hash("Synthetic-Performance-Only-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, `INSERT INTO vouchers(id,batch_id,code,secret_hash,active,credential_state,valid_from,valid_until,redeemed_at)
		SELECT 10000+n,1000+FLOOR(n/1000),CONCAT('PF',LPAD(n,14,'0')),?,1,'SEALED',?,?,? FROM performance_numbers WHERE n<?`, hash, f.now.Add(-time.Hour), f.now.Add(365*24*time.Hour), f.now, count); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "INSERT INTO voucher_products(voucher_id,product_id) SELECT 10000+n,? FROM performance_numbers WHERE n<?", f.products[0], count); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "INSERT INTO voucher_redemptions(id,voucher_id,policy_snapshot,redeemed_at) SELECT 10000+n,10000+n,?,? FROM performance_numbers WHERE n<?", policy, f.now, count); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(ctx, "INSERT INTO shipping_calendars(ship_date,capacity) VALUES('2026-10-13',100)"); err != nil {
		t.Fatal(err)
	}
	var calendarID string
	if err = conn.QueryRowContext(ctx, "SELECT id FROM shipping_calendars WHERE ship_date='2026-10-13'").Scan(&calendarID); err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(redemption.RecipientData{Address: f.input().Address, Region: "北京市 / 市辖区 / 东城区"})
	defer clear(plain)
	mac := hmac.New(sha256.New, f.orders.Redemption.LookupKey)
	mac.Write([]byte(f.input().Address.Phone))
	phone := mac.Sum(nil)
	for offset := 0; offset < count; offset += 1000 {
		seedPerformanceOrders(t, conn, f, offset, 1000, calendarID, plain, phone)
	}
	for _, statement := range []string{
		"INSERT INTO order_items(id,order_id,product_id,product_snapshot) SELECT 10000+n,10000+n,?,JSON_OBJECT('name','历史合成商品','code','PERF') FROM performance_numbers WHERE n<?",
		"INSERT INTO inventory_reservations(order_item_id,product_id,quantity,state) SELECT 10000+n,?,1,'RELEASED' FROM performance_numbers WHERE n<?",
		"INSERT INTO inventory_transactions(product_id,reserved_delta,event_key,order_id,reason) SELECT ?,1,CONCAT('performance-reserve:',n),10000+n,'performance reserve' FROM performance_numbers WHERE n<?",
		"INSERT INTO inventory_transactions(product_id,reserved_delta,event_key,order_id,reason) SELECT ?,-1,CONCAT('performance-release:',n),10000+n,'performance cancellation' FROM performance_numbers WHERE n<?",
	} {
		if _, err = conn.ExecContext(ctx, statement, f.products[0], count); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = conn.ExecContext(ctx, `INSERT INTO outbox_events(event_key,event_type,payload,state,next_attempt_at) SELECT CONCAT('performance-cancelled:',n),'order.cancelled',JSON_OBJECT('order_id',CAST(10000+n AS CHAR)),'DONE',? FROM performance_numbers WHERE n<?`, f.now, count); err != nil {
		t.Fatal(err)
	}
	t.Log("十万历史订单、密文、冻结商品、取消预占与双向账本样本已建立")
}

// seedPerformanceOrders 用途：批量写入与各业务号 AAD 绑定的历史取消订单密文。
// 参数：t 测试上下文；conn 夹具连接；f 夹具；offset 样本起点；count 批大小。
// 参数：calendarID 历史日历；plain 合成地址 JSON；phone 独立 HMAC。
// 返回值：无，不落盘明文地址。
func seedPerformanceOrders(t *testing.T, conn *sql.Conn, f *fulfillmentFixture, offset, count int, calendarID string, plain, phone []byte) {
	t.Helper()
	values := make([]string, 0, count)
	args := make([]any, 0, count*9)
	for n := offset; n < offset+count; n++ {
		number := fmt.Sprintf("O_PERFORMANCE_%08d", n)
		cipher, version, err := f.orders.Redemption.PII.Encrypt(plain, redemption.OrderAAD(number))
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, "(?,?,?,'CANCELLED',?,?,?,?,'','',?,'性能历史取消')")
		args = append(args, 10000+n, number, 10000+n, calendarID, cipher, version, phone, f.now)
	}
	query := "INSERT INTO orders(id,order_no,redemption_id,status,calendar_id,recipient_cipher,pii_key_version,phone_lookup,customer_note,admin_note,cancelled_at,cancel_reason) VALUES" + strings.Join(values, ",")
	if _, err := conn.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

// performanceActors 用途：建立一百张可兑样本及真实受限会话，测量兑换事务而非鉴真哈希。
// 参数：t 测试上下文；f 夹具；count 兑换并发样本数。
// 返回值：已认证 VOUCHER WRITE 主体，授权流程不进入计时区间。
func performanceActors(t *testing.T, f *fulfillmentFixture, count int) []identity.Actor {
	t.Helper()
	ctx := context.Background()
	typ, err := f.vouchers.CreateType(ctx, f.admin, voucher.TypeInput{Name: "并发性能样本", Mode: "ONE_OF", Count: 1}, testRequestKey(t), "performance")
	if err != nil {
		t.Fatal(err)
	}
	batch, err := f.vouchers.CreateBatch(ctx, f.admin, voucher.Input{TypeID: typ.ID, Label: "并发性能", Products: []string{f.products[0]}, Quantity: count, CodeLength: 16, CodeMode: "RANDOM", SecretLength: 12, SecretFormat: "MIXED", From: f.now.Add(-time.Hour), Until: f.now.Add(24 * time.Hour)}, testRequestKey(t), "performance")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := crypt.Hash("Synthetic-Performance-Only-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec("UPDATE voucher_batches SET state='SEALED' WHERE id=?", batch.ID); err != nil {
		t.Fatal(err)
	}
	actors := make([]identity.Actor, 0, count)
	for i := 0; i < count; i++ {
		result, err := f.db.Exec("INSERT INTO vouchers(batch_id,code,secret_hash,active,credential_state,valid_from,valid_until) VALUES(?,?,?,1,'SEALED',?,?)", batch.ID, fmt.Sprintf("PERFLIVE%08d", i), hash, f.now.Add(-time.Hour), f.now.Add(24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.db.Exec("INSERT INTO voucher_products(voucher_id,product_id) VALUES(?,?)", id, f.products[0]); err != nil {
			t.Fatal(err)
		}
		prepared, err := f.auth.PrepareCustomer("VOUCHER")
		if err != nil {
			t.Fatal(err)
		}
		tx, err := f.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = f.auth.InsertCustomer(ctx, tx, prepared, nil, "VOUCHER", "WRITE", id, 0, 1)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		a, err := f.auth.Authenticate(ctx, prepared.Token, "VOUCHER")
		if err != nil {
			t.Fatal(err)
		}
		actors = append(actors, a)
	}
	stock := inventory.New(f.db, f.operations.RequestKey)
	doc, err := stock.Create(ctx, f.admin, inventory.Input{Direction: "IN", Kind: "PHYSICAL", Items: []inventory.Line{{ProductID: f.products[0], Quantity: 200}}, Note: "性能并发准备"}, testRequestKey(t), "performance")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stock.Review(ctx, f.admin, doc.ID, inventory.Review{Version: doc.Version, Reason: "性能准备"}, true, testRequestKey(t), "performance"); err != nil {
		t.Fatal(err)
	}
	return actors
}

// explainPerformanceQueries 用途：记录列表游标与热点锁查询的真实执行计划。
// 参数：t 测试上下文；f 十万历史样本夹具。
// 返回值：无，查询计划必须可解析且包含索引访问。
func explainPerformanceQueries(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	for _, query := range []string{
		"SELECT id FROM orders WHERE deleted_at IS NULL AND status='CANCELLED' AND id<110000 ORDER BY id DESC LIMIT 21",
		"SELECT id FROM shipping_calendars WHERE ship_date='2026-10-13' FOR UPDATE",
		"SELECT product_id FROM inventory_balances WHERE product_id=1 FOR UPDATE",
	} {
		var raw string
		if err := f.db.QueryRow("EXPLAIN FORMAT=JSON " + query).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		t.Log("EXPLAIN " + raw)
		if !strings.Contains(raw, "key") {
			t.Fatal("执行计划未使用索引")
		}
	}
}
