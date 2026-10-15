package transport

import "net/http"

type accessResponse struct {
	http.ResponseWriter
	status, bytes int
}

// WriteHeader 用途：记录首个最终 HTTP 状态，不读取响应内容。
// 参数：status 为 HTTP 状态码。
// 返回值：无，重复最终状态沿用 net/http 的首次提交语义。
func (w *accessResponse) WriteHeader(status int) {
	if status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Write 用途：计量响应字节，不缓存凭证、地址或响应正文。
// 参数：data 为原处理器响应字节。
// 返回值：实际写入长度和底层错误。
func (w *accessResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

// Unwrap 用途：供 net/http ResponseController 获取原始流式响应器。
// 参数：无。
// 返回值：原响应器，保持下载及代理能力。
func (w *accessResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// accessLabels 用途：将请求归类为固定日志标签，不记录 URI、查询参数或输入。
// 参数：r 原始请求。
// 返回值：固定方法与接口类别。
func accessLabels(r *http.Request) (string, string) {
	method := r.Method
	switch method {
	case "GET", "POST", "PATCH", "PUT", "DELETE", "HEAD", "OPTIONS":
	default:
		method = "OTHER"
	}
	route := "other"
	if len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/api/v1/" {
		route = "api"
	} else if len(r.URL.Path) >= 7 && r.URL.Path[:7] == "/media/" {
		route = "media"
	} else if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		route = "probe"
	}
	return method, route
}
