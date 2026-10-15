package integration

import (
	"context"
	"testing"
	"time"

	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/infra/integrations"
)

type testTrackerProvider struct {
	testCarrierProvider
	Events []integrations.TrackingEvent
	Calls  int
}

// Track 用途：返回测试供应商已记录的固定事件集合。
// 参数：ctx 上下文；carrier 承运商编码；tracking 已发单号。
// 返回值：事件集合或取消错误，不调用真实供应商。
func (p *testTrackerProvider) Track(ctx context.Context, carrier, tracking string) ([]integrations.TrackingEvent, error) {
	p.Calls++
	return p.Events, ctx.Err()
}

// TestTrackingSchedule 用途：验证六小时去重、签收停止拉取与三十天人工异常。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，不得重复拉取同窗口、自动完成超时运单或伪造 MANUAL 轨迹。
func TestTrackingSchedule(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	ctx := context.Background()
	c, _ := f.customer(t, 0)
	first := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	other, _ := f.customer(t, 1)
	second := response[redemption.Order](t, other.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "TESTPARTNER", Name: "本地测试拉取适配器", Provider: "KDNIAO", Enabled: true}, "admin", testRequestKey(t)), 201)
	manual := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MANUAL", Name: "人工物流", Provider: "MANUAL", Enabled: true}, "admin", testRequestKey(t)), 201)
	response[map[string]any](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "BADREF", Name: "未配置引用", Provider: "KDNIAO", SecretRef: "unconfigured-reference", Enabled: true}, "admin", testRequestKey(t)), 503)
	advanceFulfillment(t, f, 24*time.Hour)
	shipment := response[order.Shipment](t, f.client.call("POST", "admin/orders/"+first.ID+"/shipments", order.ShipInput{Version: first.Version, CarrierID: carrier.ID, Tracking: "TEST-POLL-001"}, "admin", testRequestKey(t)), 200)
	manualShipment := response[order.Shipment](t, f.client.call("POST", "admin/orders/"+second.ID+"/shipments", order.ShipInput{Version: second.Version, CarrierID: manual.ID, Tracking: "MANUAL-TIMEOUT-001"}, "admin", testRequestKey(t)), 200)
	adapter := &testTrackerProvider{Events: []integrations.TrackingEvent{{ID: "pull-transit", Type: "IN_TRANSIT", Occurred: f.now}}}
	f.operations.Carriers["KDNIAO"] = adapter
	for i := 0; i < 2; i++ {
		if err := f.operations.ScheduleTracking(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if adapter.Calls != 1 {
		t.Fatal("同六小时窗口重复拉取")
	}
	var state string
	if err := f.db.QueryRow("SELECT status FROM shipments WHERE id=?", shipment.ID).Scan(&state); err != nil || state != "IN_TRANSIT" {
		t.Fatal("拉取未推进运输状态", err)
	}
	advanceFulfillment(t, f, 6*time.Hour)
	adapter.Events = []integrations.TrackingEvent{{ID: "pull-delivered", Type: "DELIVERED", Occurred: f.now}}
	if err := f.operations.ScheduleTracking(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if adapter.Calls != 2 {
		t.Fatal("新窗口未拉取")
	}
	if err := f.operations.ScheduleTracking(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if adapter.Calls != 2 {
		t.Fatal("签收后仍拉取")
	}
	advanceFulfillment(t, f, 30*24*time.Hour)
	for i := 0; i < 2; i++ {
		if err := f.operations.ScheduleTracking(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.QueryRow("SELECT status FROM shipments WHERE id=?", manualShipment.ID).Scan(&state); err != nil || state != "EXCEPTION" {
		t.Fatal("三十天未进入人工异常", err)
	}
	if err := f.db.QueryRow("SELECT status FROM orders WHERE id=?", second.ID).Scan(&state); err != nil || state != "SHIPPED" {
		t.Fatal("三十天超时被自动完成", err)
	}
	var count int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM shipment_events WHERE shipment_id=?", manualShipment.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("人工异常重复写入", err)
	}
	checkFulfillmentLedger(t, f.db)
}
