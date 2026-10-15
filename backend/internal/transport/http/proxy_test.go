package transport

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// TestProxyIPBoundary 用途：验证限流地址只信任显式代理网络且入口值不可歧义。
// 参数：t 测试上下文。
// 返回值：无，直连伪造头忽略，受信代理的非法值拒绝。
func TestProxyIPBoundary(t *testing.T) {
	for _, test := range []struct {
		name, peer, header, want string
		duplicate                bool
		status                   int
	}{
		{"可信代理", "172.20.0.2:5000", "198.51.100.7", "198.51.100.7:5000", false, 200},
		{"直连伪造", "198.51.100.8:5000", "198.51.100.7", "198.51.100.8:5000", false, 200},
		{"非法代理值", "172.20.0.2:5000", "one,two", "", false, 400},
		{"重复代理值", "172.20.0.2:5000", "198.51.100.7", "", true, 400},
		{"IPv6 客户端", "172.20.0.2:5000", "2001:db8::7", "[2001:db8::7]:5000", false, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.RemoteAddr != test.want {
					t.Errorf("限流地址不符：%s", r.RemoteAddr)
				}
				w.WriteHeader(200)
			})
			h := WithTrustedProxy([]netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}, next)
			r := httptest.NewRequest("GET", "/api/v1/session/bootstrap", nil)
			r.RemoteAddr = test.peer
			r.Header.Set("X-Real-IP", test.header)
			if test.duplicate {
				r.Header.Add("X-Real-IP", test.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatal("代理边界状态不符", w.Code)
			}
		})
	}
}
