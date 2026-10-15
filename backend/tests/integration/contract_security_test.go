package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"jstihuo/internal/app/redemption"
)

type contractOperation struct {
	ID, Method, Path, Permission string
	Security                     []map[string][]string
	JSONBody                     bool `json:"json_body"`
	Query                        []string
}

// TestContractSecurity 用途：从完整 OpenAPI 操作目录执行缺身份、CSRF 与缺字段回归。
// 参数：t 测试上下文，需真实 MySQL 与 make contract 生成的目录。
// 返回值：无，不以静态路由存在替代业务正向测试；目录摘要过期立即失败。
func TestContractSecurity(t *testing.T) {
	raw, err := os.ReadFile("testdata/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var specification struct {
		SHA        string              `json:"sha256"`
		Operations []contractOperation `json:"operations"`
	}
	if err = json.Unmarshal(raw, &specification); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../../docs/development/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	if specification.SHA != hex.EncodeToString(sum[:]) {
		t.Fatal("契约目录已过期，请运行 make contract")
	}
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	owner, _ := f.customer(t, 0)
	response[redemption.Order](t, owner.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	unused, _ := f.customer(t, 1)
	fresh := response[map[string]any](t, unused.call("GET", "session/bootstrap", nil, "preauth", ""), 200)
	unused.tokens["preauth"] = fresh["csrf_token"].(string)
	cookies := map[string]*http.Cookie{}
	for _, client := range []*fulfillmentClient{f.client, owner, unused} {
		for name, cookie := range client.cookies {
			cookies[name] = cookie
		}
	}
	tokens := map[string]string{"admin": f.client.tokens["admin"], "order": owner.tokens["order"], "voucher": unused.tokens["voucher"], "preauth": unused.tokens["preauth"]}
	f.operations.WebhookKeys = map[string][]byte{"MOCK": make([]byte, 32)}
	for _, operation := range specification.Operations {
		t.Run(operation.ID, func(t *testing.T) {
			path := strings.NewReplacer("{id}", "1", "{slug}", "guide", "{provider}", "MOCK").Replace(operation.Path)
			if len(operation.Security) > 0 {
				w := contractRequest(f, operation.Method, path, nil, "", false)
				if w.Code != 401 && w.Code != 403 && w.Code != 404 {
					t.Fatalf("缺少身份应拒绝，实际 %d", w.Code)
				}
			}
			if operation.Method != "GET" && operation.Permission != "provider.scoped" {
				w := contractRequest(f, operation.Method, path, cookies, "invalid-csrf", false)
				if w.Code != 403 {
					t.Fatalf("错误 CSRF 应拒绝，实际 %d", w.Code)
				}
				if operation.JSONBody {
					scope := "admin"
					for _, security := range operation.Security {
						for cookie := range security {
							switch cookie {
							case "preauthCookie":
								scope = "preauth"
							case "voucherCookie":
								scope = "voucher"
							case "orderCookie":
								scope = "order"
							}
						}
					}
					w = contractRequest(f, operation.Method, path, cookies, tokens[scope], true)
					if w.Code != 400 && w.Code != 422 {
						t.Fatalf("缺少字段应拒绝，实际 %d", w.Code)
					}
				}
			}
			if operation.Method == "GET" && len(operation.Query) > 0 {
				w := contractRequest(f, operation.Method, path+"?unexpected_query=1", cookies, "", false)
				if w.Code != 400 {
					t.Fatalf("未知列表参数应拒绝，实际 %d", w.Code)
				}
			}
		})
	}
	t.Logf("已执行 %d 个操作的适用鉴权/CSRF/缺字段/列表白名单边界；业务正向由各工作包集成测试覆盖", len(specification.Operations))
}

// contractRequest 用途：构造不含真实凭证正文的固定边界请求。
// 参数：f 夹具；method 方法；path 已替换路径；cookies 可空身份；csrf 测试令牌；missing 是否发送缺字段 JSON。
// 返回值：内存响应，不记录成功正文或 Cookie。
func contractRequest(f *fulfillmentFixture, method, path string, cookies map[string]*http.Cookie, csrf string, missing bool) *httptest.ResponseRecorder {
	body := ""
	if path == "/api/v1/admin/session/password" && !missing {
		body = `{"current_password":"synthetic-current-123","new_password":"synthetic-next-123"}`
	}
	if missing {
		body = "{}"
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", fmt.Sprintf("contract-%s", testRequestKey(f.client.t)))
	r.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
