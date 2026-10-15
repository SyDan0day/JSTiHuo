package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
)

// serveAfterSales 用途：分派顾客本单售后和管理员案件处理。
// 参数：w 响应器；r 售后请求。
// 返回值：案件、运单或统一错误，所有写操作校验 CSRF。
func (h OperationsHandler) serveAfterSales(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	customer := p == "/api/v1/order/after-sales"
	kind, permission := "ADMIN", "after_sale.read"
	if customer {
		kind, permission = "ORDER", ""
	} else if r.Method == "POST" {
		permission = "after_sale.write"
		if strings.HasSuffix(p, "/shipments") {
			permission = "order.ship"
		}
	}
	a, err := h.operationActor(r, kind, permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	var result any
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	base := "/api/v1/admin/after-sales"
	switch {
	case customer && r.Method == "POST":
		var in operations.AfterSaleInput
		err = operationBody(w, r, &in, "kind", "reason", "items")
		if err == nil {
			result, err = h.Service.CreateAfterSale(r.Context(), a, in, key, rid)
		}
	case (customer || p == base) && r.Method == "GET":
		var f operations.Filter
		f, err = operationFilter(r, []string{"OPEN", "APPROVED", "REJECTED", "CLOSED"}, !customer)
		if err == nil {
			result, err = h.Service.AfterSales(r.Context(), a, f)
		}
	case !customer && strings.HasSuffix(p, "/decision") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, base+"/"), "/decision")
		var in operations.AfterSaleDecision
		err = operationBody(w, r, &in, "version", "decision", "resolution")
		if err == nil {
			result, err = h.Service.DecideAfterSale(r.Context(), a, id, in, key, rid)
		}
	case !customer && strings.HasSuffix(p, "/shipments") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, base+"/"), "/shipments")
		var in order.ShipInput
		err = operationBody(w, r, &in, "version", "carrier_id", "tracking_no")
		if err == nil {
			result, err = h.Service.ReplacementShipment(r.Context(), a, id, in, key, rid)
		}
	case !customer && strings.HasPrefix(p, base+"/") && r.Method == "GET":
		result, err = h.Service.GetAfterSale(r.Context(), strings.TrimPrefix(p, base+"/"))
	default:
		err = &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	json.NewEncoder(w).Encode(result)
}

// serveIntegrations 用途：分派面单创建、任务核对和通知处理。
// 参数：w 响应器；r 运营请求。
// 返回值：任务、通知或统一错误。
func (h OperationsHandler) serveIntegrations(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	permission := "notification.read"
	status := 200
	if strings.HasSuffix(p, "/label-jobs") {
		permission, status = "order.ship", 202
	} else if strings.HasPrefix(p, "/api/v1/admin/jobs/") {
		permission = "integration.manage"
	} else if strings.HasSuffix(p, "/retry") {
		permission, status = "notification.retry", 202
	} else if strings.HasSuffix(p, "/resolve") {
		permission = "notification.manage"
	}
	a, err := h.operationActor(r, "ADMIN", permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	var result any
	switch {
	case strings.HasSuffix(p, "/label-jobs") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/admin/orders/"), "/label-jobs")
		var in operations.VersionReason
		err = operationBody(w, r, &in, "version", "reason")
		if err == nil {
			result, err = h.Service.AutoLabel(r.Context(), a, id, in, key, rid)
		}
	case strings.HasPrefix(p, "/api/v1/admin/jobs/") && strings.HasSuffix(p, "/resolve") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/admin/jobs/"), "/resolve")
		var in operations.ResolveJob
		err = operationBody(w, r, &in, "version", "resolution", "evidence_reference", "reason")
		if err == nil {
			result, err = h.Service.ResolveLabel(r.Context(), a, id, in, key, rid)
		}
	case p == "/api/v1/admin/notifications" && r.Method == "GET":
		var f operations.Filter
		f, err = operationFilter(r, []string{"PENDING", "SENT", "FAILED", "UNKNOWN", "SUPPRESSED"}, true)
		if err == nil {
			result, err = h.Service.Notifications(r.Context(), f)
		}
	case strings.HasSuffix(p, "/retry") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/admin/notifications/"), "/retry")
		var in operations.VersionReason
		err = operationBody(w, r, &in, "version", "reason")
		if err == nil {
			result, err = h.Service.RetryNotification(r.Context(), a, id, in, key, rid)
		}
	case strings.HasSuffix(p, "/resolve") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/admin/notifications/"), "/resolve")
		var in operations.ResolveNotification
		err = operationBody(w, r, &in, "version", "resolution", "evidence_reference", "reason")
		if err == nil {
			result, err = h.Service.ResolveNotice(r.Context(), a, id, in, key, rid)
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

// serveWebhook 用途：先验证原始正文签名，再严格解析物流回调。
// 参数：w 响应器；r 供应商 POST 请求。
// 返回值：204 或统一错误，不接受 Cookie 替代签名。
func (h OperationsHandler) serveWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.operationError(w, r, &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
		return
	}
	provider := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/integrations/"), "/tracking-events")
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256*1024))
	if err != nil {
		h.operationError(w, r, &identity.Fault{Status: 413, Code: "PAYLOAD_TOO_LARGE"})
		return
	}
	defer clear(raw)
	if err = h.Service.VerifyWebhook(provider, r.Header.Get("X-Timestamp"), r.Header.Get("X-Signature"), raw); err != nil {
		h.operationError(w, r, err)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var in operations.ProviderEvent
	err = operationBody(w, r, &in, "event_id", "carrier_code", "tracking_no", "event_type", "description", "occurred_at")
	if err == nil {
		err = h.Service.RecordProviderEvent(r.Context(), provider, in)
	}
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	w.WriteHeader(204)
}
