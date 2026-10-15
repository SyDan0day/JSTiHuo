package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	rules "jstihuo/internal/domain/voucher"
)

type OperationsHandler struct {
	Service *operations.Service
	Auth    IdentityHandler
}

// operationError 用途：输出运营统一错误并审计访问拒绝。
// 参数：w 响应器；r 请求；err 业务或基础设施错误。
// 返回值：无，不输出正文、联系人或内部错误。
func (h OperationsHandler) operationError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := 500, "INTERNAL_ERROR"
	var f *identity.Fault
	if errors.As(err, &f) {
		status, code = f.Status, f.Code
	} else if errors.Is(err, context.DeadlineExceeded) {
		status, code = 503, "TEMPORARY_UNAVAILABLE"
	}
	if status == 401 || status == 403 || status == 429 {
		h.Auth.Service.AuditDenied(r.Context(), w.Header().Get("X-Request-ID"))
	}
	if status == 429 {
		w.Header().Set("Retry-After", "3600")
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
}

// operationActor 用途：校验运营接口的类型化会话、权限与写入 CSRF。
// 参数：r 请求；kind 会话类型；permission 管理员操作权限，公开操作为空。
// 返回值：有效主体或认证、权限、CSRF 错误。
func (h OperationsHandler) operationActor(r *http.Request, kind, permission string) (identity.Actor, error) {
	cookie, err := r.Cookie(h.Auth.cookieName(strings.ToLower(kind)))
	if err != nil {
		return identity.Actor{}, &identity.Fault{Status: 401, Code: "AUTH_REQUIRED"}
	}
	a, err := h.Auth.Service.Authenticate(r.Context(), cookie.Value, kind)
	if err != nil {
		return a, err
	}
	if a.Force {
		return a, &identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"}
	}
	if permission != "" && !a.Can(permission) {
		return a, &identity.Fault{Status: 403, Code: "FORBIDDEN"}
	}
	if r.Method != "GET" && (r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(a, r.Header.Get("X-CSRF-Token"))) {
		return a, &identity.Fault{Status: 403, Code: "CSRF_INVALID"}
	}
	return a, nil
}

