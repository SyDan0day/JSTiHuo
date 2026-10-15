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

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/infra/integrations"
)

type testCarrierProvider struct {
	calls   int
	lookup  int
	result  integrations.Label
	failure error
	found   bool
}

// CreateLabel 用途：模拟一次外部面单调用并统计请求。
// 参数：ctx 上下文；key 稳定请求键；in 订单引用。
// 返回值：预设结果或故障。
func (p *testCarrierProvider) CreateLabel(ctx context.Context, key string, in integrations.LabelRequest) (integrations.Label, error) {
	p.calls++
	return p.result, p.failure
}

// GetLabel 用途：模拟供应商按原请求键查单。
// 参数：ctx 上下文；key 原请求键。
// 返回值：预设结果、存在标记及错误。
func (p *testCarrierProvider) GetLabel(ctx context.Context, key string) (integrations.Label, bool, error) {
	p.lookup++
	return p.result, p.found, nil
}

// Idempotent 用途：模拟不支持幂等重发的供应商。
// 参数：无。
// 返回值：false，必须查单或人工核对。
func (p *testCarrierProvider) Idempotent() bool { return false }

type testNotifier struct {
	sends, lookups int
	failure        error
	found          bool
}

// Send 用途：模拟外部发送回执或未知结果。
// 参数：ctx 上下文；key 事件键；in 通知引用。
// 返回值：固定测试回执或预设错误。
func (p *testNotifier) Send(ctx context.Context, key string, in integrations.Notice) (string, error) {
	p.sends++
	return "TEST-RECEIPT", p.failure
}

// Lookup 用途：模拟按事件键核对既有发送结果。
// 参数：ctx 上下文；key 事件键。
// 返回值：测试回执与存在标记。
func (p *testNotifier) Lookup(ctx context.Context, key string) (string, bool, error) {
	p.lookups++
	return "TEST-RECEIPT", p.found, nil
}

// Idempotent 用途：模拟不支持幂等重发的消息供应商。
// 参数：无。
// 返回值：false。
func (p *testNotifier) Idempotent() bool { return false }

