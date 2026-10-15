package integration

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
)

// TestRecoverySafety 用途：验证真实恢复 CLI 的目标防误用、事务回滚和外部结果隔离。
// 参数：t 测试上下文，需隔离 MySQL 与本机 Go 工具链。
// 返回值：无，恢复后旧会话、私有下载和潜在重复外部调用必须关闭。
func TestRecoverySafety(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	c, _ := f.customer(t, 0)
	original := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	file := uploadTestCSV(t, f, []byte("code,secret,valid_from,valid_until\n"), testRequestKey(t))
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MOCK", Name: "恢复模拟面单", Provider: "MOCK", Enabled: true}, "admin", testRequestKey(t)), 201)
	config := response[operations.Config](t, f.client.call("GET", "admin/config", nil, "admin", ""), 200)
	config.CarrierID = carrier.ID
	response[operations.Config](t, f.client.call("PUT", "admin/config", config, "admin", testRequestKey(t)), 200)
	job := response[operations.Job](t, f.client.call("POST", "admin/orders/"+original.ID+"/label-jobs", operations.VersionReason{Version: original.Version, Reason: "恢复隔离测试"}, "admin", testRequestKey(t)), 202)
	dir := t.TempDir()
	binary := filepath.Join(dir, "recovery")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/recovery")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("恢复 CLI 编译失败：%s", raw)
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TEST_DB_DSN"))
	if err != nil {
		t.Fatal("测试 DSN 不合法")
	}
	var database string
	if err = f.db.QueryRow("SELECT DATABASE()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	cfg.DBName = database
	dsnFile := filepath.Join(dir, "dsn")
	if err = os.WriteFile(dsnFile, []byte(cfg.FormatDSN()), 0600); err != nil {
		t.Fatal(err)
	}
	call := func(target string) ([]byte, error) {
		return exec.Command(binary, "sanitize", "--dsn-file", dsnFile, "--restored-database", target).CombinedOutput()
	}
	challenge, answer := c.challenge("FEEDBACK")
	response[operations.Feedback](t, c.call("POST", "feedback", operations.FeedbackInput{Name: "恢复测试", Phone: "10000000000", Message: "校验恢复密文", ChallengeID: challenge, Answer: answer}, "preauth", ""), 200)
	piiFile := filepath.Join(dir, "pii.json")
	ring := map[string]any{"active": "1", "keys": map[string]string{"1": base64.StdEncoding.EncodeToString(f.operations.PII.Keys[1])}}
	encoded, err := json.Marshal(ring)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(piiFile, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	verify := func() ([]byte, error) {
		return exec.Command(binary, "verify-db", "--dsn-file", dsnFile, "--pii-keyring-file", piiFile).CombinedOutput()
	}
	if raw, err := verify(); err != nil || !strings.Contains(string(raw), `"pii_records_checked":2`) {
		t.Fatal("订单与反馈恢复密文校验失败", string(raw))
	}
	wrong := append([]byte{}, f.operations.PII.Keys[1]...)
	wrong[0] ^= 1
	ring["keys"] = map[string]string{"1": base64.StdEncoding.EncodeToString(wrong)}
	encoded, err = json.Marshal(ring)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(piiFile, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = verify(); err == nil {
		t.Fatal("错误 PII 密钥通过恢复校验")
	}
	if _, err = call("wrong_restore_target"); err == nil {
		t.Fatal("恢复 CLI 没有检查目标库")
	}
	if _, err = f.db.Exec("CREATE TRIGGER fail_recovery_audit BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected'"); err != nil {
		t.Fatal(err)
	}
	if _, err = call(database); err == nil {
		t.Fatal("恢复收敛未随审计失败回滚")
	}
	var state string
	var sessions int
	if err = f.db.QueryRow("SELECT state FROM files WHERE id=?", file.ID).Scan(&state); err != nil || state != "STAGED" {
		t.Fatal("恢复故障修改了文件状态", err)
	}
	if err = f.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil || sessions == 0 {
		t.Fatal("恢复故障撤销了会话", err)
	}
	if _, err = f.db.Exec("DROP TRIGGER fail_recovery_audit"); err != nil {
		t.Fatal(err)
	}
	raw, err := call(database)
	if err != nil || !strings.Contains(string(raw), `"balanced":true`) {
		t.Fatal("恢复 CLI 收敛失败")
	}
	if err = f.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("恢复后旧会话仍有效", err)
	}
	if err = f.db.QueryRow("SELECT state FROM files WHERE id=?", file.ID).Scan(&state); err != nil || state != "EXPIRED" {
		t.Fatal("恢复后私有文件仍可用", err)
	}
	if err = f.db.QueryRow("SELECT state FROM integration_jobs WHERE id=?", job.ID).Scan(&state); err != nil || state != "UNKNOWN" {
		t.Fatal("恢复后面单可能被重复创建", err)
	}
	var unknown int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM notifications n JOIN outbox_events o ON o.id=n.outbox_id WHERE n.state='UNKNOWN' AND o.state='DEAD'").Scan(&unknown); err != nil || unknown < 1 {
		t.Fatal("恢复后通知未隔离", err)
	}
	if err = f.db.QueryRow("SELECT status FROM orders WHERE id=?", original.ID).Scan(&state); err != nil || state != "PENDING" {
		t.Fatal("恢复修改了订单状态", err)
	}
	response[map[string]any](t, f.client.call("GET", "admin/orders", nil, "admin", ""), 401)
	checkFulfillmentLedger(t, f.db)
}
