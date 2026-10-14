package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Error struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	RequestID   string   `json:"request_id"`
	FieldErrors []string `json:"field_errors"`
	Retryable   bool     `json:"retryable"`
}

// Router 用途：装配请求标识、安全响应头、探针与 API 超时。
// 参数：ready 就绪探针回调；log 结构化日志器。
// 参数：api 业务 HTTP 处理器。
// 返回值：HTTP Handler。
// 约束：每次请求独立上下文，兑换五秒、下载六十秒，结束取消；日志不记录凭证或请求体。
func Router(ready func(context.Context) error, log *slog.Logger, api ...http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		if _, e := rand.Read(b); e != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int, code string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(Error{code, "请求未完成", id, []string{}, status == 503})
		}
		defer func() {
			if recover() != nil {
				log.Error("request panic", "request_id", id)
				fail(500, "INTERNAL_ERROR")
			}
		}()
		if len(api) > 0 && (strings.HasPrefix(r.URL.Path, "/api/v1/") || strings.HasPrefix(r.URL.Path, "/media/")) {
			timeout := 10 * time.Second
			if r.URL.Path == "/api/v1/redemptions" {
				timeout = 5 * time.Second
			}
			if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v1/admin/files/") && strings.HasSuffix(r.URL.Path, "/download") {
				timeout = 60 * time.Second
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			api[0].ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			fail(404, "NOT_FOUND")
			return
		}
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			fail(405, "METHOD_NOT_ALLOWED")
			return
		}
		if r.URL.Path == "/readyz" {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if ready(ctx) != nil {
				fail(503, "SERVICE_UNAVAILABLE")
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}
