package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/shipping"
	rules "jstihuo/internal/domain/voucher"
)

type ShippingHandler struct {
	Carriers *shipping.Service
	Orders   *order.Service
	Auth     IdentityHandler
}

// shippingError 用途：输出不包含凭证或请求内容的物流错误。
// 参数：w 响应器；r 当前请求；err 业务或基础设施错误。
// 返回值：无，写入统一 JSON 错误。
func (h ShippingHandler) shippingError(w http.ResponseWriter, r *http.Request, err error) {
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
		w.Header().Set("Retry-After", "900")
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Error{code, "请求未完成", w.Header().Get("X-Request-ID"), []string{}, status >= 500})
}

// shippingBody 用途：解码物流接口的完整 JSON 命令。
// 参数：w 响应器；r 请求；target 目标命令；required 必须出现的非空字段名。
// 返回值：nil 或内容类型、长度、未知字段、缺字段错误。
func shippingBody(w http.ResponseWriter, r *http.Request, target any, required ...string) error {
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return bad
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
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

// shippingFilter 用途：校验承运商、运单与轨迹各自的列表白名单。
// 参数：r 请求；resource 固定资源类型 carriers、shipments 或 events。
// 返回值：固定降序游标筛选或参数错误，不接受自由查询表达式。
func shippingFilter(r *http.Request, resource string) (order.Filter, error) {
	f := order.Filter{Limit: 20}
	bad := &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	for name, values := range r.URL.Query() {
		if len(values) != 1 {
			return f, bad
		}
		v := values[0]
		switch name {
		case "limit":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 100 {
				return f, bad
			}
			f.Limit = n
		case "after_id":
			if !rules.ValidID(v) {
				return f, bad
			}
			f.After = v
		case "sort":
			if v != "id_desc" {
				return f, bad
			}
		case "q":
			if utf8.RuneCountInString(v) > 100 || (resource != "carriers" && v != "") {
				return f, bad
			}
			f.Q = v
		case "status":
			allowed := v == ""
			if resource == "carriers" {
				allowed = allowed || v == "ENABLED" || v == "DISABLED"
			} else if resource == "shipments" {
				allowed = allowed || v == "SHIPPED" || v == "IN_TRANSIT" || v == "DELIVERED" || v == "EXCEPTION"
			}
			if !allowed {
				return f, bad
			}
			f.Status = v
		default:
			return f, bad
		}
	}
	return f, nil
}

// ServeHTTP 用途：处理承运商、现场核销与已授权物流查询。
// 参数：w 响应器；r 请求。
// 返回值：无，写入实际业务结果或统一错误。
// 约束：管理员操作检查权限和 CSRF，顾客轨迹只允许 ORDER 会话访问本单。
func (h ShippingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kind := "order"
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		kind = "admin"
	}
	cookie, err := r.Cookie(h.Auth.cookieName(kind))
	if err != nil {
		h.shippingError(w, r, &identity.Fault{Status: 401, Code: "AUTH_REQUIRED"})
		return
	}
	actor, err := h.Auth.Service.Authenticate(r.Context(), cookie.Value, strings.ToUpper(kind))
	if err == nil && actor.Force {
		err = &identity.Fault{Status: 403, Code: "PASSWORD_CHANGE_REQUIRED"}
	}
	if err == nil && r.Method != "GET" && (r.Header.Get("Origin") != h.Auth.Origin || !h.Auth.Service.CheckCSRF(actor, r.Header.Get("X-CSRF-Token"))) {
		err = &identity.Fault{Status: 403, Code: "CSRF_INVALID"}
	}
	if err != nil {
		h.shippingError(w, r, err)
		return
	}
	permission := "order.read"
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/carriers") {
		permission = "carrier.read"
		if r.Method != "GET" {
			permission = "carrier.write"
		}
	} else if r.URL.Path == "/api/v1/admin/pickups" {
		permission = "order.pickup"
	}
	if kind == "admin" && !actor.Can(permission) {
		h.shippingError(w, r, &identity.Fault{Status: 403, Code: "FORBIDDEN"})
		return
	}
	value, status, err := h.dispatch(w, r, actor)
	if err != nil {
		h.shippingError(w, r, err)
		return
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

// dispatch 用途：按精确路径分派物流读取及到店核销。
// 参数：w 响应器；r 已认证请求；actor 操作主体。
// 返回值：结果、成功状态码或路径、方法、业务错误。
func (h ShippingHandler) dispatch(w http.ResponseWriter, r *http.Request, actor identity.Actor) (any, int, error) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/admin/carriers") {
		return h.carrier(w, r, actor)
	}
	if path == "/api/v1/admin/pickups" && r.Method == "POST" {
		if len(r.URL.Query()) != 0 {
			return nil, 0, &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
		var in order.PickupInput
		if err := shippingBody(w, r, &in, "code", "secret", "items", "reason"); err != nil {
			return nil, 0, err
		}
		v, err := h.Orders.Pickup(r.Context(), actor, in, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
		return v, 200, err
	}
	if r.Method != "GET" {
		return nil, 0, &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	resource := "shipments"
	if strings.HasSuffix(path, "/events") {
		resource = "events"
	}
	f, err := shippingFilter(r, resource)
	if err != nil {
		return nil, 0, err
	}
	if path == "/api/v1/order/shipments" {
		v, err := h.Orders.Shipments(r.Context(), strconv.FormatInt(actor.OrderID, 10), f)
		return v, 200, err
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 6 && parts[3] == "orders" && parts[5] == "shipments" && rules.ValidID(parts[4]) {
		if _, err = h.Orders.Redemption.GetOrder(r.Context(), parts[4], true); err != nil {
			return nil, 0, err
		}
		v, err := h.Orders.Shipments(r.Context(), parts[4], f)
		return v, 200, err
	}
	index := 3
	if actor.Kind == "ADMIN" {
		index = 4
	}
	if len(parts) == index+2 && parts[index+1] == "events" && rules.ValidID(parts[index]) {
		v, err := h.Orders.Events(r.Context(), actor, parts[index], f)
		return v, 200, err
	}
	return nil, 0, &identity.Fault{Status: 404, Code: "NOT_FOUND"}
}

// carrier 用途：处理版本化承运商目录接口。
// 参数：w 响应器；r 已认证请求；actor 有目录权限的管理员。
// 返回值：真实目录或列表、成功状态码或参数、业务错误。
func (h ShippingHandler) carrier(w http.ResponseWriter, r *http.Request, actor identity.Actor) (any, int, error) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/carriers")
	if id != "" {
		id = strings.TrimPrefix(id, "/")
		if !rules.ValidID(id) {
			return nil, 0, &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		}
	}
	if r.Method == "GET" && id == "" {
		f, err := shippingFilter(r, "carriers")
		if err != nil {
			return nil, 0, err
		}
		v, err := h.Carriers.List(r.Context(), shipping.Filter{After: f.After, Limit: f.Limit, Q: f.Q, Status: f.Status})
		return v, 200, err
	}
	if len(r.URL.Query()) != 0 {
		return nil, 0, &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
	}
	if r.Method == "GET" {
		v, err := h.Carriers.Get(r.Context(), id)
		return v, 200, err
	}
	required := []string{"code", "name", "alias", "provider", "kdniao_code", "kuaidi100_code", "wdt_code", "enabled", "secret_ref"}
	var input shipping.CarrierInput
	var version int64
	status := 201
	if r.Method == "POST" && id == "" {
		if err := shippingBody(w, r, &input, required...); err != nil {
			return nil, 0, err
		}
	} else if r.Method == "PATCH" && id != "" {
		var update shipping.CarrierUpdate
		if err := shippingBody(w, r, &update, append(required, "version")...); err != nil {
			return nil, 0, err
		}
		input, version, status = update.CarrierInput, update.Version, 200
	} else {
		return nil, 0, &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	v, err := h.Carriers.Save(r.Context(), actor, id, input, version, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
	return v, status, err
}

// WithShipping 用途：装配承运商、现场核销及物流只读接口。
// 参数：carriers 目录服务；orders 订单服务；auth 身份依赖；next 其余路由。
// 返回值：HTTP Handler，发货与人工完成写操作继续由订单 Handler 处理。
func WithShipping(carriers *shipping.Service, orders *order.Service, auth IdentityHandler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		readShipments := r.Method == "GET" && strings.HasPrefix(p, "/api/v1/admin/orders/") && strings.HasSuffix(p, "/shipments")
		if p == "/api/v1/admin/pickups" || p == "/api/v1/order/shipments" || strings.HasPrefix(p, "/api/v1/admin/carriers") || strings.HasPrefix(p, "/api/v1/shipments/") || strings.HasPrefix(p, "/api/v1/admin/shipments/") || readShipments {
			ShippingHandler{carriers, orders, auth}.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
