package integration

import (
	"context"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/app/voucher"
)

// TestContractPositiveSupplement 用途：补齐业务服务测试之外的真实 HTTP 正向契约。
// 参数：t 测试上下文，需隔离 MySQL；CONTRACT_VALIDATE 启用 Schema 校验。
// 返回值：无；读取、发行、订单状态与补发均验证真实持久化结果。
func TestContractPositiveSupplement(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	for _, path := range []string{"products", "admin/products", "admin/products/" + f.products[0], "admin/categories", "admin/content", "admin/voucher-types", "admin/files", "admin/after-sales"} {
		response[map[string]any](t, f.client.call("GET", path, nil, "admin", ""), 200)
	}
	roles := response[operations.Page[operations.Role]](t, f.client.call("GET", "admin/roles", nil, "admin", ""), 200)
	user := response[operations.Admin](t, f.client.call("POST", "admin/admins", operations.AdminInput{Login: "contract-managed", Password: "Synthetic-Contract-Temporary-123", Roles: []string{roles.Items[0].ID}}, "admin", testRequestKey(t)), 201)
	response[operations.Admin](t, f.client.call("PATCH", "admin/admins/"+user.ID, operations.AdminUpdate{Version: user.Version, Enabled: false, Roles: user.Roles}, "admin", testRequestKey(t)), 200)
	exerciseContractIssuance(t, f)
	exerciseContractOrders(t, f)
	checkFulfillmentLedger(t, f.db)
}

// exerciseContractIssuance 用途：经 HTTP 完成新批次生成、封存、编辑、批量及密码重置。
// 参数：t 测试上下文；f 已入库商品的隔离夹具。
// 返回值：无；不把生成或重置的卡密写入日志与文件。
func exerciseContractIssuance(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	c := f.client
	typ := response[voucher.Type](t, c.call("POST", "admin/voucher-types", voucher.TypeInput{Name: "契约单选", Mode: "ONE_OF", Count: 1}, "admin", testRequestKey(t)), 201)
	batch := response[voucher.Batch](t, c.call("POST", "admin/voucher-batches", voucher.Input{TypeID: typ.ID, Label: "契约发行", Products: []string{f.products[0]}, Quantity: 1, Prefix: "CT", CodeLength: 16, CodeMode: "RANDOM", SecretLength: 12, SecretFormat: "MIXED", From: f.now.Add(-time.Hour), Until: f.now.Add(30 * 24 * time.Hour), Activate: true}, "admin", testRequestKey(t)), 201)
	response[voucher.Batch](t, c.call("POST", "admin/voucher-batches/"+batch.ID+"/generate", map[string]int64{"version": batch.Version}, "admin", testRequestKey(t)), 202)
	if worked, err := f.vouchers.RunOnce(context.Background()); err != nil || !worked {
		t.Fatal("契约发行 worker 失败", err)
	}
	batch = response[voucher.Batch](t, c.call("GET", "admin/voucher-batches/"+batch.ID, nil, "admin", ""), 200)
	response[voucher.Batch](t, c.call("POST", "admin/voucher-batches/"+batch.ID+"/seal", voucher.VersionReason{Version: batch.Version, Reason: "完成契约发行"}, "admin", testRequestKey(t)), 200)
	page := response[voucher.Page[voucher.Voucher]](t, c.call("GET", "admin/vouchers?batch_id="+batch.ID, nil, "admin", ""), 200)
	if len(page.Items) != 1 {
		t.Fatal("契约发行卡数量不符")
	}
	card := response[voucher.Voucher](t, c.call("GET", "admin/vouchers/"+page.Items[0].ID, nil, "admin", ""), 200)
	card = response[voucher.Voucher](t, c.call("PATCH", "admin/vouchers/"+card.ID, voucher.EditInput{VersionReason: voucher.VersionReason{Version: card.Version, Reason: "编辑有效期"}, From: card.From, Until: card.Until}, "admin", testRequestKey(t)), 200)
	response[map[string]any](t, c.call("POST", "admin/vouchers/actions", voucher.BulkInput{IDs: []string{card.ID}, Versions: []int64{card.Version}, Action: "DEACTIVATE", Reason: "批量契约"}, "admin", testRequestKey(t)), 201)
	card = response[voucher.Voucher](t, c.call("GET", "admin/vouchers/"+card.ID, nil, "admin", ""), 200)
	response[voucher.Delivery](t, c.call("POST", "admin/vouchers/"+card.ID+"/reset-secret", voucher.ResetInput{VersionReason: voucher.VersionReason{Version: card.Version, Reason: "重置契约"}, Length: 12, Format: "MIXED"}, "admin", testRequestKey(t)), 200)
}

