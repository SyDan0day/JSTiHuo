package transport

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
)

// WithTrustedProxy 用途：只从显式信任的代理网络接受由入口覆盖的客户端 IP。
// 参数：prefixes 为部署声明的代理 CIDR；next 为业务处理器。
// 返回值：修正 RemoteAddr 的处理器；公网直连头不会影响限流主体。
// 约束：入口必须覆盖 X-Real-IP，API 端口不能直接向公网发布。
func WithTrustedProxy(prefixes []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		peer, err := netip.ParseAddr(host)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		trusted := false
		for _, prefix := range prefixes {
			trusted = trusted || prefix.Contains(peer.Unmap())
		}
		if !trusted {
			next.ServeHTTP(w, r)
			return
		}
		values := r.Header.Values("X-Real-IP")
		client, err := netip.ParseAddr(r.Header.Get("X-Real-IP"))
		if len(values) != 1 || err != nil || client.Zone() != "" || client.IsUnspecified() {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(Error{Code: "INVALID_PROXY_HEADER", Message: "请求未完成", RequestID: w.Header().Get("X-Request-ID"), FieldErrors: []string{}})
			return
		}
		copyRequest := r.Clone(r.Context())
		copyRequest.RemoteAddr = net.JoinHostPort(client.Unmap().String(), port)
		next.ServeHTTP(w, copyRequest)
	})
}
