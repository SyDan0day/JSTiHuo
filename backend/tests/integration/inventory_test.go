package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"jstihuo/internal/bootstrap"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInventory(t *testing.T) {
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
	name := "jst_inventory_" + hex.EncodeToString(suffix[:])
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
	key := bytes.Repeat([]byte{7}, 32)
	auth, e := identity.New(db, key, key, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.CreateInitialAdmin(ctx, "inventory-admin", "Inventory-Testing-123"); e != nil {
		t.Fatal(e)
	}
	db.Exec("UPDATE admin_users SET force_password_change=0")
	var aid int64
	db.QueryRow("SELECT id FROM admin_users").Scan(&aid)
	token := "inventory-session-for-tests"
	csrf := "inventory-csrf-test-abcdefghijk"
	th := sha256.Sum256([]byte(token))
	ch := sha256.Sum256([]byte(csrf))
	if _, e = db.Exec("INSERT INTO sessions(token_hash,kind,scope,admin_id,subject_version,csrf_hash,expires_at,last_seen_at) VALUES(?,'ADMIN','WRITE',?,1,?,?,?)", th[:], aid, ch[:], time.Now().Add(time.Hour), time.Now()); e != nil {
		t.Fatal(e)
	}
	svc := inventory.New(db, key)
	h := transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.API(transport.IdentityHandler{Service: auth, Origin: "http://localhost:8080", Development: true}, catalog.New(db, t.TempDir(), key), svc))
	call := func(method, path string, payload any, key string, authorized bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if payload != nil {
			json.NewEncoder(&b).Encode(payload)
		}
		r := httptest.NewRequest(method, "/api/v1/admin/inventory"+path, &b)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Idempotency-Key", key)
		if authorized {
			r.AddCookie(&http.Cookie{Name: "jst-admin", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d expected %d: %s", w.Code, status, w.Body.String())
		}
	}
	counter := 0
	newKey := func() string { counter++; return fmt.Sprintf("inventory-test-key-%06d", counter) }
	for i := 1; i <= 3; i++ {
		r, e := db.Exec("INSERT INTO products(code,name,attributes,description_html,excluded_region_codes) VALUES(?,?,JSON_ARRAY(),'text',JSON_ARRAY())", fmt.Sprintf("I%d", i), fmt.Sprintf("库存商品%d", i))
		if e != nil {
			t.Fatal(e)
		}
		id, _ := r.LastInsertId()
		if _, e = db.Exec("INSERT INTO inventory_balances(product_id) VALUES(?)", id); e != nil {
			t.Fatal(e)
		}
	}
	create := func(direction string, lines []inventory.Line) inventory.Document {
		t.Helper()
		w := call("POST", "/documents", inventory.Input{Direction: direction, Kind: "PHYSICAL", Items: lines, Note: "合成测试"}, newKey(), true)
		expect(w, 201)
		var d inventory.Document
		if e = json.Unmarshal(w.Body.Bytes(), &d); e != nil {
			t.Fatal(e)
		}
		return d
	}
	approveDoc := func(d inventory.Document) *httptest.ResponseRecorder {
		return call("POST", "/documents/"+d.ID+"/approve", inventory.Review{Version: d.Version, Reason: "测试审核"}, newKey(), true)
	}
	count := func(query string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := db.QueryRow(query, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	expect(call("GET", "", nil, "", false), 401)
	inbound := create("IN", []inventory.Line{{ProductID: "1", Quantity: 10}, {ProductID: "2", Quantity: 5}})
	approvalKey := newKey()
	review := inventory.Review{Version: 1, Reason: "initial"}
	w := call("POST", "/documents/"+inbound.ID+"/approve", review, approvalKey, true)
	expect(w, 200)
	expect(call("POST", "/documents/"+inbound.ID+"/approve", review, approvalKey, true), 200)
	review.Reason = "changed"
	expect(call("POST", "/documents/"+inbound.ID+"/approve", review, approvalKey, true), 409)
	expect(approveDoc(inbound), 409)
	if count("SELECT COUNT(*) FROM inventory_transactions") != 2 {
		t.Fatal("duplicate ledger")
	}
	if _, e = db.Exec("UPDATE inventory_balances SET reserved=8 WHERE product_id=1"); e != nil {
		t.Fatal(e)
	}
	reserved := create("OUT", []inventory.Line{{ProductID: "1", Quantity: 3}})
	expect(approveDoc(reserved), 422)
	if count("SELECT on_hand FROM inventory_balances WHERE product_id=1") != 10 {
		t.Fatal("reserved consumed")
	}
	partial := create("OUT", []inventory.Line{{ProductID: "2", Quantity: 1}, {ProductID: "3", Quantity: 1}})
	expect(approveDoc(partial), 422)
	if count("SELECT on_hand FROM inventory_balances WHERE product_id=2") != 5 || count("SELECT COUNT(*) FROM inventory_transactions") != 2 {
		t.Fatal("partial mutation")
	}
	a := create("OUT", []inventory.Line{{ProductID: "1", Quantity: 2}})
	b := create("OUT", []inventory.Line{{ProductID: "1", Quantity: 2}})
	ka, kb := newKey(), newKey()
	start := make(chan struct{})
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i, d := range []inventory.Document{a, b} {
		wg.Add(1)
		k := []string{ka, kb}[i]
		go func(d inventory.Document, k string) {
			defer wg.Done()
			<-start
			results <- call("POST", "/documents/"+d.ID+"/approve", inventory.Review{Version: 1, Reason: "concurrent"}, k, true).Code
		}(d, k)
	}
	close(start)
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for code := range results {
		if code == 200 {
			ok++
		} else if code == 422 {
			conflict++
		} else {
			t.Fatalf("concurrent status %d", code)
		}
	}
	if ok != 1 || conflict != 1 || count("SELECT on_hand FROM inventory_balances WHERE product_id=1") != 8 {
		t.Fatal("last available race")
	}
	same := create("IN", []inventory.Line{{ProductID: "2", Quantity: 1}})
	k1, k2 := newKey(), newKey()
	start = make(chan struct{})
	results = make(chan int, 2)
	for _, k := range []string{k1, k2} {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			<-start
			results <- call("POST", "/documents/"+same.ID+"/approve", inventory.Review{Version: 1, Reason: "same doc"}, k, true).Code
		}(k)
	}
	close(start)
	wg.Wait()
	close(results)
	ok, conflict = 0, 0
	for code := range results {
		if code == 200 {
			ok++
		} else if code == 409 {
			conflict++
		} else {
			t.Fatalf("same doc status %d", code)
		}
	}
	if ok != 1 || conflict != 1 || count("SELECT COUNT(*) FROM inventory_transactions WHERE document_id=?", same.ID) != 1 {
		t.Fatal("same document double approval")
	}
	reject := create("OUT", []inventory.Line{{ProductID: "2", Quantity: 1}})
	before := count("SELECT COUNT(*) FROM inventory_transactions")
	expect(call("POST", "/documents/"+reject.ID+"/reject", inventory.Review{Version: 1, Reason: "拒绝"}, newKey(), true), 200)
	if count("SELECT COUNT(*) FROM inventory_transactions") != before {
		t.Fatal("reject wrote ledger")
	}
	bv := count("SELECT version FROM inventory_balances WHERE product_id=2")
	wk := newKey()
	expect(call("PATCH", "/2/warning", inventory.Warning{Version: bv, Level: 6}, wk, true), 200)
	expect(call("PATCH", "/2/warning", inventory.Warning{Version: bv, Level: 6}, wk, true), 200)
	expect(call("PATCH", "/2/warning", inventory.Warning{Version: bv, Level: 5}, newKey(), true), 409)
	if count("SELECT COUNT(*) FROM inventory_transactions") != before {
		t.Fatal("warning wrote ledger")
	}
	expect(call("GET", "?status=LOW", nil, "", true), 200)
	// Simultaneous identical keys must all replay one committed approval.
	replayDoc := create("IN", []inventory.Line{{ProductID: "3", Quantity: 2}})
	replayKey := newKey()
	start = make(chan struct{})
	results = make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- call("POST", "/documents/"+replayDoc.ID+"/approve", inventory.Review{Version: 1, Reason: "same key"}, replayKey, true).Code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatalf("same key approval: %d", code)
		}
	}
	if count("SELECT COUNT(*) FROM inventory_transactions WHERE document_id=?", replayDoc.ID) != 1 {
		t.Fatal("same key duplicate event")
	}
	// Every on_hand movement is represented exactly once in the ledger.
	if count("SELECT COUNT(*) FROM inventory_balances b WHERE b.on_hand<>(SELECT COALESCE(SUM(t.on_hand_delta),0) FROM inventory_transactions t WHERE t.product_id=b.product_id)") != 0 {
		t.Fatal("balance and ledger diverged")
	}
	// Force only the audit append to fail and prove all preceding mutations roll back.
	auditDoc := create("IN", []inventory.Line{{ProductID: "3", Quantity: 1}})
	if _, e = db.Exec("CREATE TRIGGER inventory_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='synthetic audit failure'"); e != nil {
		t.Fatal(e)
	}
	failedKey := newKey()
	expect(call("POST", "/documents/"+auditDoc.ID+"/approve", inventory.Review{Version: 1, Reason: "audit fail"}, failedKey, true), 500)
	if count("SELECT on_hand FROM inventory_balances WHERE product_id=3") != 2 || count("SELECT COUNT(*) FROM inventory_transactions WHERE document_id=?", auditDoc.ID) != 0 || count("SELECT COUNT(*) FROM inventory_documents WHERE id=? AND state='PENDING'", auditDoc.ID) != 1 || count("SELECT COUNT(*) FROM idempotency_keys WHERE request_key=?", failedKey) != 0 {
		t.Fatal("audit failure did not roll back")
	}
	db.Exec("DROP TRIGGER inventory_audit_failure")
	expect(call("POST", "/documents", map[string]any{"direction": "IN", "stock_kind": "PHYSICAL", "note": "", "items": []inventory.Line{{ProductID: "1", Quantity: 1}, {ProductID: "1", Quantity: 2}}}, newKey(), true), 422)
	expect(call("GET", "/transactions?product_id=1", nil, "", true), 200)
	expect(call("GET", "/documents?direction=OUT&status=PENDING", nil, "", true), 200)
	bad := httptest.NewRequest("PATCH", "/api/v1/admin/inventory/2/warning", strings.NewReader(`{"version":1,"warning_level":0}`))
	bad.AddCookie(&http.Cookie{Name: "jst-admin", Value: token})
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, bad)
	expect(rw, 403)
	// Revoke just approval permission, leaving read access intact.
	if _, e = db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='inventory.approve'"); e != nil {
		t.Fatal(e)
	}
	expect(approveDoc(auditDoc), 403)
	expect(call("GET", "", nil, "", true), 200)
}