// exerciseContractOrders 用途：经 HTTP 验证编辑、取消回收恢复及完成后补发路径。
// 参数：t 测试上下文；f 有两张可用卡的隔离夹具。
// 返回值：无；版本来自权威响应，每个动作只使用自身幂等键。
func exerciseContractOrders(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	c := f.client
	customer, _ := f.customer(t, 0)
	response[map[string]any](t, customer.call("GET", "voucher-session", nil, "voucher", ""), 200)
	original := response[redemption.Order](t, customer.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	original = response[redemption.Order](t, c.call("PATCH", "admin/orders/"+original.ID, order.EditInput{Version: original.Version, Address: f.input().Address, Date: original.Date, Note: "契约编辑"}, "admin", testRequestKey(t)), 200)
	response[order.Private](t, c.call("POST", "admin/orders/"+original.ID+"/private-view", order.VersionReason{Version: original.Version, Reason: "契约资料读取"}, "admin", testRequestKey(t)), 200)
	other, _ := f.customer(t, 1)
	cancelled := response[redemption.Order](t, other.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	for _, action := range []string{"cancel", "trash", "restore"} {
		cancelled = response[redemption.Order](t, c.call("POST", "admin/orders/"+cancelled.ID+"/"+action, order.VersionReason{Version: cancelled.Version, Reason: "契约" + action}, "admin", testRequestKey(t)), 200)
	}
	carrier := response[shipping.Carrier](t, c.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MANUAL", Name: "契约人工物流", Provider: "MANUAL", Enabled: true}, "admin", testRequestKey(t)), 201)
	response[map[string]any](t, c.call("GET", "admin/carriers", nil, "admin", ""), 200)
	advanceFulfillment(t, f, 24*time.Hour)
	shipment := response[order.Shipment](t, c.call("POST", "admin/orders/"+original.ID+"/shipments", order.ShipInput{Version: original.Version, CarrierID: carrier.ID, Tracking: "CONTRACT-ORIGINAL"}, "admin", testRequestKey(t)), 200)
	response[map[string]any](t, customer.call("GET", "order/shipments", nil, "order", ""), 200)
	response[map[string]any](t, c.call("GET", "admin/shipments/"+shipment.ID+"/events", nil, "admin", ""), 200)
	original = response[redemption.Order](t, c.call("GET", "admin/orders/"+original.ID, nil, "admin", ""), 200)
	response[redemption.Order](t, c.call("POST", "admin/orders/"+original.ID+"/complete", order.VersionReason{Version: original.Version, Reason: "契约人工签收"}, "admin", testRequestKey(t)), 200)
	created := response[operations.AfterSale](t, customer.call("POST", "order/after-sales", operations.AfterSaleInput{Kind: "REPLACEMENT", Reason: "契约补发", Items: []operations.CaseItem{{OrderItemID: original.Items[0].ID, Quantity: 1}}}, "order", testRequestKey(t)), 200)
	created = response[operations.AfterSale](t, c.call("POST", "admin/after-sales/"+created.ID+"/decision", operations.AfterSaleDecision{Version: created.Version, Decision: "APPROVE", Resolution: "契约批准"}, "admin", testRequestKey(t)), 200)
	response[order.Shipment](t, c.call("POST", "admin/after-sales/"+created.ID+"/shipments", order.ShipInput{Version: created.Version, CarrierID: carrier.ID, Tracking: "CONTRACT-REPLACEMENT"}, "admin", testRequestKey(t)), 200)
}