// TestAsyncIntegrations 用途：验证面单版本绑定、超时未知核对、Outbox 栅栏和丢响应恢复。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，未知结果不得盲重发，任务失败不伪造发货。
func TestAsyncIntegrations(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	ctx := context.Background()
	c, _ := f.customer(t, 0)
	original := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	other, _ := f.customer(t, 1)
	second := response[redemption.Order](t, other.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MOCK", Name: "模拟面单", Provider: "MOCK", Enabled: true}, "admin", testRequestKey(t)), 201)
	path := "admin/orders/" + original.ID + "/label-jobs"
	in := operations.VersionReason{Version: original.Version, Reason: "测试自动面单"}
	response[map[string]any](t, f.client.call("POST", path, in, "admin", testRequestKey(t)), 503)
	config, _ := json.Marshal(map[string]any{"default_carrier_id": carrier.ID, "notification_mode": "MOCK"})
	if _, err := f.db.Exec("INSERT INTO system_configs(config_key,value) VALUES('shipping_calendar',?)", config); err != nil {
		t.Fatal(err)
	}
	job := response[operations.Job](t, f.client.call("POST", path, in, "admin", testRequestKey(t)), 202)
	again := response[operations.Job](t, f.client.call("POST", path, in, "admin", testRequestKey(t)), 202)
	if again.ID != job.ID {
		t.Fatal("同订单产生多个活动面单")
	}
	advanceFulfillment(t, f, 24*time.Hour)
	ship := order.ShipInput{Version: original.Version, CarrierID: carrier.ID, Tracking: "BYPASS-LABEL"}
	response[map[string]any](t, f.client.call("POST", "admin/orders/"+original.ID+"/shipments", ship, "admin", testRequestKey(t)), 409)
	provider := &testCarrierProvider{result: integrations.Label{CarrierID: carrier.ID, Tracking: "RECONCILED-001"}, failure: integrations.ErrUnknown}
	f.operations.Carriers["MOCK"] = provider
	runFileJob(t, f)
	unknown := getFileJob(t, f, job.ID)
	if unknown.State != "UNKNOWN" || provider.calls != 1 {
		t.Fatal("未知面单未停止自动调用")
	}
	resolve := operations.ResolveJob{Version: unknown.Version, Resolution: "CONFIRMED_CREATED", Evidence: "synthetic-provider-case-001", Reason: "测试工单核对", CarrierID: carrier.ID, Tracking: provider.result.Tracking}
	if _, err := f.db.Exec("UPDATE sessions SET reauth_at=NULL WHERE kind='ADMIN'"); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, f.client.call("POST", "admin/jobs/"+job.ID+"/resolve", resolve, "admin", testRequestKey(t)), 403)
	if _, err := f.db.Exec("UPDATE sessions SET reauth_at=? WHERE kind='ADMIN'", f.now); err != nil {
		t.Fatal(err)
	}
	ready := response[operations.Job](t, f.client.call("POST", "admin/jobs/"+job.ID+"/resolve", resolve, "admin", testRequestKey(t)), 200)
	if ready.State != "READY" || ready.Tracking != provider.result.Tracking {
		t.Fatal("核对面单未返回单号")
	}
	response[map[string]any](t, f.client.call("POST", "admin/orders/"+original.ID+"/shipments", ship, "admin", testRequestKey(t)), 409)
	ship.Tracking = ready.Tracking
	response[order.Shipment](t, f.client.call("POST", "admin/orders/"+original.ID+"/shipments", ship, "admin", testRequestKey(t)), 200)
	exerciseTrackingWebhook(t, f, ship.Tracking)
	response[map[string]any](t, f.client.call("POST", "admin/jobs/"+job.ID+"/resolve", operations.ResolveJob{Version: ready.Version, Resolution: "CONFIRMED_VOID", Evidence: "void-request", Reason: "已发禁止撤销"}, "admin", testRequestKey(t)), 409)
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	notices := response[operations.Page[operations.Notification]](t, f.client.call("GET", "admin/notifications", nil, "admin", ""), 200)
	if len(notices.Items) < 3 {
		t.Fatal("通知未落库")
	}
	for _, n := range notices.Items {
		if n.State != "SENT" || n.Channel != "MOCK" {
			t.Fatal("MOCK 通知未完成")
		}
	}
	// 第二单模拟外部调用成功、提交回执失败；接管者只能查单，不能再次创建。
	provider.failure = nil
	provider.found = true
	provider.result.Tracking = "RECOVERED-002"
	secondJob := response[operations.Job](t, f.client.call("POST", "admin/orders/"+second.ID+"/label-jobs", operations.VersionReason{Version: second.Version, Reason: "恢复测试"}, "admin", testRequestKey(t)), 202)
	f.operations.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "label:finish" {
			return errors.New("injected")
		}
		return nil
	}
	if worked, err := f.operations.RunOnce(ctx); !worked || err == nil {
		t.Fatal("未注入面单回执故障")
	}
	f.operations.FaultTx = nil
	advanceFulfillment(t, f, 61*time.Second)
	runFileJob(t, f)
	recovered := getFileJob(t, f, secondJob.ID)
	if recovered.State != "READY" || provider.calls != 2 || provider.lookup != 1 {
		t.Fatal("丢响应后重复创建面单", provider.calls, provider.lookup)
	}
	t.Run("通知未知与人工核对", func(t *testing.T) { exerciseNotificationUnknown(t, f, original.ID) })
	t.Run("通知租约与回执恢复", func(t *testing.T) { exerciseNotificationRecovery(t, f, original.ID) })
	checkFulfillmentLedger(t, f.db)
}

// enqueueNotice 用途：写入独立测试事件并领取。
// 参数：t 测试上下文；f 夹具；orderID 真实订单。
// 返回值：当前租约凭证。
func enqueueNotice(t *testing.T, f *fulfillmentFixture, orderID string) operations.OutboxClaim {
	t.Helper()
	if _, err := f.db.Exec("INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,'order.updated',JSON_OBJECT('order_id',?),?)", testRequestKey(t), orderID, f.now); err != nil {
		t.Fatal(err)
	}
	for {
		claim, ok, err := f.operations.ClaimOutbox(context.Background())
		if err != nil || !ok {
			t.Fatal("未领取事件", err)
		}
		if claim.Kind == "order.updated" {
			return claim
		}
		if err = f.operations.ProcessOutbox(context.Background(), claim); err != nil {
			t.Fatal(err)
		}
	}
}

