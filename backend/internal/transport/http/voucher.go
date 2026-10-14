package transport

import (
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/voucher"
	rules "jstihuo/internal/domain/voucher"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type VoucherHandler struct {
	Vouchers *voucher.Service
	Auth     IdentityHandler
}

// ServeHTTP 用途：处理卡型、批次、发行与授权下载 HTTP 请求。
// 参数：w HTTP 响应器；r HTTP 请求。
// 返回值：无。
// 约束：校验 Cookie、权限、CSRF、JSON，调用 Service；下载只在授权回调内发送二进制，不执行 SQL。
func (h VoucherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/"), "/")
	resource := parts[0]
	permission := "voucher.read"
	if r.Method != "GET" {
		permission = "voucher.generate"
		if resource == "files" {
			permission = "file.read"
		} else if len(parts) == 3 && parts[2] == "seal" {
			permission = "voucher.seal"
		} else if len(parts) == 3 && parts[2] == "reset-secret" {
			permission = "voucher.secret.reset"
		} else if resource == "vouchers" {
			permission = "voucher.write"
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
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return bad()
		}
		for _, n := range required {
			b, ok := fields[n]
			if !ok || string(b) == "null" {
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

	if r.Method == "POST" && len(parts) == 3 && resource == "files" && parts[2] == "download" {
		if !rules.ValidID(parts[1]) {
			fail(bad())
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if e != nil || len(body) > 0 {
			fail(bad())
			return
		}
		started := false
		e = h.Vouchers.Download(r.Context(), a, parts[1], w.Header().Get("X-Request-ID"), func(reader io.Reader, name string) error {
			if e := r.Context().Err(); e != nil {
				return e
			}
			controller := http.NewResponseController(w)
			controller.SetWriteDeadline(time.Now().Add(60 * time.Second))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
			w.Header().Set("Cache-Control", "no-store")
			started = true
			w.WriteHeader(200)
			_, e := io.Copy(w, reader)
			return e
		})
		if e != nil && !started {
			fail(e)
		}
		return
	}
	if r.Method == "GET" && len(parts) == 1 {
		f := voucher.Filter{Limit: 20}
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
				if !rules.ValidID(value) {
					fail(bad())
					return
				}
				f.After = value
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
				if (resource == "voucher-types" && value != "" && value != "ENABLED" && value != "DISABLED") || (resource == "voucher-batches" && value != "" && !strings.Contains(",DRAFT,GENERATING,GENERATED,SEALED,FAILED,", ","+value+",")) || (resource == "vouchers" && value != "") {
					fail(bad())
					return
				}
				f.Status = value
			case "batch_id":
				if resource != "vouchers" || !rules.ValidID(value) {
					fail(bad())
					return
				}
				f.Batch = value
			case "active", "redeemed":
				if resource != "vouchers" || (value != "true" && value != "false") {
					fail(bad())
					return
				}
				b := value == "true"
				if k == "active" {
					f.Active = &b
				} else {
					f.Redeemed = &b
				}
			default:
				fail(bad())
				return
			}
		}
		var result any
		switch resource {
		case "voucher-types":
			result, e = h.Vouchers.Types(r.Context(), f)
		case "voucher-batches":
			result, e = h.Vouchers.Batches(r.Context(), f)
		case "vouchers":
			result, e = h.Vouchers.Vouchers(r.Context(), f)
		default:
			fail(bad())
			return
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, result)
		return
	}
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	if r.Method == "POST" && len(parts) == 1 && resource == "voucher-types" {
		var in voucher.TypeInput
		if e = decode(&in, "name", "selection_mode", "selection_count"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.CreateType(r.Context(), a, in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
		return
	}
	if r.Method == "POST" && len(parts) == 1 && resource == "voucher-batches" {
		var in voucher.Input
		if e = decode(&in, "type_id", "label", "product_ids", "quantity", "code_prefix", "code_length", "code_mode", "secret_length", "secret_format", "valid_from", "valid_until", "activate_on_seal"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.CreateBatch(r.Context(), a, in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
		return
	}

	if r.Method == "POST" && len(parts) == 2 && resource == "vouchers" && parts[1] == "actions" {
		var in voucher.BulkInput
		if e = decode(&in, "ids", "versions", "action", "reason"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.Bulk(r.Context(), a, in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
		return
	}
	if len(parts) >= 2 && !rules.ValidID(parts[1]) {
		fail(bad())
		return
	}
	if r.Method == "GET" && len(parts) == 2 {
		var result any
		if resource == "voucher-batches" {
			result, e = h.Vouchers.GetBatch(r.Context(), parts[1])
		} else if resource == "vouchers" {
			result, e = h.Vouchers.GetVoucher(r.Context(), parts[1])
		} else {
			fail(bad())
			return
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, result)
		return
	}
	if r.Method == "POST" && len(parts) == 3 && resource == "voucher-batches" && parts[2] == "generate" {
		var in struct {
			Version int64 `json:"version"`
		}
		if e = decode(&in, "version"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.Generate(r.Context(), a, parts[1], in.Version, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(202, v)
		return
	}

	if r.Method == "POST" && len(parts) == 3 && parts[2] == "seal" && (resource == "vouchers" || resource == "voucher-batches") {
		var in voucher.VersionReason
		if e = decode(&in, "version", "reason"); e != nil {
			fail(e)
			return
		}
		var result any
		if resource == "vouchers" {
			result, e = h.Vouchers.SealVoucher(r.Context(), a, parts[1], in, key, rid)
		} else {
			result, e = h.Vouchers.SealBatch(r.Context(), a, parts[1], in, key, rid)
		}
		if e != nil {
			fail(e)
			return
		}
		send(200, result)
		return
	}
	if r.Method == "POST" && len(parts) == 3 && resource == "vouchers" && parts[2] == "reset-secret" {
		var in voucher.ResetInput
		if e = decode(&in, "version", "reason", "secret_length", "secret_format"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.Reset(r.Context(), a, parts[1], in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	if r.Method == "PATCH" && len(parts) == 2 && resource == "vouchers" {
		var in voucher.EditInput
		if e = decode(&in, "version", "reason", "valid_from", "valid_until"); e != nil {
			fail(e)
			return
		}
		v, e := h.Vouchers.Edit(r.Context(), a, parts[1], in, key, rid)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
		return
	}
	fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
}

// WithVouchers 用途：装配固定卡券和下载路由。
// 参数：s 已装配业务服务；auth 身份服务依赖。
// 参数：next 其余路由处理器。
// 返回值：HTTP Handler。
// 约束：只分派已定义路径，无业务状态修改。
func WithVouchers(s *voucher.Service, auth IdentityHandler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/v1/admin/files/") && strings.HasSuffix(p, "/download") {
			VoucherHandler{s, auth}.ServeHTTP(w, r)
			return
		}
		for _, prefix := range []string{"/api/v1/admin/voucher-types", "/api/v1/admin/voucher-batches", "/api/v1/admin/vouchers"} {
			if p == prefix || strings.HasPrefix(p, prefix+"/") {
				VoucherHandler{s, auth}.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
