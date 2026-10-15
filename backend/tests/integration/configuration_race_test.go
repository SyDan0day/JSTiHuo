package integration

import (
	"context"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
)

// TestConfigurationRace 用途：复现业务预读规则后配置改变的提交竞态。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，持旧快照的兑换必须在日历锁内按新规则拒绝。
func TestConfigurationRace(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	_, actor := f.customer(t, 0)
	service := f.orders.Redemption
	number := service.Number
	prepared, proceed := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	service.Number = func(now time.Time) (string, error) { close(prepared); <-proceed; return number(now) }
	go func() {
		_, err := service.Redeem(context.Background(), actor, f.input(), "config-race-redeem-001", "test")
		done <- err
	}()
	select {
	case <-prepared:
	case <-time.After(5 * time.Second):
		close(proceed)
		t.Fatal("兑换未预读配置")
	}
	config := response[operations.Config](t, f.client.call("GET", "admin/config", nil, "admin", ""), 200)
	config.MinLead = 2
	result := f.client.call("PUT", "admin/config", config, "admin", testRequestKey(t))
	close(proceed)
	response[operations.Config](t, result, 200)
	select {
	case err := <-done:
		requireFault(t, err, "DATE_UNAVAILABLE")
	case <-time.After(5 * time.Second):
		t.Fatal("竞态兑换未结束")
	}
	service.Number = number
	var count int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count); err != nil || count != 0 {
		t.Fatal("配置竞态仍创建订单", err)
	}
	checkFulfillmentLedger(t, f.db)
}

// TestInventoryReconciliation 用途：验证全量账本核对能发现实存与活动预占漂移。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，只读接口不得通过自动补账掩盖差异。
func TestInventoryReconciliation(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	read := func() operations.Reconcile {
		return response[operations.Reconcile](t, f.client.call("GET", "admin/inventory/reconciliation", nil, "admin", ""), 200)
	}
	if result := read(); !result.Balanced || len(result.Products) != 0 {
		t.Fatal("正常账本被报漂移")
	}
	if _, err := f.db.Exec("UPDATE inventory_balances SET on_hand=on_hand+1 WHERE product_id=?", f.products[0]); err != nil {
		t.Fatal(err)
	}
	result := read()
	if result.Balanced || len(result.Products) != 1 || result.Products[0] != f.products[0] {
		t.Fatal("实存漂移未被定位")
	}
	if _, err := f.db.Exec("UPDATE inventory_balances SET on_hand=on_hand-1 WHERE product_id=?", f.products[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("UPDATE inventory_balances SET reserved=reserved+1 WHERE product_id=?", f.products[1]); err != nil {
		t.Fatal(err)
	}
	result = read()
	if result.Balanced || len(result.Products) != 1 || result.Products[0] != f.products[1] {
		t.Fatal("预占漂移未被定位")
	}
}
