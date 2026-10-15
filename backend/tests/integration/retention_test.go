package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
)

// TestOrderPrivacyRetention 用途：验证保留边界、活动售后暂停及匿名化事务回滚。
// 参数：t 测试上下文，需真实隔离 MySQL。
// 返回值：无；清理后订单和账本仍可读，原手机索引、收件人及会话不可用。
func TestOrderPrivacyRetention(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	ctx := context.Background()
	c, _ := f.customer(t, 0)
	original := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MANUAL", Name: "隐私测试承运商", Provider: "MANUAL", Enabled: true}, "admin", testRequestKey(t)), 201)
	advanceFulfillment(t, f, 24*time.Hour)
	response[order.Shipment](t, f.client.call("POST", "admin/orders/"+original.ID+"/shipments", order.ShipInput{Version: original.Version, CarrierID: carrier.ID, Tracking: "RETENTION-001"}, "admin", testRequestKey(t)), 200)
	if _, err := f.orders.Complete(ctx, f.admin, original.ID, order.VersionReason{Version: 2, Reason: "隐私保留测试完成"}, testRequestKey(t), "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("UPDATE orders SET completed_at=? WHERE id=?", f.now.AddDate(0, 0, -180), original.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.operations.RetireCompletedPII(ctx, 179, 100); err == nil {
		t.Fatal("接受低于政策的保留期")
	}
	if count, err := f.operations.RetireCompletedPII(ctx, 180, 100); err != nil || count != 0 {
		t.Fatal("边界日提前清理", count, err)
	}
	created := response[operations.AfterSale](t, c.call("POST", "order/after-sales", operations.AfterSaleInput{Kind: "COMPLAINT", Reason: "暂停隐私清理", Items: []operations.CaseItem{}}, "order", testRequestKey(t)), 200)
	advanceFulfillment(t, f, time.Second)
	if count, err := f.operations.RetireCompletedPII(ctx, 180, 100); err != nil || count != 0 {
		t.Fatal("活动售后未暂停清理", count, err)
	}
	response[operations.AfterSale](t, f.client.call("POST", "admin/after-sales/"+created.ID+"/decision", operations.AfterSaleDecision{Version: created.Version, Decision: "CLOSE", Resolution: "案件结束"}, "admin", testRequestKey(t)), 200)
	if _, err := f.db.Exec("CREATE TRIGGER fail_retention_audit BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected'"); err != nil {
		t.Fatal(err)
	}
	if count, err := f.operations.RetireCompletedPII(ctx, 180, 100); err == nil || count != 0 {
		t.Fatal("审计故障没有整体回滚")
	}
	var lookup []byte
	if err := f.db.QueryRow("SELECT phone_lookup FROM orders WHERE id=?", original.ID).Scan(&lookup); err != nil || bytes.Equal(lookup, make([]byte, 32)) {
		t.Fatal("审计故障破坏手机索引", err)
	}
	if _, err := f.db.Exec("DROP TRIGGER fail_retention_audit"); err != nil {
		t.Fatal(err)
	}
	type retired struct {
		count int
		err   error
	}
	results := make(chan retired, 3)
	for i := 0; i < cap(results); i++ {
		go func() {
			count, err := f.operations.RetireCompletedPII(ctx, 180, 100)
			results <- retired{count, err}
		}()
	}
	count := 0
	for i := 0; i < cap(results); i++ {
		result := <-results
		if result.err != nil {
			t.Fatal("并发匿名化失败", result.err)
		}
		count += result.count
	}
	if count != 1 {
		t.Fatal("匿名化提交次数不符", count)
	}
	if count, err := f.operations.RetireCompletedPII(ctx, 180, 100); err != nil || count != 0 {
		t.Fatal("匿名化重复执行", count, err)
	}
	cleared, err := f.orders.Redemption.GetOrder(ctx, original.ID, true)
	if err != nil || cleared.Status != "COMPLETED" || cleared.Recipient != (redemption.Recipient{}) {
		t.Fatal("匿名化订单不可读或仍有收件人", err)
	}
	if err := f.db.QueryRow("SELECT phone_lookup FROM orders WHERE id=?", original.ID).Scan(&lookup); err != nil || !bytes.Equal(lookup, make([]byte, 32)) {
		t.Fatal("手机索引未清除", err)
	}
	response[map[string]any](t, c.call("GET", "order", nil, "order", ""), 401)
	checkFulfillmentLedger(t, f.db)
}
