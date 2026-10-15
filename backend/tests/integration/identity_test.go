package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/bootstrap"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func solve(t *testing.T, data string) string {
	t.Helper()
	b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	if e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	segments := []string{"abcdef", "bc", "abdeg", "abcdg", "bcfg", "acdfg", "acdefg", "abc", "abcdefg", "abcdfg"}
	coords := [][2]int{{10, 3}, {21, 12}, {21, 30}, {10, 39}, {1, 30}, {1, 12}, {10, 21}}
	answer := ""
	for i := 0; i < 4; i++ {
		s := ""
		for j, c := range coords {
			r, _, _, _ := img.At(12+i*34+c[0], 8+c[1]).RGBA()
			if r < 30000 {
				s += string(rune('a' + j))
			}
		}
		found := false
		for d, v := range segments {
			if s == v {
				answer += fmt.Sprint(d)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("unreadable captcha")
		}
	}
	return answer
}

// TestIdentity 用途：验证真实管理员认证、会话轮换及访问边界。
// 参数：t 测试上下文，需显式隔离 MySQL。
// 返回值：无，协议或业务不变量不符时测试失败。
func TestIdentity(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("TEST_DB_DSN required")
	}
	db, e := bootstrap.OpenDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	svc, e := identity.New(db, bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), bytes.Repeat([]byte{3}, 32))
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.CreateInitialAdmin(ctx, "test-admin", "Temporary-Password-123"); e != nil {
		t.Fatal(e)
	}
	if svc.CreateInitialAdmin(ctx, "second-admin", "Temporary-Password-123") == nil {
		t.Fatal("bootstrap allowed twice")
	}
	h := contractHTTP(t, transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.IdentityHandler{Service: svc, Origin: "http://localhost:8080", Development: true}))
	call := func(method, path string, body any, cookie *http.Cookie, csrf, key, origin string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Idempotency-Key", key)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("expected %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	parse := func(w *httptest.ResponseRecorder) identity.Session {
		var v identity.Session
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	getCookie := func(w *httptest.ResponseRecorder, name string) *http.Cookie {
		for _, c := range w.Result().Cookies() {
			if c.Name == name && c.MaxAge >= 0 {
				if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
					t.Fatal("unsafe cookie")
				}
				return c
			}
		}
		t.Fatal("cookie missing")
		return nil
	}
	login := func(password string) (*httptest.ResponseRecorder, *http.Cookie, string) {
		w := call("GET", "/api/v1/session/bootstrap", nil, nil, "", "", "http://localhost:8080")
		assert(w, 200)
		pre := getCookie(w, "jst-preauth")
		csrf := parse(w).CSRF
		assert(call("POST", "/api/v1/challenges", map[string]string{"purpose": "LOGIN"}, pre, "bad", "", "http://localhost:8080"), 403)
		w = call("POST", "/api/v1/challenges", map[string]string{"purpose": "LOGIN"}, pre, csrf, "", "http://localhost:8080")
		assert(w, 200)
		var challenge identity.Challenge
		json.Unmarshal(w.Body.Bytes(), &challenge)
		data := identity.Credentials{Login: "test-admin", Password: password, ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}
		w = call("POST", "/api/v1/admin/session", data, pre, csrf, "", "http://localhost:8080")
		assert(w, 200)
		assert(call("POST", "/api/v1/admin/session", data, pre, csrf, "", "http://localhost:8080"), 401)
		return w, getCookie(w, "jst-admin"), parse(w).CSRF
	}
	w, cookie, csrf := login("Temporary-Password-123")
	if !parse(w).ForceChange || len(parse(w).Permissions) != 0 {
		t.Fatal("temporary password scope")
	}
	assert(call("POST", "/api/v1/admin/session/reauth", map[string]string{"password": "Temporary-Password-123"}, cookie, csrf, "", "http://localhost:8080"), 403)
	assert(call("POST", "/api/v1/admin/session/password", map[string]string{"current_password": "Temporary-Password-123", "new_password": "Permanent-Password-123"}, cookie, csrf, "password-change-0001", "https://evil.example"), 403)
	payload := map[string]string{"current_password": "Temporary-Password-123", "new_password": "Permanent-Password-123"}
	w = call("POST", "/api/v1/admin/session/password", payload, cookie, csrf, "password-change-0001", "http://localhost:8080")
	assert(w, 200)
	assert(call("POST", "/api/v1/admin/session/password", payload, cookie, csrf, "password-change-0001", "http://localhost:8080"), 200)
	payload["new_password"] = "Different-Password-123"
	assert(call("POST", "/api/v1/admin/session/password", payload, cookie, csrf, "password-change-0001", "http://localhost:8080"), 409)
	assert(call("GET", "/api/v1/admin/session", nil, cookie, "", "", "http://localhost:8080"), 401)
	w, cookie, csrf = login("Permanent-Password-123")
	if parse(w).ForceChange || len(parse(w).Permissions) == 0 {
		t.Fatal("permissions missing")
	}
	actor, e := svc.Authenticate(ctx, cookie.Value, "ADMIN")
	if e != nil {
		t.Fatal(e)
	}
	if !actor.Can("product.write") || actor.Can("invented.permission") {
		t.Fatal("RBAC mismatch")
	}
	oldCookie := cookie
	w = call("POST", "/api/v1/admin/session/reauth", map[string]string{"password": "Permanent-Password-123"}, cookie, csrf, "", "http://localhost:8080")
	assert(w, 200)
	cookie = getCookie(w, "jst-admin")
	csrf = parse(w).CSRF
	assert(call("GET", "/api/v1/admin/session", nil, oldCookie, "", "", "http://localhost:8080"), 401)
	actor, e = svc.Authenticate(ctx, cookie.Value, "ADMIN")
	if e != nil || !actor.Recent(svc.Now()) {
		t.Fatal("reauth missing")
	}
	if e = svc.SetEnabled(ctx, actor, actor.AdminID, false, "last-admin-test"); e == nil {
		t.Fatal("last administrator disabled")
	}
	w = call("GET", "/api/v1/admin/session", nil, cookie, "", "", "http://localhost:8080")
	assert(w, 200)
	csrf = parse(w).CSRF
	assert(call("DELETE", "/api/v1/admin/session", nil, cookie, csrf, "", "http://localhost:8080"), 204)
	assert(call("GET", "/api/v1/admin/session", nil, cookie, "", "", "http://localhost:8080"), 401)
	_, cookie, _ = login("Permanent-Password-123")
	db.Exec("UPDATE admin_users SET auth_version=auth_version+1 WHERE login_name='test-admin'")
	assert(call("GET", "/api/v1/admin/session", nil, cookie, "", "", "http://localhost:8080"), 401)
	_, cookie, _ = login("Permanent-Password-123")
	db.Exec("UPDATE admin_users SET enabled=0 WHERE login_name='test-admin'")
	assert(call("GET", "/api/v1/admin/session", nil, cookie, "", "", "http://localhost:8080"), 401)
	var audits int
	if e = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='admin.password_changed'").Scan(&audits); e != nil || audits != 1 {
		t.Fatal("password audit duplicated")
	}
	var raw string
	db.QueryRow("SELECT password_hash FROM admin_users WHERE login_name='test-admin'").Scan(&raw)
	if !strings.HasPrefix(raw, "$argon2id$") {
		t.Fatal("password not hashed")
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE CAST(diff AS CHAR) LIKE '%Password%'").Scan(&count)
	if count != 0 {
		t.Fatal("credential audit leak")
	}
	// Invalid challenge attempts are persisted even when the API rejects the request.
	preResult, e := svc.Bootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	preActor, e := svc.Authenticate(ctx, preResult.Token, "PREAUTH")
	if e != nil {
		t.Fatal(e)
	}
	challenge, e := svc.Challenge(ctx, preActor, "LOGIN", "attempt-test")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 5; i++ {
		_, e = svc.Login(ctx, preActor, identity.Credentials{Login: "missing-admin", Password: "Incorrect-Password-123", ChallengeID: challenge.ID, Answer: "XXXX"}, "attempt-test", fmt.Sprintf("attempt-%d", i))
		if e == nil {
			t.Fatal("wrong challenge accepted")
		}
	}
	_, e = svc.Login(ctx, preActor, identity.Credentials{Login: "missing-admin", Password: "Incorrect-Password-123", ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}, "attempt-test", "attempt-exhausted")
	if e == nil {
		t.Fatal("challenge limit bypassed")
	}
	// Unknown accounts have the same credential error; sixth failure is rate limited.
	for i := 0; i < 6; i++ {
		challenge, e = svc.Challenge(ctx, preActor, "LOGIN", "rate-test")
		if e != nil {
			t.Fatal(e)
		}
		_, e = svc.Login(ctx, preActor, identity.Credentials{Login: "missing-admin", Password: "Incorrect-Password-123", ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}, "rate-test", fmt.Sprintf("failure-%d", i))
		f, ok := e.(*identity.Fault)
		expected := 401
		if i == 5 {
			expected = 429
		}
		if !ok || f.Status != expected {
			t.Fatalf("unexpected failure status: %v", e)
		}
	}
	// Fixed clock validates absolute/idle expiration without sleeping.
	v, e := svc.Bootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	svc.Now = func() time.Time { return time.Now().UTC().Add(11 * time.Minute) }
	if _, e = svc.Authenticate(ctx, v.Token, "PREAUTH"); e == nil {
		t.Fatal("expired preauth accepted")
	}
	svc.Now = func() time.Time { return time.Now().UTC() }
}
