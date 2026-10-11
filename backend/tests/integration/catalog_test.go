package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"image"
	"image/png"
	"io"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/bootstrap"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCatalog(t *testing.T) {
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
	database := "jst_catalog_" + hex.EncodeToString(suffix[:])
	if _, e = parent.Exec("CREATE DATABASE `" + database + "`"); e != nil {
		t.Fatal(e)
	}
	defer parent.Exec("DROP DATABASE `" + database + "`")
	cfg.DBName = database
	db, e := bootstrap.OpenDB(cfg.FormatDSN())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	key := bytes.Repeat([]byte{8}, 32)
	auth, e := identity.New(db, key, key, key)
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.CreateInitialAdmin(ctx, "catalog-admin", "Testing-Password-123"); e != nil {
		t.Fatal(e)
	}
	db.Exec("UPDATE admin_users SET force_password_change=0")
	var adminID int64
	db.QueryRow("SELECT id FROM admin_users WHERE login_name='catalog-admin'").Scan(&adminID)
	token := hex.EncodeToString(suffix[:]) + "-test-session-only"
	csrf := "test-csrf-token-abcdefghijklmnop"
	hash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrf))
	if _, e = db.Exec("INSERT INTO sessions(token_hash,kind,scope,admin_id,subject_version,csrf_hash,expires_at,last_seen_at) VALUES(?,'ADMIN','WRITE',?,1,?,?,?)", hash[:], adminID, csrfHash[:], time.Now().Add(time.Hour), time.Now()); e != nil {
		t.Fatal(e)
	}
	svc := catalog.New(db, t.TempDir(), key)
	h := transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.API(transport.IdentityHandler{Service: auth, Origin: "http://localhost:8080", Development: true}, svc))
	cookie := &http.Cookie{Name: "jst-admin", Value: token}
	call := func(method, path string, payload any, key string, authenticated bool) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if payload != nil {
			json.NewEncoder(&b).Encode(payload)
		}
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Idempotency-Key", key)
		if authenticated {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("wanted %d got %d %s", status, w.Code, w.Body.String())
		}
	}
	upload := func(data []byte, key string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		form := multipart.NewWriter(&b)
		form.WriteField("kind", "IMAGE")
		file, _ := form.CreateFormFile("file", "../unsafe-name.png")
		file.Write(data)
		form.Close()
		r := httptest.NewRequest("POST", "/api/v1/admin/files", &b)
		r.Header.Set("Content-Type", form.FormDataContentType())
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Idempotency-Key", key)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert(call("POST", "/api/v1/admin/products", map[string]string{}, "test-product-00001", false), 401)
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 16, 16)))
	w := upload(raw.Bytes(), "test-image-000001")
	assert(w, 201)
	var file catalog.File
	json.Unmarshal(w.Body.Bytes(), &file)
	if file.State != "READY" {
		t.Fatal("not ready")
	}
	w = upload(raw.Bytes(), "test-image-000001")
	assert(w, 201)
	var again catalog.File
	json.Unmarshal(w.Body.Bytes(), &again)
	if file.ID != again.ID {
		t.Fatal("duplicate image")
	}
	assert(upload([]byte(`<svg onload="alert(1)"></svg>`), "test-image-malicious"), 422)
	w = call("POST", "/api/v1/admin/categories", catalog.CategoryInput{Name: "Gifts", Sort: 1}, "test-category-0001", true)
	assert(w, 201)
	var category catalog.Category
	json.Unmarshal(w.Body.Bytes(), &category)
	in := catalog.Input{Code: "test-one", Name: "Test gift", CategoryID: category.ID, Attributes: []catalog.Attribute{{Name: "Size", Value: "One"}}, Description: `<p onclick="bad()">Hello <strong>gift</strong></p><script>alert(1)</script>`, MediaIDs: []string{file.ID}, Excluded: []string{}, Sort: 1}
	w = call("POST", "/api/v1/admin/products", in, "test-product-00001", true)
	assert(w, 201)
	var product catalog.Product
	json.Unmarshal(w.Body.Bytes(), &product)
	if product.Code != "TEST-ONE" || product.Status != "DRAFT" || len(product.MediaIDs) != 1 || strings.Contains(product.Description, "script") || strings.Contains(product.Description, "onclick") {
		t.Fatalf("bad product %+v", product)
	}
	var balance, reserved int
	db.QueryRow("SELECT on_hand,reserved FROM inventory_balances WHERE product_id=?", product.ID).Scan(&balance, &reserved)
	if balance != 0 || reserved != 0 {
		t.Fatal("initial stock not zero")
	}
	w = call("POST", "/api/v1/admin/products", in, "test-product-00001", true)
	assert(w, 201)
	var replay catalog.Product
	json.Unmarshal(w.Body.Bytes(), &replay)
	if replay.ID != product.ID {
		t.Fatal("duplicate product")
	}
	in.Name = "Changed"
	assert(call("POST", "/api/v1/admin/products", in, "test-product-00001", true), 409)
	assert(call("POST", "/api/v1/admin/products", in, "test-product-00002", true), 409)
	assert(call("GET", "/api/v1/products/"+product.ID, nil, "", false), 404)
	assert(call("GET", "/api/v1/products?status=DRAFT", nil, "", false), 400)
	assert(call("GET", "/api/v1/products?unknown=sql", nil, "", false), 400)
	statusPath := "/api/v1/admin/products/" + product.ID + "/status"
	w = call("POST", statusPath, catalog.StatusInput{Version: 1, Status: "ENABLED", Reason: "Publish"}, "test-publish-00001", true)
	assert(w, 200)
	assert(call("POST", statusPath, catalog.StatusInput{Version: 1, Status: "ENABLED", Reason: "Publish"}, "test-publish-00001", true), 200)
	assert(call("POST", statusPath, catalog.StatusInput{Version: 1, Status: "DISABLED"}, "test-stale-000001", true), 409)
	w = call("GET", "/api/v1/products/"+product.ID, nil, "", false)
	assert(w, 200)
	var public catalog.Public
	json.Unmarshal(w.Body.Bytes(), &public)
	if len(public.Images) != 1 || strings.Contains(w.Body.String(), "media_ids") {
		t.Fatal("bad public projection")
	}
	assert(call("GET", public.Images[0], nil, "", false), 200)
	assert(call("GET", "/media/../../secret", nil, "", false), 404)
	in.Code = "TEST-ONE"
	in.Name = "Updated gift"
	in.Version = 2
	in.CategoryID = ""
	in.From = nil
	in.Until = nil
	w = call("PATCH", "/api/v1/admin/products/"+product.ID, in, "test-update-000001", true)
	assert(w, 200)
	product = catalog.Product{}
	json.Unmarshal(w.Body.Bytes(), &product)
	if product.Version != 3 || product.CategoryID != "" {
		t.Fatal("full snapshot update failed")
	}
	// A bad second media reference cannot leave a partial edit or change stock.
	in.Version = 3
	in.MediaIDs = []string{file.ID, "999999"}
	assert(call("PATCH", "/api/v1/admin/products/"+product.ID, in, "test-invalid-media", true), 422)
	var version int
	db.QueryRow("SELECT version FROM products WHERE id=?", product.ID).Scan(&version)
	if version != 3 {
		t.Fatal("failed update mutated product")
	}
	in.MediaIDs = []string{file.ID}
	// Missing main image blocks publication even if metadata has drifted outside the API.
	db.Exec("DELETE FROM product_media WHERE product_id=?", product.ID)
	assert(call("POST", statusPath, catalog.StatusInput{Version: 3, Status: "ENABLED"}, "test-no-main-image", true), 422)
	db.Exec("INSERT INTO product_media(product_id,file_id,position) VALUES(?,?,0)", product.ID, file.ID)
	// Concurrent writes with the same expected version have one winner.
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			x := in
			x.Name = fmt.Sprintf("Race %d", i)
			codes <- call("PATCH", "/api/v1/admin/products/"+product.ID, x, fmt.Sprintf("test-race-update-%d", i), true).Code
		}(i)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("race outcomes %v", counts)
	}
	assert(call("POST", statusPath, catalog.StatusInput{Version: 4, Status: "DISABLED", Reason: "Stop"}, "test-disable-00001", true), 200)
	assert(call("GET", "/api/v1/products/"+product.ID, nil, "", false), 404)
	// Removing permissions is effective on the next request, even with an old cookie.
	db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='product.write'")
	assert(call("PATCH", "/api/v1/admin/products/"+product.ID, in, "test-no-permission", true), 403)
	var n int
	db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='createProduct'").Scan(&n)
	if n != 1 {
		t.Fatal("idempotent audit duplicated")
	}
	var state sql.NullString
	db.QueryRow("SELECT status FROM products WHERE id=?", product.ID).Scan(&state)
	if state.String != "DISABLED" {
		t.Fatal("status changed")
	}
}