// exerciseNotificationUnknown 用途：验证 UNKNOWN 禁止重试及近期复核后安全恢复。
// 参数：t 测试上下文；f 夹具；orderID 真实订单。
// 返回值：无，人工核对只记录证据摘要。
func exerciseNotificationUnknown(t *testing.T, f *fulfillmentFixture, orderID string) {
	ctx := context.Background()
	provider := &testNotifier{failure: integrations.ErrUnknown}
	f.operations.Notifier = provider
	claim := enqueueNotice(t, f, orderID)
	if err := f.operations.ProcessOutbox(ctx, claim); err != nil {
		t.Fatal(err)
	}
	page := response[operations.Page[operations.Notification]](t, f.client.call("GET", "admin/notifications?status=UNKNOWN", nil, "admin", ""), 200)
	if len(page.Items) != 1 {
		t.Fatal("未知通知缺失")
	}
	n := page.Items[0]
	path := "admin/notifications/" + n.ID
	response[map[string]any](t, f.client.call("POST", path+"/retry", operations.VersionReason{Version: n.Version, Reason: "盲重试"}, "admin", testRequestKey(t)), 409)
	resolve := operations.ResolveNotification{Version: n.Version, Resolution: "CONFIRMED_NOT_SENT", Evidence: "synthetic-not-sent-case-001", Reason: "人工核实未发送"}
	resolved := response[operations.Notification](t, f.client.call("POST", path+"/resolve", resolve, "admin", testRequestKey(t)), 200)
	if resolved.State != "FAILED" {
		t.Fatal("核对未发送未进入失败态")
	}
	response[operations.Notification](t, f.client.call("POST", path+"/retry", operations.VersionReason{Version: resolved.Version, Reason: "核对后重试"}, "admin", testRequestKey(t)), 202)
	provider.failure = nil
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if provider.sends != 2 {
		t.Fatal("已核对通知未执行重试")
	}
}

// exerciseNotificationRecovery 用途：验证过期版本拒绝提交与发送后丢回执的查单恢复。
// 参数：t 测试上下文；f 夹具；orderID 真实订单。
// 返回值：无，不支持幂等的供应商不被重复 Send。
func exerciseNotificationRecovery(t *testing.T, f *fulfillmentFixture, orderID string) {
	ctx := context.Background()
	provider := &testNotifier{found: true}
	f.operations.Notifier = provider
	claim := enqueueNotice(t, f, orderID)
	f.operations.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "outbox:finish" {
			return errors.New("injected")
		}
		return nil
	}
	if err := f.operations.ProcessOutbox(ctx, claim); err == nil {
		t.Fatal("通知回执故障未触发")
	}
	f.operations.FaultTx = nil
	advanceFulfillment(t, f, 61*time.Second)
	current, ok, err := f.operations.ClaimOutbox(ctx)
	if err != nil || !ok || current.ID != claim.ID {
		t.Fatal("租约未接管", err)
	}
	if err = f.operations.ProcessOutbox(ctx, claim); !errors.Is(err, operations.ErrJobLeaseLost) {
		t.Fatal("旧工作者未被栅栏拒绝", err)
	}
	if err = f.operations.ProcessOutbox(ctx, current); err != nil {
		t.Fatal(err)
	}
	if provider.sends != 1 || provider.lookups != 1 {
		t.Fatal("通知丢回执重复发送")
	}
	// 两个工作者不能同时领取同一新事件。
	if _, err = f.db.Exec("INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,'job.requested',JSON_OBJECT(),?)", testRequestKey(t), f.now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, ok, err := f.operations.ClaimOutbox(ctx)
			errs <- err
			if ok {
				ids <- claim.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatal(fmt.Sprintf("重复领取 %s", id))
		}
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatal("领取数量异常")
	}
}
