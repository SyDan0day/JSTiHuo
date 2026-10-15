package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
)

// advanceFulfillment 用途：前进隔离业务时钟并保持测试会话有效。
// 参数：t 测试上下文；f 夹具；delay 前进时长。
// 返回值：无，不修改生产时钟与会话规则。
func advanceFulfillment(t *testing.T, f *fulfillmentFixture, delay time.Duration) {
	t.Helper()
	f.now = f.now.Add(delay)
	if _, err := f.db.Exec("UPDATE sessions SET expires_at=?,last_seen_at=?,reauth_at=?", f.now.Add(time.Hour), f.now, f.now); err != nil {
		t.Fatal(err)
	}
}

// TestAfterSales 用途：验证本单授权、唯一活动案件、补发回滚与签收闭环。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，补发不逆转原单、不重复扣库存，异常整体回滚。
func TestAfterSales(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	ctx := context.Background()
	c, _ := f.customer(t, 0)
	original := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	other, _ := f.customer(t, 1)
	response[redemption.Order](t, other.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MOCK", Name: "模拟承运商", Provider: "MOCK", Enabled: true}, "admin", testRequestKey(t)), 201)
	in := operations.AfterSaleInput{Kind: "REPLACEMENT", Reason: "测试补发", Items: []operations.CaseItem{{OrderItemID: original.Items[0].ID, Quantity: 1}, {OrderItemID: original.Items[1].ID, Quantity: 1}}}
	response[map[string]any](t, c.call("POST", "order/after-sales", in, "order", testRequestKey(t)), 409)
	advanceFulfillment(t, f, 24*time.Hour)
	response[order.Shipment](t, f.client.call("POST", "admin/orders/"+original.ID+"/shipments", order.ShipInput{Version: original.Version, CarrierID: carrier.ID, Tracking: "ORIGINAL-001"}, "admin", testRequestKey(t)), 200)
	response[map[string]any](t, other.call("POST", "order/after-sales", in, "order", testRequestKey(t)), 409)
	key := testRequestKey(t)
	created := response[operations.AfterSale](t, c.call("POST", "order/after-sales", in, "order", key), 200)
	replayed := response[operations.AfterSale](t, c.call("POST", "order/after-sales", in, "order", key), 200)
	if replayed.ID != created.ID {
		t.Fatal("售后重放创建第二案件")
	}
	response[map[string]any](t, c.call("POST", "order/after-sales", in, "order", testRequestKey(t)), 409)
	otherPage := response[operations.Page[operations.AfterSale]](t, other.call("GET", "order/after-sales", nil, "order", ""), 200)
	if len(otherPage.Items) != 0 {
		t.Fatal("跨单读取售后")
	}
	path := "admin/after-sales/" + created.ID
	decision := operations.AfterSaleDecision{Version: created.Version, Decision: "APPROVE", Resolution: "同意按原单补发"}
	approved := response[operations.AfterSale](t, f.client.call("POST", path+"/decision", decision, "admin", testRequestKey(t)), 200)
	response[map[string]any](t, f.client.call("POST", path+"/decision", decision, "admin", testRequestKey(t)), 409)
	response[map[string]any](t, f.client.call("POST", path+"/decision", operations.AfterSaleDecision{Version: approved.Version, Decision: "CLOSE", Resolution: "跳过签收"}, "admin", testRequestKey(t)), 409)
	ship := order.ShipInput{Version: approved.Version, CarrierID: carrier.ID, Tracking: "REPLACEMENT-001"}
	f.orders.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "replacement:1" {
			return errors.New("injected")
		}
		return nil
	}
	response[map[string]any](t, f.client.call("POST", path+"/shipments", ship, "admin", testRequestKey(t)), 500)
	f.orders.FaultTx = nil
	var count int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM shipments WHERE after_sale_id=?", created.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("补发故障未整体回滚", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.operations.ReplacementShipment(ctx, f.admin, created.ID, ship, fmt.Sprintf("replacement-race-%02d", i), "test")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			var fault *identity.Fault
			if !errors.As(err, &fault) || fault.Status != 409 {
				t.Fatal(err)
			}
		}
	}
	if success != 1 {
		t.Fatal("补发并发胜者数量", success)
	}
	var shipmentID string
	if err := f.db.QueryRow("SELECT id FROM shipments WHERE after_sale_id=?", created.ID).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	var onHand, reserved int
	if err := f.db.QueryRow("SELECT on_hand,reserved FROM inventory_balances WHERE product_id=?", f.products[0]).Scan(&onHand, &reserved); err != nil || onHand != 8 || reserved != 1 {
		t.Fatal("补发侵占其他订单库存", onHand, reserved, err)
	}
	if err := f.orders.RecordTracking(ctx, order.TrackingInput{ShipmentID: shipmentID, ProviderID: "replacement-delivered", Type: "DELIVERED", Description: "MOCK 测试签收", Occurred: f.now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	closed := response[operations.AfterSale](t, f.client.call("GET", path, nil, "admin", ""), 200)
	if closed.State != "CLOSED" {
		t.Fatal("补发签收未关闭案件")
	}
	current := response[redemption.Order](t, c.call("GET", "order", nil, "order", ""), 200)
	if current.Status != "SHIPPED" {
		t.Fatal("补发签收改变原订单")
	}
	var refs string
	if err := f.db.QueryRow("SELECT CAST(JSON_ARRAYAGG(result_ref) AS CHAR) FROM idempotency_keys WHERE scope LIKE '%customerAfterSale'").Scan(&refs); err != nil {
		t.Fatal(err)
	}
	var saved []map[string]string
	if json.Unmarshal([]byte(refs), &saved) != nil || len(saved) != 1 || len(saved[0]) != 1 {
		t.Fatal("案件正文写入幂等表")
	}
	checkFulfillmentLedger(t, f.db)
}
