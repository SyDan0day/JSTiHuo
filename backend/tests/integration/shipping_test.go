package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/app/voucher"
	"jstihuo/internal/bootstrap"
	crypt "jstihuo/internal/infra/crypto"
	"jstihuo/internal/infra/integrations/mock"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
)

type fulfillmentFixture struct {
	db         *sql.DB
	auth       *identity.Service
	orders     *order.Service
	vouchers   *voucher.Service
	carriers   *shipping.Service
	operations *operations.Service
	admin      identity.Actor
	handler    http.Handler
	now        time.Time
	products   []string
	cards      [][]string
	client     *fulfillmentClient
}
type fulfillmentClient struct {
	t       *testing.T
	handler http.Handler
	cookies map[string]*http.Cookie
	tokens  map[string]string
}

// call 用途：通过真实 HTTP Handler 发起请求并维护各类型 Cookie 与 CSRF。
// 参数：method 请求方法；path API 后缀；input 请求正文；scope 会话类型；key 幂等键。
// 返回值：内存响应器，凭证正文不写日志或磁盘。
func (c *fulfillmentClient) call(method, path string, input any, scope, key string) *httptest.ResponseRecorder {
	c.t.Helper()
	var body bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&body).Encode(input); err != nil {
			c.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, "/api/v1/"+path, &body)
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", c.tokens[scope])
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
		} else {
			c.cookies[cookie.Name] = cookie
		}
	}
	if token := w.Header().Get("X-CSRF-Token"); token != "" {
		c.tokens[w.Header().Get("X-CSRF-Scope")] = token
	}
	return w
}

