package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type IdentityHandler struct {
	Service     *identity.Service
	Origin      string
	Development bool
}

func (h IdentityHandler) cookieName(kind string) string {
	if h.Development {
		return "jst-" + kind
	}
	return "__Host-jst-" + kind
}
func (h IdentityHandler) cookie(w http.ResponseWriter, kind, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: h.cookieName(kind), Value: token, Path: "/", HttpOnly: true, Secure: !h.Development, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (h IdentityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	send := func(status int, v any) {
		w.WriteHeader(status)
		if v != nil {
			json.NewEncoder(w).Encode(v)
		}
	}
	fail := func(e error) {
		status, code := 500, "INTERNAL_ERROR"
		var f *identity.Fault
		if errors.As(e, &f) {
			status, code = f.Status, f.Code
		}
		if status == 401 || status == 403 || status == 429 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.Service.AuditDenied(ctx, w.Header().Get("X-Request-ID"))
		}
		if status == 429 {
			w.Header().Set("Retry-After", "900")
		}
		send(status, Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
	}
	decode := func(v any) error {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
		d.DisallowUnknownFields()
		if e := d.Decode(v); e != nil {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		if d.Decode(&struct{}{}) != io.EOF {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		return nil
	}
	if r.Method != "GET" && r.Header.Get("Origin") != h.Origin {
		fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
		return
	}
	if r.Method == "GET" {
		if origin := r.Header.Get("Origin"); origin != "" && origin != h.Origin {
			fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
			fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
			return
		}
	}
	path := r.URL.Path
	if path == "/api/v1/session/bootstrap" && r.Method == "GET" {
		result, e := h.Service.Bootstrap(r.Context())
		if e != nil {
			fail(e)
			return
		}
		h.cookie(w, "preauth", result.Token, 600)
		send(200, result.Session)
		return
	}
	kind := "admin"
	if path == "/api/v1/challenges" || (path == "/api/v1/admin/session" && r.Method == "POST") {
		kind = "preauth"
	}
	c, e := r.Cookie(h.cookieName(kind))
	if e != nil {
		fail(&identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
		return
	}
	var passwordInput struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if path == "/api/v1/admin/session/password" && r.Method == "POST" {
		key := r.Header.Get("Idempotency-Key")
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(key) {
			fail(&identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		if e = decode(&passwordInput); e != nil {
			fail(e)
			return
		}
		v, found, e := h.Service.PasswordReplay(r.Context(), c.Value, r.Header.Get("X-CSRF-Token"), passwordInput.Current, passwordInput.Next, key)
		if e != nil {
			fail(e)
			return
		}
		if found {
			h.cookie(w, "admin", "", -1)
			send(200, v)
			return
		}
	}
	a, e := h.Service.Authenticate(r.Context(), c.Value, strings.ToUpper(kind))
	if e != nil {
		fail(e)
		return
	}
	if r.Method != "GET" && !h.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token")) {
		fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
		return
	}
	if a.Force && path != "/api/v1/admin/session/password" && !(path == "/api/v1/admin/session" && (r.Method == "DELETE" || r.Method == "GET")) {
		fail(&identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"})
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	requestID := w.Header().Get("X-Request-ID")
	switch {
	case path == "/api/v1/challenges" && r.Method == "POST":
		var in struct {
			Purpose string `json:"purpose"`
		}
		if e = decode(&in); e != nil {
			fail(e)
			return
		}
		v, e := h.Service.Challenge(r.Context(), a, in.Purpose, ip)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
	case path == "/api/v1/admin/session" && r.Method == "POST":
		var in identity.Credentials
		if e = decode(&in); e != nil {
			fail(e)
			return
		}
		v, e := h.Service.Login(r.Context(), a, in, ip, requestID)
		if e != nil {
			fail(e)
			return
		}
		h.cookie(w, "preauth", "", -1)
		h.cookie(w, "admin", v.Token, 28800)
		send(200, v.Session)
	case path == "/api/v1/admin/session" && r.Method == "GET":
		v, e := h.Service.ReadSession(r.Context(), a)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
	case path == "/api/v1/admin/session" && r.Method == "DELETE":
		if e = h.Service.Logout(r.Context(), a, requestID); e != nil {
			fail(e)
			return
		}
		h.cookie(w, "admin", "", -1)
		send(204, nil)
	case path == "/api/v1/admin/session/reauth" && r.Method == "POST":
		var in struct {
			Password string `json:"password"`
		}
		if e = decode(&in); e != nil {
			fail(e)
			return
		}
		v, e := h.Service.Reauth(r.Context(), a, in.Password, ip, requestID)
		if e != nil {
			fail(e)
			return
		}
		h.cookie(w, "admin", v.Token, int(v.Session.Expires.Sub(h.Service.Now()).Seconds()))
		send(200, v.Session)
	case path == "/api/v1/admin/session/password" && r.Method == "POST":
		key := r.Header.Get("Idempotency-Key")
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(key) {
			fail(&identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		v, e := h.Service.ChangePassword(r.Context(), a, passwordInput.Current, passwordInput.Next, key, requestID)
		if e != nil {
			replayed, found, replayErr := h.Service.PasswordReplay(r.Context(), c.Value, r.Header.Get("X-CSRF-Token"), passwordInput.Current, passwordInput.Next, key)
			if replayErr != nil {
				fail(replayErr)
				return
			}
			if !found {
				fail(e)
				return
			}
			v = replayed
		}
		h.cookie(w, "admin", "", -1)
		send(200, v)
	default:
		fail(&identity.Fault{Status: 404, Code: "NOT_FOUND"})
	}
}
