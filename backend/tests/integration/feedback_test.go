package integration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"jstihuo/internal/app/operations"
)

// TestFeedback 用途：验证真实公开反馈、一次挑战、联系人密文与后台处理闭环。
// 参数：t 测试上下文，需隔离 MySQL。
// 返回值：无，禁止重复挑战创建、联系人泄露、越权处理或重复审计。
func TestFeedback(t *testing.T) {
	f := newFulfillment(t)
	c := f.client
	challenge, answer := c.challenge("FEEDBACK")
	in := operations.FeedbackInput{Name: "测试联系人", Phone: "10000000000", Email: "test@example.invalid", OrderNo: "O_UNKNOWN", Message: "<script>window.bad=1</script>配送咨询", ChallengeID: challenge, Answer: answer}
	created := response[operations.Feedback](t, c.call("POST", "feedback", in, "preauth", ""), 200)
	if created.ID == "" || created.State != "OPEN" || created.Version != 1 {
		t.Fatal("反馈回执不符")
	}
	response[map[string]any](t, c.call("POST", "feedback", in, "preauth", ""), 422)
	var cipher []byte
	var version int
	var associated bool
	if err := f.db.QueryRow("SELECT contact_cipher,pii_key_version,order_id IS NOT NULL FROM feedback WHERE id=?", created.ID).Scan(&cipher, &version, &associated); err != nil {
		t.Fatal(err)
	}
	if associated || bytes.Contains(cipher, []byte(in.Phone)) || bytes.Contains(cipher, []byte(in.Name)) {
		t.Fatal("联系人未加密或参考订单产生授权关联")
	}
	plain, err := f.orders.Redemption.PII.Decrypt(cipher, []byte("feedback:"+created.ID), version)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(plain)
	var contact map[string]string
	if json.Unmarshal(plain, &contact) != nil || contact["Phone"] != in.Phone || contact["OrderNo"] != in.OrderNo {
		t.Fatal("联系人密文不能正确恢复")
	}
	page := response[operations.Page[operations.Feedback]](t, c.call("GET", "admin/feedback?status=OPEN", nil, "admin", ""), 200)
	if len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatal("反馈列表未持久化")
	}
	for _, query := range []string{"status=ENABLED", "limit=0", "after_id=01", "sort=message", "q=a&q=b", "phone=10000000000"} {
		response[map[string]any](t, c.call("GET", "admin/feedback?"+query, nil, "admin", ""), 400)
	}
	resolve := operations.FeedbackResolve{Version: 1, Resolution: "已核实处理"}
	path := "admin/feedback/" + created.ID + "/resolve"
	resolved := response[operations.Feedback](t, c.call("POST", path, resolve, "admin", "feedback-resolve-01"), 200)
	if resolved.State != "RESOLVED" || resolved.Version != 2 {
		t.Fatal("处理状态或版本不符")
	}
	replayed := response[operations.Feedback](t, c.call("POST", path, resolve, "admin", "feedback-resolve-01"), 200)
	if replayed != resolved {
		t.Fatal("处理重放不是原响应")
	}
	var reference string
	if err = f.db.QueryRow("SELECT CAST(result_ref AS CHAR) FROM idempotency_keys WHERE request_key='feedback-resolve-01'").Scan(&reference); err != nil {
		t.Fatal(err)
	}
	var fields map[string]string
	if json.Unmarshal([]byte(reference), &fields) != nil || len(fields) != 1 || fields["id"] != created.ID {
		t.Fatal("幂等表必须只保存对象引用")
	}
	response[map[string]any](t, c.call("POST", path, resolve, "admin", "feedback-stale-001"), 409)
	resolve.Resolution = "不同请求"
	response[map[string]any](t, c.call("POST", path, resolve, "admin", "feedback-resolve-01"), 409)
	var audits int
	var raw string
	if err = f.db.QueryRow("SELECT COUNT(*),CAST(JSON_ARRAYAGG(diff) AS CHAR) FROM audit_logs WHERE entity_type='feedback'").Scan(&audits, &raw); err != nil {
		t.Fatal(err)
	}
	if audits != 2 || strings.Contains(raw, in.Phone) || strings.Contains(raw, in.Message) {
		t.Fatal("反馈审计重复或泄露输入")
	}
	if _, err = f.db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='feedback.write'"); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, c.call("POST", path, resolve, "admin", "feedback-forbid-01"), 403)
	delete(c.cookies, "jst-preauth")
	response[map[string]any](t, c.call("POST", "feedback", in, "preauth", ""), 401)
}
