package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	rules "jstihuo/internal/domain/voucher"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type OrderHandler struct {
	Service *order.Service
	Auth    IdentityHandler
}

// ServeHTTP 用途：处理订单查询提权、顾客修改及后台查询/编辑/取消/回收/PII 读取。
// 参数：w HTTP 响应器；r HTTP 请求。
// 返回值：无。
// 约束：检查类型化 Cookie、操作权限、CSRF 和完整 JSON，成功结果来自数据库；只在审计授权后返回完整 PII，不执行 SQL。
func (h OrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	send := func(status int, v any) { w.WriteHeader(status); json.NewEncoder(w).Encode(v) }
	fail := func(e error) {
		status, code := 500, "INTERNAL_ERROR"
		var f *identity.Fault
		if errors.As(e, &f) {
			status, code = f.Status, f.Code
		} else if errors.Is(e, context.DeadlineExceeded) {
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
	admin := strings.HasPrefix(path, "/api/v1/admin/")
	kind := "order"
	if admin {
		kind = "admin"
	}
	if path == "/api/v1/order-sessions" {
		kind = "preauth"
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
	if a.Force {
		fail(&identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"})
		return
	}
	if r.Method != "GET" {
		if r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token")) {
			fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
			return
		}
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
		for _, name := range required {
			value, ok := fields[name]
			if !ok || string(value) == "null" {
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
	rid, key := w.Header().Get("X-Request-ID"), r.Header.Get("Idempotency-Key")
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !admin {
		if len(r.URL.Query()) != 0 {
			fail(bad())
			return
		}
		switch {
		case path == "/api/v1/order-sessions" && r.Method == "POST":
			var in order.QueryCredentials
			if e = decode(&in, "identifier", "phone", "challenge_id", "challenge_answer"); e != nil {
				fail(e)
				return
			}
			v, e := h.Service.Query(r.Context(), a, in, ip, rid)
			if e != nil {
				fail(e)
				return
			}
			h.Auth.cookie(w, "preauth", "", -1)
			h.Auth.cookie(w, "order", v.Token, 1800)
			w.Header().Set("X-Order-Scope", "READ")
			send(200, v.Session)
		case path == "/api/v1/order-session/step-up" && r.Method == "POST":
			var in redemption.Credentials
			if e = decode(&in, "code", "secret", "challenge_id", "challenge_answer"); e != nil {
				fail(e)
				return
			}
			preCookie, e := r.Cookie(h.Auth.cookieName("preauth"))
			if e != nil {
				fail(&identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
				return
			}
			pre, e := h.Auth.Service.Authenticate(r.Context(), preCookie.Value, "PREAUTH")
			if e != nil {
				fail(e)
				return
			}
			v, e := h.Service.StepUp(r.Context(), a, pre, in, ip, rid)
			if e != nil {
				fail(e)
				return
			}
			h.Auth.cookie(w, "preauth", "", -1)
			h.Auth.cookie(w, "order", v.Token, 1800)
			w.Header().Set("X-Order-Scope", "WRITE")
			send(200, v.Session)
		case path == "/api/v1/order" && r.Method == "PATCH":
			var in order.EditInput
			if e = decode(&in, "version", "address", "ship_date", "customer_note"); e != nil {
				fail(e)
				return
			}
			v, e := h.Service.Edit(r.Context(), a, strconv.FormatInt(a.OrderID, 10), in, key, rid)
			if e != nil {
				fail(e)
				return
			}
			send(200, v)
		default:
			fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
		}
		return
	}
	suffix := strings.TrimPrefix(path, "/api/v1/admin/orders")
	parts := strings.Split(strings.TrimPrefix(suffix, "/"), "/")
	permission := "order.read"
	if r.Method != "GET" {
		permission = "order.write"
		if len(parts) == 2 {
			switch parts[1] {
			case "cancel":
				permission = "order.cancel"
			case "trash":
				permission = "order.trash"
			case "restore":
				permission = "order.restore"
			case "private-view":
				permission = "order.pii"
			case "shipments":
				permission = "order.ship"
			case "complete":
				permission = "order.complete"
			}
		}
	}
	if !a.Can(permission) {
		fail(&identity.Fault{Status: 403, Code: "FORBIDDEN"})
		return
	}
	if suffix == "" && r.Method == "GET" {
		f := order.Filter{Limit: 20}
		for name, values := range r.URL.Query() {
			if len(values) != 1 {
				fail(bad())
				return
			}
			v := values[0]
			switch name {
			case "limit":
				n, e := strconv.Atoi(v)
				if e != nil || n < 1 || n > 100 {
					fail(bad())
					return
				}
				f.Limit = n
			case "after_id":
				if !rules.ValidID(v) {
					fail(bad())
					return
				}
				f.After = v
			case "q":
				if utf8.RuneCountInString(v) > 100 {
					fail(bad())
					return
				}
				f.Q = v
			case "status":
				if v != "" && v != "PENDING" && v != "SHIPPED" && v != "COMPLETED" && v != "CANCELLED" {
					fail(bad())
					return
				}
				f.Status = v
			case "deleted":
				if v != "true" && v != "false" {
					fail(bad())
					return
				}
				f.Deleted = v == "true"
			case "sort":
				if v != "id_desc" {
					fail(bad())
					return
				}
			default:
				fail(bad())
				return
			}
		}
		v, e := h.Service.List(r.Context(), f)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if len(parts) < 1 || !rules.ValidID(parts[0]) || len(r.URL.Query()) != 0 {
		fail(bad())
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == "GET" {
		v, e := h.Service.Redemption.GetOrder(r.Context(), id, true)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if len(parts) == 1 && r.Method == "PATCH" {
		var in order.EditInput
		if e = decode(&in, "version", "address", "ship_date", "customer_note"); e != nil {
			fail(e)
			return
		}
		v, e := h.Service.Edit(r.Context(), a, id, in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if len(parts) == 2 && r.Method == "POST" {
		if parts[1] == "shipments" {
			var in order.ShipInput
			if e = decode(&in, "version", "carrier_id", "tracking_no"); e != nil {
				fail(e)
				return
			}
			v, err := h.Service.Ship(r.Context(), a, id, in, key, rid)
			if err != nil {
				fail(err)
				return
			}
			send(200, v)
			return
		}
		var in order.VersionReason
		if e = decode(&in, "version", "reason"); e != nil {
			fail(e)
			return
		}
		var result any
		switch parts[1] {
		case "cancel":
			result, e = h.Service.Cancel(r.Context(), a, id, in, key, rid)
		case "complete":
			result, e = h.Service.Complete(r.Context(), a, id, in, key, rid)
		case "trash", "restore":
			result, e = h.Service.Visibility(r.Context(), a, id, in, parts[1] == "restore", key, rid)
		case "private-view":
			result, e = h.Service.PrivateView(r.Context(), a, id, in, key, rid)
		default:
			fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
			return
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, result)
		return
	}
	fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
}

// WithOrders 用途：装配后台订单及顾客查询提权/修改固定路径。
// 参数：s 已装配业务服务；auth 身份服务依赖。
// 参数：next 其余路由处理器。
// 返回值：HTTP Handler。
// 约束：顾客订单 GET 继续交给已有兑换只读处理，只有规定路径分派到订单服务。
func WithOrders(s *order.Service, auth IdentityHandler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/api/v1/admin/orders" || strings.HasPrefix(p, "/api/v1/admin/orders/") || p == "/api/v1/order-sessions" || p == "/api/v1/order-session/step-up" || (p == "/api/v1/order" && r.Method == "PATCH") {
			OrderHandler{s, auth}.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
