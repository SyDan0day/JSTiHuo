package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
)

type ContentHandler struct {
	Catalog *catalog.Service
	Auth    IdentityHandler
}

// contentError 用途：返回内容操作的统一错误，拒绝访问写入审计。
// 参数：w 响应器；r 请求；err 业务或基础设施错误。
// 返回值：无，不输出正文或内部数据库信息。
func (h ContentHandler) contentError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 500, "INTERNAL_ERROR"
	var f *identity.Fault
	if errors.As(err, &f) {
		status, code = f.Status, f.Code
	}
	if status == 401 || status == 403 {
		h.Auth.Service.AuditDenied(r.Context(), w.Header().Get("X-Request-ID"))
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
}

// contentActor 用途：校验后台内容会话、操作权限与写入 CSRF。
// 参数：r 当前请求。
// 返回值：管理员主体或 401/403 错误。
func (h ContentHandler) contentActor(r *http.Request) (identity.Actor, error) {
	cookie, err := r.Cookie(h.Auth.cookieName("admin"))
	if err != nil {
		return identity.Actor{}, &identity.Fault{Status: 401, Code: "AUTH_REQUIRED"}
	}
	a, err := h.Auth.Service.Authenticate(r.Context(), cookie.Value, "ADMIN")
	if err != nil {
		return a, err
	}
	if a.Force {
		return a, &identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"}
	}
	permission := "content.read"
	if r.Method != "GET" {
		permission = "content.write"
		if r.Header.Get("Origin") != h.Auth.Origin ||
			!h.Auth.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token")) {
			return a, &identity.Fault{Status: 403, Code: "CSRF_INVALID"}
		}
	}
	if !a.Can(permission) {
		return a, &identity.Fault{Status: 403, Code: "FORBIDDEN"}
	}
	return a, nil
}

// contentBody 用途：严格解码完整内容快照。
// 参数：w 响应器；r 请求；target 创建或编辑命令。
// 返回值：nil 或 400；拒绝缺字段、null、未知字段及超大正文。
func contentBody(w http.ResponseWriter, r *http.Request, target any) error {
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return bad
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512*1024))
	if err != nil {
		return bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return bad
	}
	required := []string{"slug", "title", "body_html", "kind", "published", "sort_order"}
	if _, update := target.(*catalog.ContentUpdate); update {
		required = append(required, "version")
	}
	for _, name := range required {
		if value, ok := fields[name]; !ok || string(value) == "null" {
			return bad
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return bad
	}
	return nil
}

// contentFilter 用途：校验内容列表的固定筛选白名单。
// 参数：r 请求；public 是否为公开列表。
// 返回值：降序游标筛选或 400 错误。
func contentFilter(r *http.Request, public bool) (catalog.Filter, error) {
	copyRequest := r.Clone(r.Context())
	url := *r.URL
	query := url.Query()
	status := query.Get("status")
	if len(query["status"]) > 1 ||
		(status != "" && status != "PUBLISHED" && status != "DRAFT") ||
		(public && status == "DRAFT") {
		return catalog.Filter{}, &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	}
	query.Del("status")
	url.RawQuery = query.Encode()
	copyRequest.URL = &url
	f, err := catalogFilter(copyRequest, true, false)
	f.Status = status
	return f, err
}

// ServeHTTP 用途：分派公开内容查询与后台创建编辑。
// 参数：w 响应器；r 内容资源请求。
// 返回值：无，输出真实持久化结果或统一错误。
func (h ContentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	public := !strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	base := "/api/v1/content"
	var actor identity.Actor
	var err error
	if !public {
		base = "/api/v1/admin/content"
		actor, err = h.contentActor(r)
	}
	if err != nil {
		h.contentError(w, r, err)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, base+"/")
	var result any
	status := 200
	switch {
	case r.Method == "GET" && r.URL.Path == base:
		var f catalog.Filter
		f, err = contentFilter(r, public)
		if err == nil {
			result, err = h.Catalog.ContentList(r.Context(), f, public)
		}
	case r.Method == "GET":
		result, err = h.Catalog.ContentGet(r.Context(), id, public)
	case r.Method == "POST" && !public && r.URL.Path == base:
		var in catalog.ContentInput
		err = contentBody(w, r, &in)
		if err == nil {
			result, err = h.Catalog.CreateContent(r.Context(), actor, in,
				r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
		}
		status = 201
	case r.Method == "PATCH" && !public && r.URL.Path != base:
		var in catalog.ContentUpdate
		err = contentBody(w, r, &in)
		if err == nil {
			result, err = h.Catalog.UpdateContent(r.Context(), actor, id, in,
				r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
		}
	default:
		err = &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	if err != nil {
		h.contentError(w, r, err)
		return
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}
