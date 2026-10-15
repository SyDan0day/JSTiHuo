package transport

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	rules "jstihuo/internal/domain/voucher"
)

// serveAdministration 用途：分派固定权限的管理员、角色、配置与运营只读接口。
// 参数：w 响应器；r 已匹配请求。
// 返回值：业务资源或统一错误，不提供任意配置、日志删除或数据库操作。
func (h OperationsHandler) serveAdministration(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	permission := ""
	switch {
	case p == "/api/v1/admin/admins" || strings.HasPrefix(p, "/api/v1/admin/admins/"):
		permission = "admin.read"
		if r.Method != "GET" {
			permission = "admin.write"
		}
	case p == "/api/v1/admin/roles" || strings.HasPrefix(p, "/api/v1/admin/roles/"):
		permission = "role.read"
		if r.Method != "GET" {
			permission = "role.write"
		}
	case p == "/api/v1/admin/permissions":
		permission = "role.read"
	case p == "/api/v1/admin/config":
		permission = "config.read"
		if r.Method != "GET" {
			permission = "config.write"
		}
	case p == "/api/v1/admin/dashboard":
		permission = "dashboard.read"
	case p == "/api/v1/admin/audit-logs":
		permission = "audit.read"
	case p == "/api/v1/admin/files":
		permission = "file.read"
	case p == "/api/v1/admin/inventory/reconciliation":
		permission = "inventory.read"
	default:
		h.operationError(w, r, &identity.Fault{Status: 404, Code: "NOT_FOUND"})
		return
	}
	a, err := h.operationActor(r, "ADMIN", permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	var result any
	status := 200
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	switch {
	case strings.HasPrefix(p, "/api/v1/admin/admins/") && r.Method == "GET":
		if len(r.URL.Query()) > 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.GetAdmin(r.Context(), strings.TrimPrefix(p, "/api/v1/admin/admins/"))
		}
	case strings.HasPrefix(p, "/api/v1/admin/roles/") && r.Method == "GET":
		if len(r.URL.Query()) > 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.GetRole(r.Context(), strings.TrimPrefix(p, "/api/v1/admin/roles/"))
		}
	case p == "/api/v1/admin/inventory/reconciliation" && r.Method == "GET":
		if len(r.URL.Query()) > 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.Reconcile(r.Context())
		}
	case r.Method == "GET" && (p == "/api/v1/admin/admins" || p == "/api/v1/admin/roles" || p == "/api/v1/admin/permissions"):
		var f operations.Filter
		states := []string{"ENABLED", "DISABLED"}
		if p == "/api/v1/admin/permissions" {
			states = nil
		}
		f, err = operationFilter(r, states, true)
		if err == nil {
			switch p {
			case "/api/v1/admin/admins":
				result, err = h.Service.Admins(r.Context(), f)
			case "/api/v1/admin/roles":
				result, err = h.Service.Roles(r.Context(), f)
			default:
				result, err = h.Service.Permissions(r.Context(), a, f)
			}
		}
	case p == "/api/v1/admin/admins" && r.Method == "POST":
		status = 201
		var in operations.AdminInput
		err = operationBody(w, r, &in, "login_name", "temporary_password", "role_ids")
		if err == nil {
			result, err = h.Service.CreateAdmin(r.Context(), a, in, key, rid)
		}
	case strings.HasPrefix(p, "/api/v1/admin/admins/") && r.Method == "PATCH":
		var in operations.AdminUpdate
		err = operationBody(w, r, &in, "version", "enabled", "role_ids")
		if err == nil {
			result, err = h.Service.UpdateAdmin(r.Context(), a, strings.TrimPrefix(p, "/api/v1/admin/admins/"), in, key, rid)
		}
	case p == "/api/v1/admin/roles" && r.Method == "POST":
		status = 201
		var in operations.RoleInput
		err = operationBody(w, r, &in, "name", "enabled", "permission_keys")
		if err == nil {
			result, err = h.Service.CreateRole(r.Context(), a, in, key, rid)
		}
	case strings.HasPrefix(p, "/api/v1/admin/roles/") && r.Method == "PATCH":
		var in operations.RoleUpdate
		err = operationBody(w, r, &in, "version", "name", "enabled", "permission_keys")
		if err == nil {
			result, err = h.Service.UpdateRole(r.Context(), a, strings.TrimPrefix(p, "/api/v1/admin/roles/"), in, key, rid)
		}
	case p == "/api/v1/admin/config" && r.Method == "GET":
		if len(r.URL.Query()) > 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.GetConfig(r.Context())
		}
	case p == "/api/v1/admin/config" && r.Method == "PUT":
		var in operations.Config
		err = operationBody(w, r, &in, "version", "min_lead_days", "horizon_days", "cutoff_hour", "daily_capacity", "blackout_dates", "notification_mode")
		if err == nil {
			result, err = h.Service.UpdateConfig(r.Context(), a, in, key, rid)
		}
	case p == "/api/v1/admin/dashboard" && r.Method == "GET":
		if len(r.URL.Query()) > 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.Dashboard(r.Context())
		}
	case p == "/api/v1/admin/audit-logs" && r.Method == "GET":
		var f operations.AuditFilter
		f, err = auditFilter(r)
		if err == nil {
			result, err = h.Service.Audits(r.Context(), f)
		}
	case p == "/api/v1/admin/files" && r.Method == "GET":
		var f operations.Filter
		f, err = operationFilter(r, []string{"STAGED", "READY", "EXPIRED", "DELETED"}, true)
		if err == nil && f.Q != "" && f.Q != "IMAGE" && f.Q != "IMPORT" && f.Q != "CREDENTIAL_EXPORT" && f.Q != "REPORT" {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		if err == nil {
			result, err = h.Service.Files(r.Context(), a, f)
		}
	default:
		err = &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}

// auditFilter 用途：验证审计动作、结果与对象引用筛选白名单。
// 参数：r 审计列表请求。
// 返回值：筛选或字段、重复查询参数错误。
func auditFilter(r *http.Request) (operations.AuditFilter, error) {
	var out operations.AuditFilter
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	query := r.URL.Query()
	for _, name := range []string{"entity_type", "entity_id"} {
		if len(query[name]) > 1 {
			return out, bad
		}
	}
	out.Entity, out.EntityID = query.Get("entity_type"), query.Get("entity_id")
	if utf8.RuneCountInString(out.Entity) > 50 || out.EntityID != "" && !rules.ValidID(out.EntityID) {
		return out, bad
	}
	if out.Entity != "" {
		valid := false
		for _, entity := range []string{"admin", "role", "order", "product", "category", "carrier", "voucher", "voucher_type", "voucher_batch", "inventory_document", "inventory_balance", "file", "content", "content_page", "feedback", "after_sale", "shipment", "job", "outbox", "config"} {
			valid = valid || strings.EqualFold(entity, out.Entity)
		}
		if !valid {
			return out, bad
		}
	}
	copyRequest := r.Clone(r.Context())
	copyURL := *r.URL
	copyRequest.URL = &copyURL
	query.Del("entity_type")
	query.Del("entity_id")
	copyURL.RawQuery = query.Encode()
	var err error
	out.Filter, err = operationFilter(copyRequest, []string{"SUCCESS", "DENIED", "AUTHORIZED"}, true)
	return out, err
}
