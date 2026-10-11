package transport

import (
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type InventoryHandler struct {
	Inventory *inventory.Service
	Auth      IdentityHandler
}

func (h InventoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	send := func(status int, v any) { w.WriteHeader(status); json.NewEncoder(w).Encode(v) }
	fail := func(e error) {
		status, code := 500, "INTERNAL_ERROR"
		var f *identity.Fault
		if errors.As(e, &f) {
			status, code = f.Status, f.Code
		}
		if status == 401 || status == 403 {
			h.Auth.Service.AuditDenied(r.Context(), w.Header().Get("X-Request-ID"))
		}
		send(status, Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
	}
	bad := func() error { return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"} }
	c, e := r.Cookie(h.Auth.cookieName("admin"))
	if e != nil {
		fail(&identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
		return
	}
	a, e := h.Auth.Service.Authenticate(r.Context(), c.Value, "ADMIN")
	if e != nil {
		fail(e)
		return
	}
	if a.Force {
		fail(&identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/inventory")
	permission := "inventory.read"
	if r.Method != "GET" {
		permission = "inventory.write"
		if strings.HasSuffix(path, "/approve") || strings.HasSuffix(path, "/reject") {
			permission = "inventory.approve"
		}
		if r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token")) {
			fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
			return
		}
	}
	if !a.Can(permission) {
		fail(&identity.Fault{Status: 403, Code: "FORBIDDEN"})
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
		if json.Unmarshal(raw, &fields) != nil {
			return bad()
		}
		for _, n := range required {
			if b, ok := fields[n]; !ok || string(b) == "null" {
				return bad()
			}
		}
		for _, b := range fields {
			if string(b) == "null" {
				return bad()
			}
		}
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
			return bad()
		}
		return nil
	}
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	if r.Method == "GET" && (path == "" || path == "/documents" || path == "/transactions") {
		f := inventory.Filter{Limit: 20}
		for k, v := range r.URL.Query() {
			if len(v) != 1 {
				fail(bad())
				return
			}
			value := v[0]
			switch k {
			case "limit":
				n, e := strconv.Atoi(value)
				if e != nil || n < 1 || n > 100 {
					fail(bad())
					return
				}
				f.Limit = n
			case "after_id":
				n, e := strconv.ParseInt(value, 10, 64)
				if e != nil || !inventory.ValidID(value) {
					fail(bad())
					return
				}
				f.After = n
			case "q":
				if utf8.RuneCountInString(value) > 100 {
					fail(bad())
					return
				}
				f.Q = value
			case "sort":
				if value != "id_desc" {
					fail(bad())
					return
				}
			case "status":
				if (path == "" && value != "" && value != "LOW") || (path == "/documents" && value != "" && value != "PENDING" && value != "APPROVED" && value != "REJECTED") || (path == "/transactions" && value != "") {
					fail(bad())
					return
				}
				f.Status = value
			case "direction":
				if path != "/documents" || (value != "IN" && value != "OUT") {
					fail(bad())
					return
				}
				f.Direction = value
			case "product_id":
				if path != "/transactions" || !inventory.ValidID(value) {
					fail(bad())
					return
				}
				f.Product = value
			default:
				fail(bad())
				return
			}
		}
		var v any
		switch path {
		case "":
			v, e = h.Inventory.Balances(r.Context(), f)
		case "/documents":
			v, e = h.Inventory.Documents(r.Context(), f)
		case "/transactions":
			v, e = h.Inventory.Transactions(r.Context(), f)
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if path == "/documents" && r.Method == "POST" {
		var in inventory.Input
		if e = decode(&in, "direction", "stock_kind", "items", "note"); e != nil {
			fail(e)
			return
		}
		for _, l := range in.Items {
			if l.ProductID == "" || l.Quantity == 0 {
				fail(bad())
				return
			}
		}
		v, e := h.Inventory.Create(r.Context(), a, in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if r.Method == "POST" && len(parts) == 3 && parts[0] == "documents" && (parts[2] == "approve" || parts[2] == "reject") {
		var in inventory.Review
		if e = decode(&in, "version", "reason"); e != nil {
			fail(e)
			return
		}
		v, e := h.Inventory.Review(r.Context(), a, parts[1], in, parts[2] == "approve", key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if r.Method == "PATCH" && len(parts) == 2 && parts[1] == "warning" {
		var in inventory.Warning
		if e = decode(&in, "version", "warning_level"); e != nil {
			fail(e)
			return
		}
		v, e := h.Inventory.SetWarning(r.Context(), a, parts[0], in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
}