// response 用途：验证期望状态并解码成功响应。
// 参数：t 测试上下文；w HTTP 响应；status 期望状态码。
// 返回值：指定类型结果，失败只输出状态及结构化错误，不输出成功凭证。
func response[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if w.Code != status {
		t.Fatalf("HTTP status %d, expected %d: %s", w.Code, status, w.Body.String())
	}
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// challenge 用途：获取并解答真实预认证挑战。
// 参数：purpose 固定挑战用途。
// 返回值：挑战标识与答案，不修改业务订单。
func (c *fulfillmentClient) challenge(purpose string) (string, string) {
	c.t.Helper()
	session := response[identity.Session](c.t, c.call("GET", "session/bootstrap", nil, "", ""), 200)
	c.tokens["preauth"] = session.CSRF
	challenge := response[identity.Challenge](c.t, c.call("POST", "challenges", map[string]string{"purpose": purpose}, "preauth", ""), 200)
	return challenge.ID, solve(c.t, challenge.Image)
}

// newFulfillment 用途：创建独立 MySQL 子库并装配完整本地交易服务。
// 参数：t 测试上下文。
// 返回值：固定时钟、真实管理员登录和 HTTP 路由的隔离夹具，测试结束删除子库。
func newFulfillment(t *testing.T) *fulfillmentFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("TEST_DB_DSN required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := bootstrap.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "jst_fulfillment_" + hex.EncodeToString(random[:])
	if _, err = parent.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := bootstrap.OpenDB(cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); parent.Exec("DROP DATABASE `" + name + "`"); parent.Close() })
	ctx := context.Background()
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	f := &fulfillmentFixture{db: db, now: time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC)}
	key := bytes.Repeat([]byte{12}, 32)
	f.auth, err = identity.New(db, key, key, key)
	if err != nil {
		t.Fatal(err)
	}
	f.auth.Now = func() time.Time { return f.now }
	password := "Fixture-" + hex.EncodeToString(random[:]) + "-123"
	if err = f.auth.CreateInitialAdmin(ctx, "fulfillment-admin", password); err != nil {
		t.Fatal(err)
	}
	// 仅引导夹具标记初始改密完成，随后登录及复核仍走真实协议。
	if _, err = db.Exec("UPDATE admin_users SET force_password_change=0"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cat := catalog.New(db, root, key)
	stock := inventory.New(db, key)
	redeem, err := redemption.New(db, f.auth, cat, crypt.Keyring{Active: 1, Keys: map[int][]byte{1: key}}, key, key)
	if err != nil {
		t.Fatal(err)
	}
	redeem.Now, redeem.Test = f.auth.Now, true
	f.orders = order.New(db, f.auth, redeem)
	f.vouchers = voucher.New(db, root, key, key, 1)
	f.vouchers.Now = f.auth.Now
	f.carriers = shipping.New(db, key)
	f.carriers.Now = f.auth.Now
	auth := transport.IdentityHandler{Service: f.auth, Origin: "http://localhost:8080", Development: true}
	api := transport.API(auth, cat, stock)
	api = transport.WithVouchers(f.vouchers, auth, api)
	api = transport.WithRedemption(redeem, auth, api)
	api = transport.WithOrders(f.orders, auth, api)
	api = transport.WithShipping(f.carriers, f.orders, auth, api)
	operating := operations.New(db, f.auth, redeem.PII, key)
	operating.Now, operating.Test, operating.FileRoot = f.auth.Now, true, root
	operating.FileKeys = crypt.Keyring{Active: 1, Keys: f.vouchers.ExportKeys}
	operating.Orders, operating.Vouchers = f.orders, f.vouchers
	f.operations = operating
	api = transport.WithOperations(operating, auth, api)
	f.handler = contractHTTP(t, transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), api))
	f.client = &fulfillmentClient{t: t, handler: f.handler, cookies: map[string]*http.Cookie{}, tokens: map[string]string{}}
	cid, answer := f.client.challenge("LOGIN")
	session := response[identity.Session](t, f.client.call("POST", "admin/session", identity.Credentials{Login: "fulfillment-admin", Password: password, ChallengeID: cid, Answer: answer}, "preauth", ""), 200)
	f.client.tokens["admin"] = session.CSRF
	reauth := response[identity.Session](t, f.client.call("POST", "admin/session/reauth", map[string]string{"password": password}, "admin", ""), 200)
	f.client.tokens["admin"] = reauth.CSRF
	f.admin, err = f.auth.Authenticate(ctx, f.client.cookies["jst-admin"].Value, "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// seedFulfillment 用途：通过业务服务创建带主图商品、批准入库和真实生成封存卡。
// 参数：t 测试上下文；quantity 卡券数量。
// 返回值：无，发行凭证仅保留于测试进程内存。
func (f *fulfillmentFixture) seedFulfillment(t *testing.T, quantity int) {
	t.Helper()
	ctx := context.Background()
	cat := f.orders.Redemption.Catalog
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	file, err := cat.Upload(ctx, f.admin, imageBytes.Bytes(), "e2-image-fixture-0001", "test")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		product, err := cat.Create(ctx, f.admin, catalog.Input{Code: fmt.Sprintf("E2-%d", index), Name: fmt.Sprintf("合成商品%d", index), Attributes: []catalog.Attribute{}, MediaIDs: []string{file.ID}, Excluded: []string{}}, fmt.Sprintf("e2-product-%06d", index), "test")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = cat.Status(ctx, f.admin, product.ID, catalog.StatusInput{Version: product.Version, Status: "ENABLED"}, fmt.Sprintf("e2-enable-%06d", index), "test"); err != nil {
			t.Fatal(err)
		}
		f.products = append(f.products, product.ID)
	}
	stock := inventory.New(f.db, bytes.Repeat([]byte{12}, 32))
	lines := []inventory.Line{}
	for _, id := range f.products {
		lines = append(lines, inventory.Line{ProductID: id, Quantity: 10})
	}
	doc, err := stock.Create(ctx, f.admin, inventory.Input{Direction: "IN", Kind: "PHYSICAL", Items: lines, Note: "隔离测试"}, "e2-stock-create-0001", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stock.Review(ctx, f.admin, doc.ID, inventory.Review{Version: doc.Version, Reason: "测试入库"}, true, "e2-stock-approve-001", "test"); err != nil {
		t.Fatal(err)
	}
	typ, err := f.vouchers.CreateType(ctx, f.admin, voucher.TypeInput{Name: "选二", Mode: "N_OF", Count: 2}, "e2-type-create-00001", "test")
	if err != nil {
		t.Fatal(err)
	}
	batch, err := f.vouchers.CreateBatch(ctx, f.admin, voucher.Input{TypeID: typ.ID, Label: "E2隔离测试", Products: f.products, Quantity: quantity, Prefix: "JT", CodeLength: 12, CodeMode: "SEQUENTIAL", Start: "1", SecretLength: 12, SecretFormat: "MIXED", From: f.now.Add(-time.Hour), Until: f.now.Add(30 * 24 * time.Hour)}, "e2-batch-create-001", "test")
	if err != nil {
		t.Fatal(err)
	}
	batch, err = f.vouchers.Generate(ctx, f.admin, batch.ID, batch.Version, "e2-batch-generate-01", "test")
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := f.vouchers.RunOnce(ctx); err != nil || !worked {
		t.Fatal("generation", err)
	}
	batch, err = f.vouchers.GetBatch(ctx, batch.ID)
	if err != nil || batch.Generated != quantity || batch.State != "GENERATED" {
		t.Fatal("batch state", err)
	}
	w := f.client.call("POST", "admin/files/"+batch.FileID+"/download", nil, "admin", "")
	if w.Code != 200 {
		t.Fatal("download status", w.Code)
	}
	rows, err := csv.NewReader(bytes.NewReader(w.Body.Bytes())).ReadAll()
	if err != nil || len(rows) != quantity+1 {
		t.Fatal("credential rows", err)
	}
	f.cards = rows[1:]
	if _, err = f.vouchers.SealBatch(ctx, f.admin, batch.ID, voucher.VersionReason{Version: batch.Version, Reason: "发行测试"}, "e2-batch-seal-0001", "test"); err != nil {
		t.Fatal(err)
	}
	if w = f.client.call("POST", "admin/files/"+batch.FileID+"/download", nil, "admin", ""); w.Code != 410 {
		t.Fatal("sealed download", w.Code)
	}
	_, inactive := f.customer(t, 0)
	if inactive.Scope != "READ" {
		t.Fatal("inactive card acquired WRITE")
	}
	if _, err = f.orders.Redemption.Redeem(ctx, inactive, f.input(), "e2-inactive-redeem1", "test"); err == nil {
		t.Fatal("inactive card redeemed")
	} else {
		requireFault(t, err, "SCOPE_REQUIRED")
	}
	page, err := f.vouchers.Vouchers(ctx, voucher.Filter{Batch: batch.ID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	ids, versions := []string{}, []int64{}
	for _, card := range page.Items {
		if card.Active {
			t.Fatal("premature activation")
		}
		ids = append(ids, card.ID)
		versions = append(versions, card.Version)
	}
	if _, err = f.vouchers.Bulk(ctx, f.admin, voucher.BulkInput{IDs: ids, Versions: versions, Action: "ACTIVATE"}, "e2-bulk-activate-001", "test"); err != nil {
		t.Fatal(err)
	}
}

// customer 用途：为指定合成卡建立真实顾客 WRITE 会话。
// 参数：t 测试上下文；index 内存凭证行序号。
// 返回值：独立 HTTP 客户端及卡券主体，失败不打印凭证。
func (f *fulfillmentFixture) customer(t *testing.T, index int) (*fulfillmentClient, identity.Actor) {
	t.Helper()
	c := &fulfillmentClient{t: t, handler: f.handler, cookies: map[string]*http.Cookie{}, tokens: map[string]string{}}
	cid, answer := c.challenge("VOUCHER")
	value := response[redemption.Verification](t, c.call("POST", "voucher-sessions", redemption.Credentials{Code: f.cards[index][1], Secret: f.cards[index][2], ChallengeID: cid, Answer: answer}, "preauth", ""), 200)
	c.tokens["voucher"] = value.CSRF
	actor, err := f.auth.Authenticate(context.Background(), c.cookies["jst-voucher"].Value, "VOUCHER")
	if err != nil {
		t.Fatal(err)
	}
	return c, actor
}

// input 用途：构造不含真实个人资料的选二配送命令。
// 参数：无，使用夹具商品和固定明天日期。
// 返回值：完整兑换命令。
func (f *fulfillmentFixture) input() redemption.Input {
	return redemption.Input{Policy: "selection-v1", Items: []redemption.Selection{{ProductID: f.products[0], Quantity: 1}, {ProductID: f.products[1], Quantity: 1}}, Address: redemption.Address{Name: "测试收件人", Phone: "10000000000", Province: "110000", City: "110100", District: "110101", Detail: "合成测试地址，请勿寄送"}, Date: "2026-10-13", Note: "测试"}
}

// checkFulfillmentLedger 用途：核对库存账、活动预占与真实余额及主运单唯一性。
// 参数：t 测试上下文；db 隔离数据库。
// 返回值：无，发现任何不一致时失败。
func checkFulfillmentLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	queries := []string{
		"SELECT COUNT(*) FROM inventory_balances b WHERE b.on_hand<0 OR b.reserved<0 OR b.on_hand<b.reserved OR b.on_hand<>(SELECT COALESCE(SUM(on_hand_delta),0) FROM inventory_transactions WHERE product_id=b.product_id) OR b.reserved<>(SELECT COALESCE(SUM(reserved_delta),0) FROM inventory_transactions WHERE product_id=b.product_id) OR b.reserved<>(SELECT COALESCE(SUM(quantity),0) FROM inventory_reservations WHERE product_id=b.product_id AND state='ACTIVE')",
		"SELECT COUNT(*) FROM (SELECT voucher_id FROM voucher_redemptions GROUP BY voucher_id HAVING COUNT(*)>1) bad",
		"SELECT COUNT(*) FROM (SELECT order_id FROM shipments WHERE kind='PRIMARY' GROUP BY order_id HAVING COUNT(*)>1) bad",
		"SELECT COUNT(*) FROM inventory_transactions WHERE reason='shipping' AND shipment_id IS NULL",
	}
	for index, query := range queries {
		var bad int
		if err := db.QueryRow(query).Scan(&bad); err != nil || bad != 0 {
			t.Fatal("ledger check", index, bad, err)
		}
	}
}

// TestMockFulfillment 用途：验证登录、生成发行、兑换、人工发货、查询与 MOCK 签收主链。
// 参数：t 测试上下文。
// 返回值：无，使用真实 MySQL 与 HTTP Handler；不代表浏览器渲染验收。
func TestMockFulfillment(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	c, actor := f.customer(t, 0)
	ctx := context.Background()
	options := response[redemption.Options](t, c.call("GET", "redemption/options", nil, "voucher", ""), 200)
	if len(options.Products) != 3 || options.Count != 2 {
		t.Fatal("frozen selection")
	}
	calendar := response[redemption.Calendar](t, c.call("GET", "redemption/calendar", nil, "voucher", ""), 200)
	if !calendar.Items[1].Available {
		t.Fatal("tomorrow unavailable")
	}
	input := f.input()
	invalid := input
	invalid.Items = append(append([]redemption.Selection{}, input.Items...), redemption.Selection{ProductID: f.products[2], Quantity: 1})
	if w := c.call("POST", "redemptions", invalid, "voucher", "e2-invalid-select-01"); w.Code != 422 {
		t.Fatal("selection size", w.Code)
	}
	w := c.call("POST", "redemptions", input, "voucher", "e2-flow-redeem-0001")
	created := response[redemption.Order](t, w, 201)
	replayed, err := f.orders.Redemption.Redeem(ctx, actor, input, "e2-flow-redeem-0001", "test")
	if err != nil || replayed.Order.ID != created.ID {
		t.Fatal("redeem replay", err)
	}
	var onHand, reserved int
	if err = f.db.QueryRow("SELECT on_hand,reserved FROM inventory_balances WHERE product_id=?", f.products[0]).Scan(&onHand, &reserved); err != nil || onHand != 10 || reserved != 1 {
		t.Fatal("reserve", onHand, reserved, err)
	}
	var slot int
	if err = f.db.QueryRow("SELECT used_slots FROM shipping_calendars WHERE ship_date=?", input.Date).Scan(&slot); err != nil || slot != 1 {
		t.Fatal("slot", slot, err)
	}
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MOCK", Name: "测试物流", Provider: "MOCK", Enabled: true}, "admin", "e2-carrier-create-01"), 201)
	ship := order.ShipInput{Version: created.Version, CarrierID: carrier.ID, Tracking: "TEST-TRACK-001"}
	if w = f.client.call("POST", "admin/orders/"+created.ID+"/shipments", ship, "admin", "e2-flow-early-ship1"); w.Code != 409 {
		t.Fatal("early ship", w.Code)
	}
	f.now = f.now.Add(24 * time.Hour)
	// 前进时钟后重新装配有效管理员会话，避免用过期 Cookie 测试业务状态。
	if _, err = f.db.Exec("UPDATE sessions SET expires_at=?,last_seen_at=? WHERE kind='ADMIN'", f.now.Add(time.Hour), f.now); err != nil {
		t.Fatal(err)
	}
	shipment := response[order.Shipment](t, f.client.call("POST", "admin/orders/"+created.ID+"/shipments", ship, "admin", "e2-flow-ship-00001"), 200)
	if err = f.db.QueryRow("SELECT on_hand,reserved FROM inventory_balances WHERE product_id=?", f.products[0]).Scan(&onHand, &reserved); err != nil || onHand != 9 || reserved != 0 {
		t.Fatal("ship balance", onHand, reserved, err)
	}
	query := &fulfillmentClient{t: t, handler: f.handler, cookies: map[string]*http.Cookie{}, tokens: map[string]string{}}
	cid, answer := query.challenge("ORDER_QUERY")
	session := response[identity.Session](t, query.call("POST", "order-sessions", order.QueryCredentials{Identifier: created.Number, Phone: input.Address.Phone, ChallengeID: cid, Answer: answer}, "preauth", ""), 200)
	query.tokens["order"] = session.CSRF
	current := response[redemption.Order](t, query.call("GET", "order", nil, "order", ""), 200)
	if current.Status != "SHIPPED" || current.Recipient.Phone == input.Address.Phone {
		t.Fatal("query state or masking")
	}
	f.now = f.now.Add(30 * time.Second)
	adapter := mock.Tracking{DB: f.db, Now: f.auth.Now, Publish: f.orders.RecordTracking}
	if worked, err := adapter.RunOnce(ctx); err != nil || !worked {
		t.Fatal("mock transit", err)
	}
	events := response[order.TrackingPage](t, query.call("GET", "shipments/"+shipment.ID+"/events", nil, "order", ""), 200)
	if len(events.Items) != 1 || events.Items[0].Type != "IN_TRANSIT" {
		t.Fatal("transit events")
	}
	f.now = f.now.Add(30 * time.Second)
	if _, err = adapter.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	current = response[redemption.Order](t, query.call("GET", "order", nil, "order", ""), 200)
	if current.Status != "COMPLETED" {
		t.Fatal("completion")
	}
	if err = f.orders.RecordTracking(ctx, order.TrackingInput{ShipmentID: shipment.ID, ProviderID: "mock:" + shipment.ID + ":DELIVERED", Type: "DELIVERED", Description: "MOCK 模拟签收", Occurred: shipment.Shipped.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	var completions int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE event_type='order.completed'").Scan(&completions); err != nil || completions != 1 {
		t.Fatal("duplicate completion", completions, err)
	}
	if _, err = f.orders.Redemption.Redeem(ctx, actor, input, "e2-used-other-key1", "test"); err == nil {
		t.Fatal("reused voucher")
	}
	_, unused := f.customer(t, 1)
	if unused.Scope != "WRITE" {
		t.Fatal("second card not usable")
	}
	checkFulfillmentLedger(t, f.db)
}

// exerciseShippingRaces 用途：核验取消与发货争用同一订单时只能一次消费资源。
// 参数：t 测试上下文；f 隔离夹具；pending 尚未履约的订单。
// 返回值：无，合法胜者只有一个，取消后再发货必须拒绝。
func exerciseShippingRaces(t *testing.T, f *fulfillmentFixture, pending redemption.Order) {
	t.Helper()
	ctx := context.Background()
	// 只恢复测试目录的启用状态，不变更业务资源与账本。
	if _, err := f.db.Exec("UPDATE carriers SET enabled=1"); err != nil {
		t.Fatal(err)
	}
	var carrierID string
	if err := f.db.QueryRow("SELECT id FROM carriers LIMIT 1").Scan(&carrierID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := f.orders.Cancel(ctx, f.admin, pending.ID, order.VersionReason{Version: pending.Version, Reason: "并发取消"}, "e2-race-cancel-0001", "test")
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := f.orders.Ship(ctx, f.admin, pending.ID, order.ShipInput{Version: pending.Version, CarrierID: carrierID, Tracking: "RACE-TRACK-001"}, "e2-race-ship-000001", "test")
		errs <- err
	}()
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			requireFault(t, err, "VERSION_CONFLICT")
		}
	}
	if success != 1 {
		t.Fatal("ship/cancel winners", success)
	}
	current, err := f.orders.Redemption.GetOrder(ctx, pending.ID, true)
	if err != nil || current.Version != pending.Version+1 {
		t.Fatal("race order version", err)
	}
	if current.Status != "CANCELLED" && current.Status != "SHIPPED" {
		t.Fatal("race state", current.Status)
	}
	if current.Status == "SHIPPED" {
		if _, err = f.orders.Cancel(ctx, f.admin, current.ID, order.VersionReason{Version: current.Version, Reason: "已发不得取消"}, "e2-cancel-shipped01", "test"); err == nil {
			t.Fatal("cancel shipped")
		} else {
			requireFault(t, err, "ILLEGAL_TRANSITION")
		}
	} else {
		if _, err = f.orders.Ship(ctx, f.admin, current.ID, order.ShipInput{Version: current.Version, CarrierID: carrierID, Tracking: "CANCELLED-001"}, "e2-ship-cancelled01", "test"); err == nil {
			t.Fatal("ship cancelled")
		} else {
			requireFault(t, err, "ILLEGAL_TRANSITION")
		}
	}
	checkFulfillmentLedger(t, f.db)
}

// requireFault 用途：核验稳定业务错误码。
// 参数：t 测试上下文；err 实际错误；code 期望业务错误码。
// 返回值：无，缺少错误或错误码不同则失败。
func requireFault(t *testing.T, err error, code string) {
	t.Helper()
	var f *identity.Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

// TestShippingTransactions 用途：验证发货去重、碰撞回滚、人工完成与轨迹归属。
// 参数：t 测试上下文。
// 返回值：无，所有并发使用真实 MySQL 锁并核对完整库存账。
func TestShippingTransactions(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 5)
	ctx := context.Background()
	created := []redemption.Order{}
	actors := []identity.Actor{}
	for index := 0; index < 3; index++ {
		_, actor := f.customer(t, index)
		actors = append(actors, actor)
		result, err := f.orders.Redemption.Redeem(ctx, actor, f.input(), fmt.Sprintf("e2-order-fixture-%02d", index), "test")
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, result.Order)
	}
	carrier, err := f.carriers.Save(ctx, f.admin, "", shipping.CarrierInput{Code: "MANUAL", Name: "人工物流", Provider: "MANUAL", Enabled: true}, 0, "e2-manual-create-01", "test")
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(24 * time.Hour)
	for index, step := range []string{"ship:1", "ship:2", "audit", "commit"} {
		f.orders.FaultTx = func(_ context.Context, _ *sql.Tx, current string) error {
			if current == step {
				return errors.New("injected")
			}
			return nil
		}
		if _, err = f.orders.Ship(ctx, f.admin, created[0].ID, order.ShipInput{Version: 1, CarrierID: carrier.ID, Tracking: "ROLLBACK-001"}, fmt.Sprintf("e2-fault-step-%06d", index), "test"); err == nil || err.Error() != "injected" {
			t.Fatal("fault accepted", step, err)
		}
		checkFulfillmentLedger(t, f.db)
		current, err := f.orders.Redemption.GetOrder(ctx, created[0].ID, true)
		if err != nil || current.Status != "PENDING" || current.Version != 1 {
			t.Fatal("partial shipment", err)
		}
	}
	f.orders.FaultTx = nil
	var wg sync.WaitGroup
	errorsCh := make(chan error, 100)
	success := make(chan order.Shipment, 100)
	for index := 0; index < 100; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			v, e := f.orders.Ship(ctx, f.admin, created[0].ID, order.ShipInput{Version: 1, CarrierID: carrier.ID, Tracking: "MANUAL-001"}, fmt.Sprintf("e2-ship-concurrent-%03d", index), "test")
			if e == nil {
				success <- v
			} else {
				errorsCh <- e
			}
		}(index)
	}
	wg.Wait()
	close(success)
	close(errorsCh)
	if len(success) != 1 {
		t.Fatal("primary success count", len(success))
	}
	shipment := <-success
	for err := range errorsCh {
		requireFault(t, err, "VERSION_CONFLICT")
	}
	var winningKey string
	if err = f.db.QueryRow("SELECT request_key FROM idempotency_keys WHERE scope=? AND JSON_UNQUOTE(JSON_EXTRACT(result_ref,'$.id'))=?", fmt.Sprintf("admin:%d:shipOrder", f.admin.AdminID), created[0].ID).Scan(&winningKey); err != nil {
		t.Fatal(err)
	}
	replay, err := f.orders.Ship(ctx, f.admin, created[0].ID, order.ShipInput{Version: 1, CarrierID: carrier.ID, Tracking: "MANUAL-001"}, winningKey, "test")
	if err != nil || replay.ID != shipment.ID {
		t.Fatal("ship replay", err)
	}
	if _, err = f.orders.Ship(ctx, f.admin, created[1].ID, order.ShipInput{Version: 1, CarrierID: carrier.ID, Tracking: "MANUAL-001"}, "e2-track-collision01", "test"); err == nil {
		t.Fatal("tracking collision")
	} else {
		requireFault(t, err, "CONFLICT")
	}
	checkFulfillmentLedger(t, f.db)
	other, _ := strconv.ParseInt(created[1].ID, 10, 64)
	if _, err = f.orders.Events(ctx, identity.Actor{Kind: "ORDER", OrderID: other, Scope: "READ"}, shipment.ID, order.Filter{Limit: 20}); err == nil {
		t.Fatal("cross-order tracking")
	} else {
		requireFault(t, err, "NOT_FOUND")
	}
	completed, err := f.orders.Complete(ctx, f.admin, created[0].ID, order.VersionReason{Version: 2, Reason: "人工确认"}, "e2-manual-complete01", "test")
	if err != nil || completed.Status != "COMPLETED" {
		t.Fatal("manual complete", err)
	}
	events, err := f.orders.Events(ctx, f.admin, shipment.ID, order.Filter{Limit: 20})
	if err != nil || len(events.Items) != 0 {
		t.Fatal("fabricated manual tracking", err)
	}
	if _, err = f.orders.Ship(ctx, f.admin, created[1].ID, order.ShipInput{Version: 1, CarrierID: carrier.ID, Tracking: "MANUAL-002"}, "e2-second-ship-0001", "test"); err != nil {
		t.Fatal(err)
	}
	second, err := f.orders.PrimaryShipment(ctx, created[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []order.TrackingInput{
		{ShipmentID: second.ID, ProviderID: "new", Type: "IN_TRANSIT", Description: "运输", Occurred: f.now.Add(time.Hour)},
		{ShipmentID: second.ID, ProviderID: "old", Type: "EXCEPTION", Description: "迟到事件", Occurred: f.now.Add(time.Minute)},
		{ShipmentID: second.ID, ProviderID: "done", Type: "DELIVERED", Description: "签收", Occurred: f.now.Add(2 * time.Hour)},
		{ShipmentID: second.ID, ProviderID: "later", Type: "EXCEPTION", Description: "不得倒退", Occurred: f.now.Add(3 * time.Hour)},
	} {
		if err = f.orders.RecordTracking(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.orders.RecordTracking(ctx, order.TrackingInput{ShipmentID: second.ID, ProviderID: "done", Type: "DELIVERED", Description: "篡改同事件", Occurred: f.now.Add(2 * time.Hour)}); err == nil {
		t.Fatal("event content collision")
	} else {
		requireFault(t, err, "IDEMPOTENCY_CONFLICT")
	}
	second, err = f.orders.PrimaryShipment(ctx, created[1].ID)
	if err != nil || second.Status != "DELIVERED" {
		t.Fatal("tracking regressed", err)
	}
	checkFulfillmentLedger(t, f.db)
	exercisePickup(t, f)
	exerciseShippingHTTP(t, f, created[2], shipment, carrier)
}

// exercisePickup 用途：验证到店验密、权益、预占隔离、回滚与并发消费。
// 参数：t 测试上下文；f 尚有未消费卡的隔离夹具。
// 返回值：无，每项失败或事务残留都使测试失败。
func exercisePickup(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	ctx := context.Background()
	input := order.PickupInput{Code: f.cards[3][1], Secret: f.cards[3][2], Items: f.input().Items, Reason: "到店测试"}
	wrong := input
	wrong.Secret = "Wrong-Secret-123"
	if _, err := f.orders.Pickup(ctx, f.admin, wrong, "e2-pickup-wrong-001", "test"); err == nil {
		t.Fatal("wrong secret")
	} else {
		requireFault(t, err, "INVALID_CREDENTIALS")
	}
	invalid := input
	invalid.Items = input.Items[:1]
	if _, err := f.orders.Pickup(ctx, f.admin, invalid, "e2-pickup-invalid01", "test"); err == nil {
		t.Fatal("pickup selection")
	} else {
		requireFault(t, err, "SELECTION_INVALID")
	}
	// 临时出库全部可用库存，保证到店不能侵占其他配送订单的预占。
	stock := inventory.New(f.db, bytes.Repeat([]byte{12}, 32))
	var available int64
	if err := f.db.QueryRow("SELECT on_hand-reserved FROM inventory_balances WHERE product_id=?", f.products[0]).Scan(&available); err != nil {
		t.Fatal(err)
	}
	doc, err := stock.Create(ctx, f.admin, inventory.Input{Direction: "OUT", Kind: "PHYSICAL", Items: []inventory.Line{{ProductID: f.products[0], Quantity: available}}}, "e2-pickup-stock-out1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stock.Review(ctx, f.admin, doc.ID, inventory.Review{Version: doc.Version, Reason: "测试预占隔离"}, true, "e2-pickup-out-review", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.orders.Pickup(ctx, f.admin, input, "e2-pickup-shortage1", "test"); err == nil {
		t.Fatal("pickup spent reserved")
	} else {
		requireFault(t, err, "OUT_OF_STOCK")
	}
	checkFulfillmentLedger(t, f.db)
	doc, err = stock.Create(ctx, f.admin, inventory.Input{Direction: "IN", Kind: "PHYSICAL", Items: []inventory.Line{{ProductID: f.products[0], Quantity: available}}}, "e2-pickup-stock-in01", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stock.Review(ctx, f.admin, doc.ID, inventory.Review{Version: doc.Version}, true, "e2-pickup-in-review1", "test"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"pickup:1", "pickup:2", "audit", "commit"} {
		f.orders.FaultTx = func(_ context.Context, _ *sql.Tx, current string) error {
			if current == step {
				return errors.New("injected")
			}
			return nil
		}
		if _, err := f.orders.Pickup(ctx, f.admin, input, "e2-pickup-fault-0001", "test"); err == nil || err.Error() != "injected" {
			t.Fatal("pickup fault", step, err)
		}
		checkFulfillmentLedger(t, f.db)
	}
	f.orders.FaultTx = nil
	var wg sync.WaitGroup
	errorsCh := make(chan error, 10)
	success := make(chan redemption.Order, 10)
	for index := 0; index < 10; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			v, e := f.orders.Pickup(ctx, f.admin, input, fmt.Sprintf("e2-pickup-concurrent-%02d", index), "test")
			if e == nil {
				success <- v
			} else {
				errorsCh <- e
			}
		}(index)
	}
	wg.Wait()
	close(success)
	close(errorsCh)
	if len(success) != 1 {
		t.Fatal("pickup successes", len(success))
	}
	created := <-success
	for err := range errorsCh {
		requireFault(t, err, "ALREADY_REDEEMED")
	}
	if created.Status != "COMPLETED" || created.Mode != "PICKUP" || created.Date != "" {
		t.Fatal("pickup shape")
	}
	var count int
	if err := f.db.QueryRow("SELECT (SELECT COUNT(*) FROM shipments WHERE order_id=?)+(SELECT COUNT(*) FROM inventory_reservations r JOIN order_items i ON i.id=r.order_item_id WHERE i.order_id=?)", created.ID, created.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("pickup delivery resources", count, err)
	}
	checkFulfillmentLedger(t, f.db)
}

// exerciseShippingHTTP 用途：验证物流接口的权限、CSRF、筛选、版本和到店重放。
// 参数：t 测试上下文；f 隔离夹具；pending 待发订单；shipment 已发运单；carrier 承运商。
// 返回值：无，所有断言通过真实 Handler 与数据库。
func exerciseShippingHTTP(t *testing.T, f *fulfillmentFixture, pending redemption.Order, shipment order.Shipment, carrier shipping.Carrier) {
	t.Helper()
	if _, err := f.db.Exec("UPDATE sessions SET expires_at=?,last_seen_at=? WHERE kind='ADMIN'", f.now.Add(time.Hour), f.now); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"admin/carriers?limit=0", "admin/carriers?status=SHIPPED", "admin/orders/" + pending.ID + "/shipments?q=x", "admin/shipments/" + shipment.ID + "/events?status=DELIVERED", "admin/carriers?limit=1&limit=2"} {
		if w := f.client.call("GET", path, nil, "admin", ""); w.Code != 400 {
			t.Fatal("filter not rejected", path, w.Code)
		}
	}
	response[shipping.Carrier](t, f.client.call("GET", "admin/carriers/"+carrier.ID, nil, "admin", ""), 200)
	response[order.ShipmentPage](t, f.client.call("GET", "admin/orders/"+pending.ID+"/shipments", nil, "admin", ""), 200)
	oldCSRF := f.client.tokens["admin"]
	f.client.tokens["admin"] = "invalid"
	if w := f.client.call("POST", "admin/pickups", map[string]string{}, "admin", "e2-http-csrf-denied"); w.Code != 403 {
		t.Fatal("pickup CSRF", w.Code)
	}
	f.client.tokens["admin"] = oldCSRF
	update := shipping.CarrierUpdate{Version: carrier.Version, CarrierInput: carrier.CarrierInput}
	update.Enabled = false
	disabled := response[shipping.Carrier](t, f.client.call("PATCH", "admin/carriers/"+carrier.ID, update, "admin", "e2-disable-carrier01"), 200)
	if disabled.Enabled || disabled.Version != carrier.Version+1 {
		t.Fatal("carrier update")
	}
	if w := f.client.call("PATCH", "admin/carriers/"+carrier.ID, update, "admin", "e2-carrier-stale-001"); w.Code != 409 {
		t.Fatal("carrier version", w.Code)
	}
	ship := order.ShipInput{Version: pending.Version, CarrierID: carrier.ID, Tracking: "DISABLED-TRACK1"}
	if w := f.client.call("POST", "admin/orders/"+pending.ID+"/shipments", ship, "admin", "e2-disabled-ship-001"); w.Code != 422 {
		t.Fatal("disabled carrier", w.Code)
	}
	// 管理员只保留读权限时，前端按钮隐藏之外仍由服务端拒绝写入。
	if _, err := f.db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='carrier.write'"); err != nil {
		t.Fatal(err)
	}
	if w := f.client.call("PATCH", "admin/carriers/"+carrier.ID, update, "admin", "e2-carrier-forbidden"); w.Code != 403 {
		t.Fatal("carrier permission", w.Code)
	}
	cid, answer := f.client.challenge("ORDER_QUERY")
	session := response[identity.Session](t, f.client.call("POST", "order-sessions", order.QueryCredentials{Identifier: pending.Number, Phone: f.input().Address.Phone, ChallengeID: cid, Answer: answer}, "preauth", ""), 200)
	f.client.tokens["order"] = session.CSRF
	if w := f.client.call("GET", "shipments/"+shipment.ID+"/events", nil, "order", ""); w.Code != 404 {
		t.Fatal("HTTP cross-order tracking", w.Code)
	}
	input := order.PickupInput{Code: f.cards[4][1], Secret: f.cards[4][2], Items: f.input().Items, Reason: "HTTP到店"}
	created := response[redemption.Order](t, f.client.call("POST", "admin/pickups", input, "admin", "e2-pickup-http-0001"), 200)
	replay := response[redemption.Order](t, f.client.call("POST", "admin/pickups", input, "admin", "e2-pickup-http-0001"), 200)
	if created.ID != replay.ID {
		t.Fatal("HTTP pickup replay")
	}
	checkFulfillmentLedger(t, f.db)
	exerciseShippingRaces(t, f, pending)
}
