package transport

import (
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CatalogHandler struct {
	Catalog *catalog.Service
	Auth    IdentityHandler
}

// ServeHTTP 用途：提供受控商品、分类及图片接口，商品写入返回权威快照。
// 参数：w 响应器；r 当前请求。
// 返回值：无，权限、字段和业务错误统一输出 JSON。
func (h CatalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		if status == 401 || status == 403 {
			h.Auth.Service.AuditDenied(r.Context(), w.Header().Get("X-Request-ID"))
		}
		send(status, Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/media/") {
		if r.Method != "GET" {
			fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
			return
		}
		f, e := h.Catalog.Image(r.Context(), strings.TrimPrefix(path, "/media/"))
		if e != nil {
			fail(e)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		http.ServeContent(w, r, "image.png", time.Time{}, f)
		return
	}
	public := path == "/api/v1/products" || strings.HasPrefix(path, "/api/v1/products/")
	var actor identity.Actor
	if !public {
		c, e := r.Cookie(h.Auth.cookieName("admin"))
		if e != nil {
			fail(&identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
			return
		}
		actor, e = h.Auth.Service.Authenticate(r.Context(), c.Value, "ADMIN")
		if e != nil {
			fail(e)
			return
		}
		if actor.Force {
			fail(&identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"})
			return
		}
		permission := "product.read"
		if r.Method != "GET" {
			permission = "product.write"
			if strings.HasPrefix(path, "/api/v1/admin/files") {
				permission = "file.write"
			}
			if r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(actor, r.Header.Get("X-CSRF-Token")) {
				fail(&identity.Fault{Status: 403, Code: "CSRF_INVALID"})
				return
			}
		}
		if !actor.Can(permission) {
			fail(&identity.Fault{Status: 403, Code: "FORBIDDEN"})
			return
		}
	}
	decode := func(v any, required ...string) error {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 256*1024))
		if e != nil {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		if _, ok := v.(*catalog.Input); ok {
			hasVersion := false
			for _, n := range required {
				if n == "version" {
					hasVersion = true
				}
			}
			if !hasVersion {
				if _, exists := fields["version"]; exists {
					return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
				}
			}
		}
		for _, n := range required {
			if b, ok := fields[n]; !ok || string(b) == "null" {
				return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
			}
		}
		for _, b := range fields {
			if string(b) == "null" {
				return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
			}
		}
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
			return &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		return nil
	}
	key := r.Header.Get("Idempotency-Key")
	requestID := w.Header().Get("X-Request-ID")
	base := "/api/v1/admin/products"
	if public {
		base = "/api/v1/products"
	}
	n := strings.TrimPrefix(path, base+"/")
	switch {
	case path == base && r.Method == "GET":
		f, e := catalogFilter(r, false, public)
		if e != nil {
			fail(e)
			return
		}
		if public {
			v, e := h.Catalog.PublicList(r.Context(), f)
			if e != nil {
				fail(e)
				return
			}
			send(200, v)
		} else {
			v, e := h.Catalog.List(r.Context(), f, false)
			if e != nil {
				fail(e)
				return
			}
			send(200, v)
		}
	case path == base && r.Method == "POST" && !public:
		var in catalog.Input
		if e := decode(&in, "code", "name", "attributes", "description_html", "media_ids", "excluded_region_codes", "sort_order"); e != nil {
			fail(e)
			return
		}
		if in.Version != 0 {
			fail(&identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"})
			return
		}
		v, e := h.Catalog.Create(r.Context(), actor, in, key, requestID)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
	case strings.HasPrefix(path, base+"/") && r.Method == "GET":
		if public {
			v, e := h.Catalog.PublicGet(r.Context(), n)
			if e != nil {
				fail(e)
				return
			}
			send(200, v)
		} else {
			v, e := h.Catalog.Get(r.Context(), n, false)
			if e != nil {
				fail(e)
				return
			}
			send(200, v)
		}
	case strings.HasPrefix(path, base+"/") && strings.HasSuffix(path, "/status") && r.Method == "POST" && !public:
		var in catalog.StatusInput
		if e := decode(&in, "version", "status", "reason"); e != nil {
			fail(e)
			return
		}
		v, e := h.Catalog.Status(r.Context(), actor, strings.TrimSuffix(n, "/status"), in, key, requestID)
		if e != nil {
			fail(e)
			return
		}
		product, e := h.Catalog.Get(r.Context(), v.ID, false)
		if e != nil {
			fail(e)
			return
		}
		send(200, product)
	case strings.HasPrefix(path, base+"/") && r.Method == "PATCH" && !public:
		var in catalog.Input
		if e := decode(&in, "code", "name", "attributes", "description_html", "media_ids", "excluded_region_codes", "sort_order", "version"); e != nil {
			fail(e)
			return
		}
		v, e := h.Catalog.Update(r.Context(), actor, n, in, key, requestID)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
	case path == "/api/v1/admin/categories" && r.Method == "GET":
		f, e := catalogFilter(r, true, false)
		if e != nil {
			fail(e)
			return
		}
		v, e := h.Catalog.Categories(r.Context(), f)
		if e != nil {
			fail(e)
			return
		}
		send(200, v)
	case path == "/api/v1/admin/categories" && r.Method == "POST":
		var in catalog.CategoryInput
		if e := decode(&in, "name", "sort_order"); e != nil {
			fail(e)
			return
		}
		v, e := h.Catalog.CreateCategory(r.Context(), actor, in, key, requestID)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
	case path == "/api/v1/admin/files" && r.Method == "POST":
		r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
		if e := r.ParseMultipartForm(1024 * 1024); e != nil {
			fail(&identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		if len(r.MultipartForm.Value) != 1 || len(r.MultipartForm.Value["kind"]) != 1 || r.FormValue("kind") != "IMAGE" || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
			fail(&identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		file, _, e := r.FormFile("file")
		if e != nil {
			fail(&identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		defer file.Close()
		raw, e := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
		if e != nil {
			fail(e)
			return
		}
		v, e := h.Catalog.Upload(r.Context(), actor, raw, key, requestID)
		if e != nil {
			fail(e)
			return
		}
		send(201, v)
	default:
		fail(&identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
	}
}
func catalogFilter(r *http.Request, category, public bool) (catalog.Filter, error) {
	f := catalog.Filter{Limit: 20}
	bad := func() (catalog.Filter, error) { return f, &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"} }
	for k, v := range r.URL.Query() {
		if len(v) != 1 {
			return bad()
		}
		switch k {
		case "after_id":
			n, e := strconv.ParseInt(v[0], 10, 64)
			if e != nil || n <= 0 || strings.HasPrefix(v[0], "0") {
				return bad()
			}
			f.After = n
		case "limit":
			n, e := strconv.Atoi(v[0])
			if e != nil || n < 1 || n > 100 {
				return bad()
			}
			f.Limit = n
		case "q":
			if utf8.RuneCountInString(v[0]) > 100 {
				return bad()
			}
			f.Q = v[0]
		case "status":
			if category {
				if v[0] != "" {
					return bad()
				}
			} else {
				f.Status = v[0]
				if f.Status != "" && f.Status != "DRAFT" && f.Status != "ENABLED" && f.Status != "DISABLED" {
					return bad()
				}
				if public && f.Status != "" && f.Status != "ENABLED" {
					return bad()
				}
			}
		case "sort":
			if v[0] != "id_desc" {
				return bad()
			}
		default:
			return bad()
		}
	}
	return f, nil
}

// API 用途：装配商品、内容和可选库存路由。
// 参数：auth 身份处理器；catalogue 目录服务；stock 可选库存服务。
// 返回值：仅分派已实现 API 的处理器。
func API(auth IdentityHandler, catalogue *catalog.Service, stock ...*inventory.Service) http.Handler {
	cat := CatalogHandler{catalogue, auth}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/api/v1/content" || strings.HasPrefix(p, "/api/v1/content/") ||
			p == "/api/v1/admin/content" || strings.HasPrefix(p, "/api/v1/admin/content/") {
			ContentHandler{catalogue, auth}.ServeHTTP(w, r)
			return
		}
		if len(stock) > 0 && (p == "/api/v1/admin/inventory" || strings.HasPrefix(p, "/api/v1/admin/inventory/")) {
			InventoryHandler{stock[0], auth}.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "/media/") || p == "/api/v1/products" || strings.HasPrefix(p, "/api/v1/products/") || p == "/api/v1/admin/products" || strings.HasPrefix(p, "/api/v1/admin/products/") || p == "/api/v1/admin/categories" || p == "/api/v1/admin/files" {
			cat.ServeHTTP(w, r)
			return
		}
		auth.ServeHTTP(w, r)
	})
}