// operationBody 用途：严格解码运营 JSON，拒绝未定义的稀疏更新。
// 参数：w 响应器；r 请求；target 命令对象；required 必须出现的字段。
// 返回值：nil 或内容类型、超长、缺失、空值、未知字段错误。
func operationBody(w http.ResponseWriter, r *http.Request, target any, required ...string) error {
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return bad
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
	if err != nil {
		return bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return bad
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return bad
		}
	}
	for _, value := range fields {
		if string(value) == "null" {
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

// operationFilter 用途：校验运营列表的固定降序游标与状态白名单。
// 参数：r 请求；states 允许的非空状态；search 是否支持 q。
// 返回值：筛选或 400，不接受任意 SQL 排序。
func operationFilter(r *http.Request, states []string, search bool) (operations.Filter, error) {
	f := operations.Filter{Limit: 20}
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	for name, values := range r.URL.Query() {
		if len(values) != 1 {
			return f, bad
		}
		v := values[0]
		switch name {
		case "after_id":
			if !rules.ValidID(v) {
				return f, bad
			}
			f.After = v
		case "limit":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 100 {
				return f, bad
			}
			f.Limit = n
		case "q":
			if (!search && v != "") || utf8.RuneCountInString(v) > 100 {
				return f, bad
			}
			f.Q = v
		case "status":
			valid := v == ""
			for _, state := range states {
				valid = valid || v == state
			}
			if !valid {
				return f, bad
			}
			f.Status = v
		case "sort":
			if v != "id_desc" {
				return f, bad
			}
		default:
			return f, bad
		}
	}
	return f, nil
}

// ServeHTTP 用途：处理公开反馈与后台反馈处理接口。
// 参数：w 响应器；r 请求。
// 返回值：无，写入数据库结果或统一错误。
func (h OperationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	public := r.URL.Path == "/api/v1/feedback"
	kind, permission := "ADMIN", "feedback.read"
	if r.Method != "GET" {
		permission = "feedback.write"
	}
	if public {
		kind, permission = "PREAUTH", ""
	}
	a, err := h.operationActor(r, kind, permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	var result any
	switch {
	case public && r.Method == "POST":
		var in operations.FeedbackInput
		err = operationBody(w, r, &in, "contact_name", "phone", "message", "challenge_id", "challenge_answer")
		if err == nil {
			var order *identity.Actor
			if cookie, e := r.Cookie(h.Auth.cookieName("order")); e == nil {
				if actor, e := h.Auth.Service.Authenticate(r.Context(), cookie.Value, "ORDER"); e == nil {
					order = &actor
				}
			}
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			result, err = h.Service.CreateFeedback(r.Context(), a, order, in, ip, w.Header().Get("X-Request-ID"))
		}
	case r.URL.Path == "/api/v1/admin/feedback" && r.Method == "GET":
		var f operations.Filter
		f, err = operationFilter(r, []string{"OPEN", "RESOLVED"}, true)
		if err == nil {
			result, err = h.Service.FeedbackList(r.Context(), f)
		}
	case strings.HasSuffix(r.URL.Path, "/resolve") && !public && r.Method == "POST":
		var in operations.FeedbackResolve
		err = operationBody(w, r, &in, "version", "resolution")
		if err == nil {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/feedback/"), "/resolve")
			result, err = h.Service.ResolveFeedback(r.Context(), a, id, in, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
		}
	default:
		err = &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(result)
}

// WithOperations 用途：装配已实现的运营 API 路由。
// 参数：s 运营服务；auth 身份服务；next 其他处理器。
// 返回值：只分派已定义路径的 HTTP 处理器。
func WithOperations(s *operations.Service, auth IdentityHandler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		h := OperationsHandler{s, auth}
		if p == "/api/v1/admin/inventory/reconciliation" {
			h.serveAdministration(w, r)
			return
		}
		if p == "/api/v1/admin/admins" || strings.HasPrefix(p, "/api/v1/admin/admins/") || p == "/api/v1/admin/roles" || strings.HasPrefix(p, "/api/v1/admin/roles/") || p == "/api/v1/admin/permissions" || p == "/api/v1/admin/config" || p == "/api/v1/admin/dashboard" || p == "/api/v1/admin/audit-logs" || p == "/api/v1/admin/files" && r.Method == "GET" {
			h.serveAdministration(w, r)
			return
		}
		if strings.HasPrefix(p, "/api/v1/integrations/") && strings.HasSuffix(p, "/tracking-events") {
			h.serveWebhook(w, r)
			return
		}
		if p == "/api/v1/order/after-sales" || p == "/api/v1/admin/after-sales" || strings.HasPrefix(p, "/api/v1/admin/after-sales/") {
			h.serveAfterSales(w, r)
			return
		}
		if p == "/api/v1/admin/notifications" || strings.HasPrefix(p, "/api/v1/admin/notifications/") || strings.HasPrefix(p, "/api/v1/admin/orders/") && strings.HasSuffix(p, "/label-jobs") || strings.HasPrefix(p, "/api/v1/admin/jobs/") && strings.HasSuffix(p, "/resolve") {
			h.serveIntegrations(w, r)
			return
		}
		if p == "/api/v1/admin/files" || strings.HasPrefix(p, "/api/v1/admin/files/") && strings.HasSuffix(p, "/download") {
			h.servePrivateFiles(w, r, next)
			return
		}
		if p == "/api/v1/admin/import-jobs" || strings.HasPrefix(p, "/api/v1/admin/import-jobs/") ||
			p == "/api/v1/admin/export-jobs" || strings.HasPrefix(p, "/api/v1/admin/jobs/") {
			h.serveJobs(w, r)
			return
		}
		if p == "/api/v1/feedback" || p == "/api/v1/admin/feedback" || strings.HasPrefix(p, "/api/v1/admin/feedback/") {
			OperationsHandler{s, auth}.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
