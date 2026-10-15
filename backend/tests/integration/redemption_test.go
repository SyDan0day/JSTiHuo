package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/bootstrap"
	"jstihuo/internal/infra/businessid"
	crypt "jstihuo/internal/infra/crypto"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRedemption 用途：验证鉴真、地区、日历、PII 和兑换并发/故障原子性。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，集成用例只连接显式指定的隔离测试库并清理子库。
func TestRedemption(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("TEST_DB_DSN required")
	}
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	parent, e := bootstrap.OpenDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer parent.Close()
	var suffix [8]byte
	rand.Read(suffix[:])
	name := "jst_redemption_" + hex.EncodeToString(suffix[:])
	if _, e = parent.Exec("CREATE DATABASE `" + name + "`"); e != nil {
		t.Fatal(e)
	}
	defer parent.Exec("DROP DATABASE `" + name + "`")
	cfg.DBName = name
	db, e := bootstrap.OpenDB(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	key := bytes.Repeat([]byte{9}, 32)
	now := time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC)
	auth, e := identity.New(db, key, key, key)
	if e != nil {
		t.Fatal(e)
	}
	auth.Now = func() time.Time { return now }
	svc, e := redemption.New(db, auth, catalog.New(db, t.TempDir(), key), crypt.Keyring{Active: 1, Keys: map[int][]byte{1: key}}, key, key)
	if e != nil {
		t.Fatal(e)
	}
	svc.Now = auth.Now
	svc.Test = true
	// 五张合成卡共享不可变 N_OF=2 权益；第五张用于真实业务号冲突恢复。
	r, e := db.Exec("INSERT INTO admin_users(login_name,password_hash,force_password_change) VALUES('fixture-admin','unused',0)")
	if e != nil {
		t.Fatal(e)
	}
	aid, _ := r.LastInsertId()
	imageID := seedRedemptionImage(t, db, svc.Catalog.FileRoot, aid)
	r, e = db.Exec("INSERT INTO voucher_types(name,selection_mode,selection_count) VALUES('选二','N_OF',2)")
	if e != nil {
		t.Fatal(e)
	}
	tid, _ := r.LastInsertId()
	products := []string{}
	for i := 0; i < 3; i++ {
		r, e = db.Exec("INSERT INTO products(code,name,attributes,description_html,excluded_region_codes,status) VALUES(?,?,JSON_ARRAY(),'',JSON_ARRAY(),'ENABLED')", fmt.Sprintf("RD-%d", i), fmt.Sprintf("合成商品 %d", i))
		if e != nil {
			t.Fatal(e)
		}
		id, _ := r.LastInsertId()
		if _, e = db.Exec("INSERT INTO product_media(product_id,file_id,position) VALUES(?,?,0)", id, imageID); e != nil {
			t.Fatal(e)
		}
		products = append(products, fmt.Sprint(id))
		if _, e = db.Exec("INSERT INTO inventory_transactions(product_id,on_hand_delta,event_key,reason) VALUES(?,10,?,'fixture')", id, fmt.Sprintf("fixture:%d", id)); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("INSERT INTO inventory_balances(product_id,on_hand) VALUES(?,10)", id); e != nil {
			t.Fatal(e)
		}
	}
	policy, _ := json.Marshal(map[string]any{"type_id": fmt.Sprint(tid), "revision": 1, "selection_mode": "N_OF", "selection_count": 2, "product_ids": products})
	r, e = db.Exec("INSERT INTO voucher_batches(batch_no,type_id,label,policy_snapshot,quantity,code_prefix,valid_from,valid_until,state,created_by) VALUES('B_REDEMPTION_TEST',?,'test',?,5,'JT',?,?,'SEALED',?)", tid, policy, now.Add(-time.Hour), now.Add(30*24*time.Hour), aid)
	if e != nil {
		t.Fatal(e)
	}
	bid, _ := r.LastInsertId()
	secret := "TESTONLY1234"
	hash, e := crypt.Hash(secret)
	if e != nil {
		t.Fatal(e)
	}
	cardIDs := []string{}
	for i := 0; i < 5; i++ {
		r, e = db.Exec("INSERT INTO vouchers(batch_id,code,secret_hash,credential_state,active,valid_from,valid_until) VALUES(?,?,?,'SEALED',1,?,?)", bid, fmt.Sprintf("JTTEST%010d", i), hash, now.Add(-time.Hour), now.Add(30*24*time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		vid, _ := r.LastInsertId()
		cardIDs = append(cardIDs, fmt.Sprint(vid))
		for _, p := range products {
			if _, e = db.Exec("INSERT INTO voucher_products(voucher_id,product_id) VALUES(?,?)", vid, p); e != nil {
				t.Fatal(e)
			}
		}
	}
	authHandler := transport.IdentityHandler{Service: auth, Origin: "http://localhost:8080", Development: true}
	h := contractHTTP(t, transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.WithOrders(order.New(db, auth, svc), authHandler, transport.WithRedemption(svc, authHandler, transport.API(authHandler, svc.Catalog)))))
	call := func(method, path string, body any, token, csrf, kind, key string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		req := httptest.NewRequest(method, "/api/v1/"+path, &b)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:8080")
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Idempotency-Key", key)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "jst-" + kind, Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	verify := func(i int) (identity.Actor, redemption.VerifyResult) {
		pre, e := auth.Bootstrap(ctx)
		if e != nil {
			t.Fatal(e)
		}
		a, e := auth.Authenticate(ctx, pre.Token, "PREAUTH")
		if e != nil {
			t.Fatal(e)
		}
		challenge, e := auth.Challenge(ctx, a, "VOUCHER", fmt.Sprintf("fixture-ip-%d", i))
		if e != nil {
			t.Fatal(e)
		}
		in := redemption.Credentials{Code: fmt.Sprintf(" jttest%010d ", i), Secret: secret, ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}
		v, e := svc.Verify(ctx, a, in, fmt.Sprintf("fixture-ip-%d", i), "test")
		if e != nil {
			t.Fatal(e)
		}
		va, e := auth.Authenticate(ctx, v.Session.Token, "VOUCHER")
		if e != nil {
			t.Fatal(e)
		}
		if va.Scope != "WRITE" {
			t.Fatal(va)
		}
		if _, e = auth.Authenticate(ctx, pre.Token, "PREAUTH"); e == nil {
			t.Fatal("preauth not consumed")
		}
		return va, v
	}
	actors := []identity.Actor{}
	tokens := []redemption.VerifyResult{}
	for i := 0; i < 5; i++ {
		a, v := verify(i)
		actors = append(actors, a)
		tokens = append(tokens, v)
	}
	if w := call("GET", "regions", nil, "", "", "", " "); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call("GET", "regions?parent_code=000001", nil, "", "", "", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}

	// Real HTTP options restore scoped CSRF, and a wrong CSRF fails before writes.
	optionHTTP := call("GET", "redemption/options", nil, tokens[0].Session.Token, "", "voucher", "")
	if optionHTTP.Code != 200 || optionHTTP.Header().Get("X-CSRF-Scope") != "voucher" {
		t.Fatal("options transport", optionHTTP.Code, optionHTTP.Body.String())
	}
	tokens[0].Session.Session.CSRF = optionHTTP.Header().Get("X-CSRF-Token")
	if w := call("POST", "redemptions", map[string]string{}, tokens[0].Session.Token, "wrong-csrf", "voucher", "http-csrf-invalid-01"); w.Code != 403 {
		t.Fatal("CSRF gate", w.Code)
	}
	option, e := svc.Options(ctx, actors[0])
	if e != nil || len(option.Products) != 3 || option.Count != 2 {
		t.Fatal(option, e)
	}
	cal, e := svc.Calendar(ctx, actors[0])
	if e != nil || len(cal.Items) != 31 || cal.Items[0].Available || !cal.Items[1].Available {
		t.Fatal(cal, e)
	}
	var consumed, orders, reserved int
	db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE redeemed_at IS NOT NULL").Scan(&consumed)
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orders)
	db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reserved)
	if consumed+orders+reserved != 0 {
		t.Fatal("preview mutated business")
	}
	input := redemption.Input{Policy: "selection-v1", Items: []redemption.Selection{{ProductID: products[1], Quantity: 1}, {ProductID: products[0], Quantity: 1}}, Address: redemption.Address{Name: " 测试收件人 ", Phone: "10000000000", Province: "110000", City: "110100", District: "110101", Detail: " 合成测试地址，请勿寄送 "}, Date: "2026-10-13", Note: "synthetic"}
	// Every mandatory transaction boundary is faulted before any durable publish.
	for _, step := range []string{"order", "reserve:1", "reserve:2", "calendar", "outbox", "commit"} {
		svc.Fault = func(current string) error {
			if current == step {
				return errors.New("injected failure")
			}
			return nil
		}
		if _, e = svc.Redeem(ctx, actors[0], input, "fault-injection-0001", "test"); e == nil {
			t.Fatal("fault ignored", step)
		}
		db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orders)
		db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE redeemed_at IS NOT NULL").Scan(&consumed)
		db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reserved)
		var slots, events, keys int
		db.QueryRow("SELECT SUM(used_slots) FROM shipping_calendars").Scan(&slots)
		db.QueryRow("SELECT COUNT(*) FROM outbox_events").Scan(&events)
		db.QueryRow("SELECT COUNT(*) FROM idempotency_keys").Scan(&keys)
		if orders+consumed+reserved+slots+events+keys != 0 {
			t.Fatal("partial fault", step)
		}
	}
	svc.Fault = nil
	svc.FaultTx = func(ctx context.Context, tx *sql.Tx, step string) error {
		if step != "commit" {
			return nil
		}
		var connectionID int64
		if e := tx.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connectionID); e != nil {
			return e
		}
		_, e := db.ExecContext(ctx, fmt.Sprintf("KILL CONNECTION %d", connectionID))
		return e
	}
	if _, e = svc.Redeem(ctx, actors[0], input, "disconnect-before-commit", "test"); e == nil {
		t.Fatal("disconnect ignored")
	}
	svc.FaultTx = nil
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orders)
	db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reserved)
	if orders+reserved != 0 {
		t.Fatal("disconnect partially committed")
	}

	// Exact quantities, foreign selections, hierarchy, stock, dates and regions.
	invalid := input
	invalid.Items = []redemption.Selection{{ProductID: products[0], Quantity: 1}}
	if _, e = svc.Redeem(ctx, actors[0], invalid, "invalid-selection-001", "test"); e == nil {
		t.Fatal("N_OF underselect")
	}
	invalid = input
	invalid.Items = []redemption.Selection{{ProductID: products[0], Quantity: 2}, {ProductID: products[1], Quantity: 1}}
	if _, e = svc.Redeem(ctx, actors[0], invalid, "invalid-selection-002", "test"); e == nil {
		t.Fatal("quantity two")
	}
	invalid = input
	invalid.Address.City = "310100"
	if _, e = svc.Redeem(ctx, actors[0], invalid, "invalid-address-0001", "test"); e == nil {
		t.Fatal("hierarchy")
	}
	invalid = input
	invalid.Date = "2026-10-12"
	if _, e = svc.Redeem(ctx, actors[0], invalid, "invalid-date-000001", "test"); e == nil {
		t.Fatal("past date")
	}
	db.Exec("UPDATE products SET excluded_region_codes=JSON_ARRAY('110000') WHERE id=?", products[0])
	if _, e = svc.Redeem(ctx, actors[0], input, "invalid-region-0001", "test"); e == nil {
		t.Fatal("region restriction")
	}
	db.Exec("UPDATE products SET excluded_region_codes=JSON_ARRAY() WHERE id=?", products[0])
	var wg sync.WaitGroup
	results := make(chan redemption.RedeemResult, 100)
	errs := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := svc.Redeem(ctx, actors[0], input, "same-key-100-replays", "test")
			if e != nil {
				errs <- e
			} else {
				results <- v
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		t.Fatal("concurrent replay", e)
	}
	var original redemption.RedeemResult
	for result := range results {
		if original.Order.ID != "" && original.Order.ID != result.Order.ID {
			t.Fatal("two orders")
		}
		original = result
	}
	if original.Order.ID == "" {
		t.Fatal("no orders")
	}

	// Simulate a lost success response: POST the original command/key after consumption.
	recoveredHTTP := call("POST", "redemptions", input, tokens[0].Session.Token, tokens[0].Session.Session.CSRF, "voucher", "same-key-100-replays")
	if recoveredHTTP.Code != 201 || recoveredHTTP.Header().Get("X-CSRF-Scope") != "order" || len(recoveredHTTP.Result().Cookies()) == 0 {
		t.Fatal("HTTP success recovery", recoveredHTTP.Code, recoveredHTTP.Body.String())
	}
	var recoveredOrder redemption.Order
	if e = json.Unmarshal(recoveredHTTP.Body.Bytes(), &recoveredOrder); e != nil || recoveredOrder.ID != original.Order.ID {
		t.Fatal("HTTP replay reference", e)
	}
	// Distinct keys on one second card result in one consumption and 99 conflicts.
	successes := make(chan redemption.RedeemResult, 100)
	failures := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, e := svc.Redeem(ctx, actors[1], input, fmt.Sprintf("distinct-key-%06d", i), "test")
			if e != nil {
				failures <- e
			} else {
				successes <- v
			}
		}(i)
	}
	wg.Wait()
	if len(successes) != 1 || len(failures) != 99 {
		t.Fatal("single consumption", len(successes), len(failures))
	}
	close(failures)
	for e := range failures {
		var f *identity.Fault
		if !errors.As(e, &f) || f.Code != "ALREADY_REDEEMED" {
			t.Fatal(e)
		}
	}
	db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orders)
	db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reserved)
	if orders != 2 || reserved != 4 {
		t.Fatal("balance", orders, reserved)
	}
	var cipher []byte
	var phoneHash []byte
	db.QueryRow("SELECT recipient_cipher,phone_lookup FROM orders WHERE id=?", original.Order.ID).Scan(&cipher, &phoneHash)
	bareHash := sha256.Sum256([]byte("10000000000"))
	if bytes.Contains(cipher, []byte(input.Address.Phone)) || bytes.Contains(cipher, []byte(input.Address.Name)) || bytes.Equal(phoneHash, bareHash[:]) {
		t.Fatal("PII plaintext/lookup")
	}
	if original.Order.Recipient.Phone != "100****0000" || strings.Contains(mustJSON(t, original.Order), input.Address.Detail) {
		t.Fatal("unmasked order")
	}
	if w := call("GET", "order", nil, original.Session.Token, original.Session.Session.CSRF, "order", ""); w.Code != 200 || w.Header().Get("X-CSRF-Scope") != "order" {
		t.Fatal(w.Code, w.Body.String())
	}

	// The completed D2 path must expose real pending orders to authorized admins.
	roleResult, e := db.Exec("INSERT INTO roles(name) VALUES('Order reader')")
	if e != nil {
		t.Fatal(e)
	}
	roleID, e := roleResult.LastInsertId()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO permissions(permission_key,description) VALUES('order.read','order.read')"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO role_permissions(role_id,permission_id) SELECT ?,id FROM permissions WHERE permission_key='order.read'", roleID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO admin_user_roles(admin_id,role_id) VALUES(?,?)", aid, roleID); e != nil {
		t.Fatal(e)
	}
	adminToken, adminCSRF := "redemption-admin-session", "redemption-admin-csrf-token"
	adminTokenHash, adminCSRFHash := sha256.Sum256([]byte(adminToken)), sha256.Sum256([]byte(adminCSRF))
	if _, e = db.Exec("INSERT INTO sessions(token_hash,kind,scope,admin_id,subject_version,csrf_hash,expires_at,last_seen_at) VALUES(?,'ADMIN','WRITE',?,1,?,?,?)", adminTokenHash[:], aid, adminCSRFHash[:], now.Add(time.Hour), now); e != nil {
		t.Fatal(e)
	}
	adminHTTP := call("GET", "admin/orders?status=PENDING&limit=1", nil, adminToken, "", "admin", "")
	if adminHTTP.Code != 200 {
		t.Fatal("admin pending orders", adminHTTP.Code, adminHTTP.Body.String())
	}
	var adminPage order.Page
	if e = json.Unmarshal(adminHTTP.Body.Bytes(), &adminPage); e != nil || len(adminPage.Items) != 1 || adminPage.Next == "" {
		t.Fatal("pending pagination", adminPage, e)
	}
	if w := call("GET", "admin/orders/"+original.Order.ID, nil, adminToken, "", "admin", ""); w.Code != 200 || strings.Contains(w.Body.String(), input.Address.Detail) {
		t.Fatal("admin masked detail", w.Code)
	}
	if w := call("GET", "admin/orders?deleted=1", nil, adminToken, "", "admin", ""); w.Code != 400 {
		t.Fatal("deleted filter", w.Code)
	}
	// Date capacity and stock race over two remaining cards allow only one order.
	db.Exec("UPDATE shipping_calendars SET capacity=3 WHERE ship_date='2026-10-13'")
	db.Exec("UPDATE inventory_balances SET on_hand=reserved+1 WHERE product_id IN (?,?)", products[0], products[1])
	raceResults := make(chan error, 2)
	for i := 2; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := svc.Redeem(ctx, actors[i], input, fmt.Sprintf("scarce-stock-%06d", i), "test")
			raceResults <- e
		}(i)
	}
	wg.Wait()
	close(raceResults)
	ok, failed := 0, 0
	for e := range raceResults {
		if e == nil {
			ok++
		} else {
			failed++
		}
	}
	if ok != 1 || failed != 1 {
		t.Fatal("scarcity", ok, failed)
	}
	var invalidBalances int
	db.QueryRow("SELECT COUNT(*) FROM inventory_balances WHERE reserved>on_hand OR reserved<0").Scan(&invalidBalances)
	if invalidBalances != 0 {
		t.Fatal("negative availability")
	}
	var reconciled int
	db.QueryRow("SELECT COUNT(*) FROM inventory_balances b WHERE b.reserved<>(SELECT COALESCE(SUM(quantity),0) FROM inventory_reservations r WHERE r.product_id=b.product_id AND r.state='ACTIVE')").Scan(&reconciled)
	if reconciled != 0 {
		t.Fatal("reservation mismatch")
	}

	// Assert stock and capacity independently with the card that lost the race.
	remaining := actors[2]
	var redeemed bool
	db.QueryRow("SELECT redeemed_at IS NOT NULL FROM vouchers WHERE id=?", remaining.VoucherID).Scan(&redeemed)
	if redeemed {
		remaining = actors[3]
	}
	if _, e = svc.Redeem(ctx, remaining, input, "capacity-overflow-0001", "test"); e == nil {
		t.Fatal("capacity overflow")
	} else {
		var f *identity.Fault
		if !errors.As(e, &f) || f.Code != "CAPACITY_EXCEEDED" {
			t.Fatal(e)
		}
	}
	nextDay := input
	nextDay.Date = "2026-10-14"
	if _, e = svc.Redeem(ctx, remaining, nextDay, "stock-overflow-000001", "test"); e == nil {
		t.Fatal("stock overflow")
	} else {
		var f *identity.Fault
		if !errors.As(e, &f) || f.Code != "OUT_OF_STOCK" {
			t.Fatal(e)
		}
	}
	// The card's expiry does not constrain a later appointment; product expiry does.
	db.Exec("UPDATE vouchers SET valid_until=? WHERE id=?", now.Add(2*time.Hour), remaining.VoucherID)
	db.Exec("UPDATE products SET redeem_until=? WHERE id=?", now.Add(24*time.Hour), products[0])
	if _, e = svc.Redeem(ctx, remaining, nextDay, "product-date-invalid", "test"); e == nil {
		t.Fatal("product date limit")
	} else {
		var f *identity.Fault
		if !errors.As(e, &f) || f.Code != "DATE_UNAVAILABLE" {
			t.Fatal(e)
		}
	}
	db.Exec("UPDATE products SET redeem_until=NULL WHERE id=?", products[0])
	db.Exec("UPDATE inventory_balances SET on_hand=on_hand+1 WHERE product_id IN (?,?)", products[0], products[1])
	if _, e = svc.Redeem(ctx, remaining, nextDay, "appointment-after-card", "test"); e != nil {
		t.Fatal("appointment after card expiry", e)
	}
	// Consumed card credentials can still show authentic state, but receive READ only.
	pre, e := auth.Bootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	pa, e := auth.Authenticate(ctx, pre.Token, "PREAUTH")
	if e != nil {
		t.Fatal(e)
	}
	challenge, e := auth.Challenge(ctx, pa, "VOUCHER", "consumed-auth")
	if e != nil {
		t.Fatal(e)
	}
	verified, e := svc.Verify(ctx, pa, redemption.Credentials{Code: "JTTEST0000000000", Secret: secret, ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}, "consumed-auth", "test")
	if e != nil || !verified.Value.Redeemed {
		t.Fatal(verified.Value, e)
	}
	readActor, e := auth.Authenticate(ctx, verified.Session.Token, "VOUCHER")
	if e != nil || readActor.Scope != "READ" {
		t.Fatal(readActor, e)
	}
	if _, e = svc.Redeem(ctx, readActor, input, "read-cannot-redeem-001", "test"); e == nil {
		t.Fatal("READ scope redeemed")
	}

	// Force an actual order_no unique-index collision and verify whole-transaction retry.
	originalNumberSource := svc.Number
	numberCalls := 0
	svc.Number = func(clock time.Time) (string, error) {
		numberCalls++
		if numberCalls == 1 {
			return original.Order.Number, nil
		}
		return businessid.New("O_", clock)
	}
	if _, e = db.Exec("UPDATE inventory_balances SET on_hand=on_hand+1 WHERE product_id IN (?,?)", products[0], products[1]); e != nil {
		t.Fatal(e)
	}
	uniqueInput := input
	uniqueInput.Date = "2026-10-15"
	collisionOrder, e := svc.Redeem(ctx, actors[4], uniqueInput, "business-number-retry", "test")
	svc.Number = originalNumberSource
	if e != nil || numberCalls != 2 || collisionOrder.Order.Number == original.Order.Number {
		t.Fatal("business number collision", numberCalls, e)
	}
	adminActor, e := auth.Authenticate(ctx, adminToken, "ADMIN")
	if e != nil {
		t.Fatal(e)
	}
	exerciseOrderManagement(t, db, order.New(db, auth, svc), adminActor, func(clock time.Time) { now = clock })
	_ = cardIDs
}

// seedRedemptionImage 用途：补齐隔离兑换夹具的真实自有主图与文件元数据。
// 参数：t 测试上下文；db 隔离库；root 临时文件根；admin 图片所有者。
// 返回值：READY 图片 ID；失败终止测试，不放宽公开商品契约。
func seedRedemptionImage(t *testing.T, db *sql.DB, root string, admin int64) int64 {
	t.Helper()
	raw, err := os.ReadFile("../../../tests/fixtures/own-gift.png")
	if err != nil {
		t.Fatal(err)
	}
	name := "0123456789abcdef0123456789abcdef.png"
	if err = os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	result, err := db.Exec("INSERT INTO files(storage_key,kind,mime_type,size_bytes,sha256,state,owner_admin_id) VALUES(?,'IMAGE','image/png',?,?,'READY',?)", name, len(raw), sum[:], admin)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
