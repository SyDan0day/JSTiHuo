package integration

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/voucher"
	"jstihuo/internal/bootstrap"
	password "jstihuo/internal/infra/crypto"
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

// TestVoucherGeneration 用途：验证卡型冻结、整批生成、租约恢复和发行生命周期。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，集成用例只连接显式指定的隔离测试库并清理子库。
func TestVoucherGeneration(t *testing.T) {
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
	name := "jst_voucher_" + hex.EncodeToString(suffix[:])
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
	key := bytes.Repeat([]byte{6}, 32)
	auth, e := identity.New(db, key, key, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.CreateInitialAdmin(ctx, "voucher-admin", "Voucher-Testing-123"); e != nil {
		t.Fatal(e)
	}
	db.Exec("UPDATE admin_users SET force_password_change=0")
	var aid int64
	db.QueryRow("SELECT id FROM admin_users").Scan(&aid)
	token, csrf := "voucher-test-session", "voucher-test-csrf-abcdefghijklmnop"
	th, ch := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(csrf))
	now := time.Now().UTC()
	r, e := db.Exec("INSERT INTO sessions(token_hash,kind,scope,admin_id,subject_version,csrf_hash,expires_at,last_seen_at,reauth_at) VALUES(?,'ADMIN','WRITE',?,1,?,?,?,?)", th[:], aid, ch[:], now.Add(time.Hour), now, now)
	if e != nil {
		t.Fatal(e)
	}
	sid, _ := r.LastInsertId()
	actor := identity.Actor{AdminID: aid, AuthVersion: 1, SessionID: sid}
	root := t.TempDir()
	svc := voucher.New(db, root, key, key, 1)
	h := transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.WithVouchers(svc, transport.IdentityHandler{Service: auth, Origin: "http://localhost:8080", Development: true}, transport.API(transport.IdentityHandler{Service: auth, Origin: "http://localhost:8080", Development: true}, catalog.New(db, root, key))))
	call := func(method, path string, body any, k string, authenticated bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		req := httptest.NewRequest(method, "/api/v1/admin/"+path, &b)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:8080")
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Idempotency-Key", k)
		if authenticated {
			req.AddCookie(&http.Cookie{Name: "jst-admin", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	typ, e := svc.CreateType(ctx, actor, voucher.TypeInput{Name: "选二", Mode: "N_OF", Count: 2}, "voucher-type-key-0001", "test")
	if e != nil {
		t.Fatal(e)
	}
	same, e := svc.CreateType(ctx, actor, voucher.TypeInput{Name: "选二", Mode: "N_OF", Count: 2}, "voucher-type-key-0001", "test")
	if e != nil || same.ID != typ.ID {
		t.Fatal("replay", e)
	}
	revision, e := svc.CreateType(ctx, actor, voucher.TypeInput{Name: "选二", Mode: "ONE_OF", Count: 1}, "voucher-type-key-0002", "test")
	if e != nil || revision.Revision != 2 {
		t.Fatal(revision, e)
	}
	products := []string{}
	for i := 0; i < 3; i++ {
		r, e := db.Exec("INSERT INTO products(code,name,attributes,description_html,excluded_region_codes) VALUES(?,?,JSON_ARRAY(),'',JSON_ARRAY())", fmt.Sprintf("P%d", i), fmt.Sprintf("商品%d", i))
		if e != nil {
			t.Fatal(e)
		}
		n, _ := r.LastInsertId()
		products = append(products, fmt.Sprint(n))
	}
	input := voucher.Input{TypeID: typ.ID, Label: "合成测试", Products: products, Quantity: 3, Prefix: "JT", CodeLength: 12, CodeMode: "SEQUENTIAL", Start: "1", SecretLength: 12, SecretFormat: "MIXED", From: now.Add(-time.Hour), Until: now.Add(24 * time.Hour), Activate: true}
	batch, e := svc.CreateBatch(ctx, actor, input, "voucher-batch-key-001", "test")
	if e != nil {
		t.Fatal(e)
	}
	if w := call("GET", "voucher-batches", nil, "", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "voucher-batches/"+batch.ID+"/generate", map[string]int{"version": 1}, "voucher-generate-001", true); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var wg sync.WaitGroup
	claims := make(chan voucher.Claim, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, ok, e := svc.Claim(ctx)
			if e != nil {
				errs <- e
			} else if ok {
				claims <- c
			}
		}()
	}
	wg.Wait()
	close(claims)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if len(claims) != 1 {
		t.Fatal("claim count", len(claims))
	}
	claim := <-claims
	if e = svc.Process(ctx, claim); e != nil {
		t.Fatal(e)
	}
	batch, e = svc.GetBatch(ctx, batch.ID)
	if e != nil || batch.State != "GENERATED" || batch.Generated != 3 || batch.FileID == "" {
		t.Fatal(batch, e)
	}
	var storage string
	var kv int
	var owner int64
	var expiry time.Time
	db.QueryRow("SELECT storage_key,key_version,owner_admin_id,expires_at FROM files WHERE id=?", batch.FileID).Scan(&storage, &kv, &owner, &expiry)
	sealed, e := os.ReadFile(filepath.Join(root, storage))
	if e != nil {
		t.Fatal(e)
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, e := gcm.Open(nil, sealed[:12], sealed[12:], voucher.ExportAAD(batch.ID, kv))
	if e != nil {
		t.Fatal(e)
	}
	rows, e := csv.NewReader(bytes.NewReader(plain)).ReadAll()
	if e != nil || len(rows) != 4 || owner != aid || expiry.Before(now.Add(23*time.Hour)) {
		t.Fatal("export metadata", e)
	}
	for _, row := range rows[1:] {
		var hash string
		var active bool
		var count int
		var id string
		if e := db.QueryRow("SELECT id,secret_hash,active FROM vouchers WHERE code=?", row[1]).Scan(&id, &hash, &active); e != nil {
			t.Fatal("read credential", e)
		}
		if e := db.QueryRow("SELECT COUNT(*) FROM voucher_products WHERE voucher_id=?", id).Scan(&count); e != nil {
			t.Fatal("read rights", e)
		}
		if !password.Verify(row[2], hash) || active || count != 3 || bytes.Contains(sealed, []byte(row[2])) {
			t.Fatal("credential or rights invalid")
		}
	}
	if w := call("GET", "vouchers?batch_id="+batch.ID, nil, "", true); w.Code != 200 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("GET", "vouchers?active=1", nil, "", true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := call("GET", "voucher-batches?limit=1", nil, "", true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if _, e = svc.Generate(ctx, actor, batch.ID, 1, "voucher-generate-002", "test"); e != nil {
		t.Fatal("generated replay", e)
	}
	var n int
	db.QueryRow("SELECT COUNT(*) FROM vouchers").Scan(&n)
	if n != 3 {
		t.Fatal(n)
	}
	// Second item collides: the first inserted card and file metadata must roll back.
	input.Start = "0"
	bad, e := svc.CreateBatch(ctx, actor, input, "voucher-batch-key-002", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Generate(ctx, actor, bad.ID, 1, "voucher-generate-003", "test"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.RunOnce(ctx); e == nil {
		t.Fatal("collision accepted")
	}
	bad, _ = svc.GetBatch(ctx, bad.ID)
	if bad.State != "FAILED" || bad.Generated != 0 {
		t.Fatal(bad)
	}
	db.QueryRow("SELECT COUNT(*) FROM files").Scan(&n)
	if n != 1 {
		t.Fatal("orphan metadata", n)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "credentials"))
	if len(entries) != 1 {
		t.Fatal("orphan files", len(entries))
	}
	// A crashed worker's expired lease is reclaimed; its epoch cannot publish.
	input.Start = "100"
	recoverable, e := svc.CreateBatch(ctx, actor, input, "voucher-batch-key-003", "test")
	if e != nil {
		t.Fatal(e)
	}
	svc.Generate(ctx, actor, recoverable.ID, 1, "voucher-generate-004", "test")
	old, ok, e := svc.Claim(ctx)
	if e != nil || !ok {
		t.Fatal(e)
	}
	db.Exec("UPDATE voucher_batches SET lease_until=? WHERE id=?", now.Add(-time.Minute), recoverable.ID)
	fresh, ok, e := svc.Claim(ctx)
	if e != nil || !ok || fresh.Epoch != old.Epoch+1 {
		t.Fatal(fresh, e)
	}
	if e = svc.Process(ctx, old); e != voucher.ErrLeaseLost {
		t.Fatal("stale worker", e)
	}
	if e = svc.Process(ctx, fresh); e != nil {
		t.Fatal(e)
	}
	recovered, _ := svc.GetBatch(ctx, recoverable.ID)
	if recovered.Generated != 3 || recovered.State != "GENERATED" {
		t.Fatal(recovered)
	}
	// Submission checks persisted reauthentication and rejects changed request digests.
	db.Exec("UPDATE sessions SET reauth_at=NULL WHERE id=?", sid)
	if w := call("POST", "voucher-batches/"+bad.ID+"/generate", map[string]int64{"version": bad.Version}, "voucher-generate-005", true); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	db.Exec("UPDATE sessions SET reauth_at=? WHERE id=?", time.Now().UTC(), sid)
	if _, e = svc.CreateType(ctx, actor, voucher.TypeInput{Name: "changed", Mode: "ONE_OF", Count: 1}, "voucher-type-key-0001", "test"); e == nil {
		t.Fatal("digest conflict ignored")
	}
	// Duplicate products and an insufficient N_OF set create no batch.
	for i, ps := range [][]string{{products[0]}, {products[0], products[0]}} {
		invalid := input
		invalid.Products = ps
		if _, e = svc.CreateBatch(ctx, actor, invalid, fmt.Sprintf("voucher-invalid-%03d", i), "test"); e == nil {
			t.Fatal("invalid selection")
		}
	}
	// Check the maximum batch transaction with a low-cost fixture hasher; production
	// Argon2 hashes and verification were exercised above on the three-card batch.
	fixtureHash := svc.Hash
	svc.Hash = func(secret string) (string, error) {
		sum := sha256.Sum256([]byte(secret))
		return "fixture-" + hex.EncodeToString(sum[:]), nil
	}
	input.Start = "10000"
	input.Quantity = 1000
	maximum, e := svc.CreateBatch(ctx, actor, input, "voucher-maximum-0001", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Generate(ctx, actor, maximum.ID, 1, "voucher-maximum-0002", "test"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.RunOnce(ctx); e != nil {
		t.Fatal(e)
	}
	maximum, e = svc.GetBatch(ctx, maximum.ID)
	if e != nil || maximum.Generated != 1000 || maximum.State != "GENERATED" {
		t.Fatal("maximum", maximum, e)
	}
	svc.Hash = fixtureHash
	var linked int
	if e = db.QueryRow("SELECT COUNT(*) FROM voucher_products vp JOIN vouchers v ON v.id=vp.voucher_id WHERE v.batch_id=?", maximum.ID).Scan(&linked); e != nil || linked != 3000 {
		t.Fatal("maximum rights", linked, e)
	}
	// Crash leftovers are removed after the grace period, while referenced files survive.
	orphan := filepath.Join(root, "credentials", "orphan.gcm")
	if e = os.WriteFile(orphan, []byte("synthetic orphan"), 0600); e != nil {
		t.Fatal(e)
	}
	oldTime := time.Now().Add(-25 * time.Hour)
	os.Chtimes(orphan, oldTime, oldTime)
	referenced := filepath.Join(root, storage)
	os.Chtimes(referenced, oldTime, oldTime)
	if e = svc.CleanupOrphans(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(orphan); !os.IsNotExist(e) {
		t.Fatal("orphan retained", e)
	}
	if _, e = os.Stat(referenced); e != nil {
		t.Fatal("referenced ciphertext removed", e)
	}
	var policy []byte
	if e = db.QueryRow("SELECT policy_snapshot FROM voucher_batches WHERE id=?", batch.ID).Scan(&policy); e != nil {
		t.Fatal(e)
	}
	var frozen voucher.Policy
	if e = json.Unmarshal(policy, &frozen); e != nil || frozen.Count != 2 || frozen.Revision != 1 {
		t.Fatal("policy changed", frozen, e)
	}
	if w := call("POST", "voucher-types", map[string]string{"name": "bad"}, "voucher-invalid-004", true); w.Code != 400 {
		t.Fatal("required fields", w.Code)
	}
	exerciseVoucherLifecycle(t, db, svc, actor, batch, maximum, call)
	// Revalidate persisted permissions on writes, even if an actor was previously authorized.
	_, e = db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='voucher.generate'")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CreateType(ctx, actor, voucher.TypeInput{Name: "denied", Mode: "ONE_OF", Count: 1}, "voucher-denied-0001", "test"); e == nil {
		t.Fatal("write permission ignored")
	}

	var nullable sql.NullString
	db.QueryRow("SELECT export_file_id FROM voucher_batches WHERE id=?", bad.ID).Scan(&nullable)
	if nullable.Valid {
		t.Fatal("failed batch has export")
	}
}
