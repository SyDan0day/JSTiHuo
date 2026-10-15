package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type RedemptionHandler struct {
	Service *redemption.Service
	Auth    IdentityHandler
}

// ServeHTTP 用途：处理地区、鉴真、选品、日历、兑换和订单读取 HTTP 请求。
// 参数：w HTTP 响应器；r HTTP 请求。
// 返回值：无。
// 约束：校验 Cookie、CSRF、JSON 后调用用例，错误统一响应，成功轮换对应 cookie、CSRF；Handler 不执行 SQL。
func (h RedemptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	send := func(status int, v any) { w.WriteHeader(status); json.NewEncoder(w).Encode(v) }
	fail := func(e error) {
		status, code := 500, "INTERNAL_ERROR"
		var f *identity.Fault
		if errors.As(e, &f) {
			status, code = f.Status, f.Code
		} else if errors.Is(e, context.DeadlineExceeded) || errors.Is(e, context.Canceled) {
			status, code = 503, "TEMPORARY_UNAVAILABLE"
		}
		if status == 401 || status == 403 || status == 429 {
			h.Auth.Service.AuditDenied(r.Context(), w.Header().Get("X-Request-ID"))
		}
		if status == 429 {
			w.Header().Set("Retry-After", "900")
		}
		send(status, Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
	}
	bad := func() error { return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"} }
	path := r.URL.Path
	if path == "/api/v1/regions" && r.Method == "GET" {
		parent := "000000"
		for key, values := range r.URL.Query() {
			if key != "parent_code" || len(values) != 1 || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(values[0]) {
				fail(bad())
				return
			}
			parent = values[0]
		}
		if parent != "000000" {
			if _, ok := h.Service.Regions.ByCode[parent]; !ok {
				fail(bad())
				return
			}
		}
		items := h.Service.Regions.ByParent[parent]
		if items == nil {
			send(200, map[string]any{"items": []any{}})
		} else {
			send(200, map[string]any{"items": items})
		}
		return
	}
	if len(r.URL.Query()) != 0 {
		fail(bad())
		return
	}
	kind := "voucher"
	if path == "/api/v1/voucher-sessions" {
		kind = "preauth"
	}
	if path == "/api/v1/redemption/calendar" {
		if c, e := r.Cookie(h.Auth.cookieName("voucher")); e == nil {
			if va, e := h.Auth.Service.Authenticate(r.Context(), c.Value, "VOUCHER"); e == nil {
				if _, e = h.Service.Options(r.Context(), va); e != nil {
					kind = "order"
				}
			} else {
				kind = "order"
			}
		} else {
			kind = "order"
		}
	}
	if path == "/api/v1/order" {
		kind = "order"
	}
	cookie, e := r.Cookie(h.Auth.cookieName(kind))
	if e != nil {
		fail(&identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
		return
	}
	a, e := h.Auth.Service.Authenticate(r.Context(), cookie.Value, strings.ToUpper(kind))
	if e != nil {
		fail(e)
		return
	}
	if r.Method != "GET" {
		if r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token")) {
			fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
			return
		}
	} else if origin := r.Header.Get("Origin"); origin != "" && origin != h.Auth.Origin {
		fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
		return
	}
	decode := func(v any, required ...string) error {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			return bad()
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
		if e != nil {
			return bad()
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return bad()
		}
		for _, key := range required {
			raw, ok := fields[key]
			if !ok || string(raw) == "null" {
				return bad()
			}
		}
		for _, value := range fields {
			if string(value) == "null" {
				return bad()
			}
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(v) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return bad()
		}
		return nil
	}
	rid := w.Header().Get("X-Request-ID")
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	switch {
	case path == "/api/v1/voucher-session" && r.Method == "GET":
		session, err := h.Auth.Service.ReadSession(r.Context(), a)
		if err != nil {
			fail(err)
			return
		}
		w.Header().Set("X-CSRF-Token", session.CSRF)
		w.Header().Set("X-CSRF-Scope", "voucher")
		send(200, session)
	case path == "/api/v1/voucher-sessions" && r.Method == "POST":
		var in redemption.Credentials
		if e = decode(&in, "code", "secret", "challenge_id", "challenge_answer"); e != nil {
			fail(e)
			return
		}
		result, e := h.Service.Verify(r.Context(), a, in, ip, rid)
		if e != nil {
			fail(e)
			return
		}
		h.Auth.cookie(w, "preauth", "", -1)
		h.Auth.cookie(w, "voucher", result.Session.Token, 900)
		send(200, result.Value)
	case path == "/api/v1/redemption/options" && r.Method == "GET":
		v, e := h.Service.Options(r.Context(), a)
		if e != nil {
			fail(e)
			return
		}
		session, e := h.Auth.Service.ReadSession(r.Context(), a)
		if e != nil {
			fail(e)
			return
		}
		w.Header().Set("X-CSRF-Token", session.CSRF)
		w.Header().Set("X-CSRF-Scope", "voucher")
		send(200, v)
	case path == "/api/v1/redemption/calendar" && r.Method == "GET":
		var v redemption.Calendar
		var e error
		if a.Kind == "ORDER" {
			v, e = h.Service.OrderCalendar(r.Context(), a)
		} else {
			v, e = h.Service.Calendar(r.Context(), a)
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
	case path == "/api/v1/redemptions" && r.Method == "POST":
		var in redemption.Input
		if e = decode(&in, "policy_version", "items", "address", "ship_date", "customer_note"); e != nil {
			fail(e)
			return
		}
		v, e := h.Service.Redeem(r.Context(), a, in, r.Header.Get("Idempotency-Key"), rid)
		if e != nil {
			fail(e)
			return
		}
		h.Auth.cookie(w, "order", v.Session.Token, 1800)
		w.Header().Set("X-Order-Scope", "WRITE")
		w.Header().Set("X-CSRF-Token", v.Session.Session.CSRF)
		w.Header().Set("X-CSRF-Scope", "order")
		send(201, v.Order)
	case path == "/api/v1/order" && r.Method == "GET":
		v, e := h.Service.GetOrder(r.Context(), strconv.FormatInt(a.OrderID, 10), false)
		if e != nil {
			fail(e)
			return
		}
		session, e := h.Auth.Service.ReadSession(r.Context(), a)
		if e != nil {
			fail(e)
			return
		}
		w.Header().Set("X-CSRF-Token", session.CSRF)
		w.Header().Set("X-CSRF-Scope", "order")
		w.Header().Set("X-Order-Scope", a.Scope)
		send(200, v)
	default:
		fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
	}
}

// WithRedemption 用途：把顾客业务路由装配到现有 API。
// 参数：s 已装配业务服务；auth 身份服务依赖。
// 参数：next 其余路由处理器。
// 返回值：HTTP Handler。
// 约束：只按固定路径分派，不处理业务。
func WithRedemption(s *redemption.Service, auth IdentityHandler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/regions", "/api/v1/voucher-sessions", "/api/v1/voucher-session", "/api/v1/redemption/options", "/api/v1/redemption/calendar", "/api/v1/redemptions", "/api/v1/order":
			RedemptionHandler{s, auth}.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}
