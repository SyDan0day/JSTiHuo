package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/voucher"
	password "jstihuo/internal/infra/crypto"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// exerciseVoucherLifecycle 用途：执行真实发行事务、下载锁、重置和 TTL 断言。
// 参数：t Go 测试上下文；db 业务数据库。
// 参数：svc 隔离库业务服务；actor 已认证操作主体。
// 参数：batch 三卡真实哈希测试批次；maximum 千卡事务测试批次。
// 参数：call 本地 HTTP 测试调用器。
// 返回值：无。
// 约束：失败报告测试错误，所有修改仅作用于临时子库。
func exerciseVoucherLifecycle(t *testing.T, db *sql.DB, svc *voucher.Service, actor identity.Actor, batch, maximum voucher.Batch, call func(string, string, any, string, bool) *httptest.ResponseRecorder) {
	t.Helper()
	ctx := context.Background()
	page, e := svc.Vouchers(ctx, voucher.Filter{Batch: batch.ID, Limit: 20})
	if e != nil {
		t.Fatal(e)
	}
	v := page.Items[0]
	if _, e = svc.Bulk(ctx, actor, voucher.BulkInput{IDs: []string{v.ID}, Versions: []int64{v.Version}, Action: "ACTIVATE"}, "c2-premature-activate", "test"); e == nil {
		t.Fatal("unsealed activation")
	}
	w := call("POST", "files/"+batch.FileID+"/download", nil, "", true)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("download", w.Code, w.Body.String())
	}
	original, e := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if e != nil || len(original) != 4 {
		t.Fatal(e)
	}
	// Distinct ownership and live permission gates apply even with a valid session.
	r, e := db.Exec("INSERT INTO admin_users(login_name,password_hash,enabled,force_password_change) SELECT 'other-issuer',password_hash,1,0 FROM admin_users WHERE id=?", actor.AdminID)
	if e != nil {
		t.Fatal(e)
	}
	other, _ := r.LastInsertId()
	db.Exec("UPDATE files SET owner_admin_id=? WHERE id=?", other, batch.FileID)
	if w = call("POST", "files/"+batch.FileID+"/download", nil, "", true); w.Code != 403 {
		t.Fatal("ownership", w.Code)
	}
	db.Exec("UPDATE files SET owner_admin_id=? WHERE id=?", actor.AdminID, batch.FileID)
	sealed, e := svc.SealBatch(ctx, actor, batch.ID, voucher.VersionReason{Version: batch.Version, Reason: "发行完毕"}, "c2-seal-batch-0001", "test")
	if e != nil || sealed.State != "SEALED" {
		t.Fatal(sealed, e)
	}
	if w = call("POST", "files/"+batch.FileID+"/download", nil, "", true); w.Code != 410 {
		t.Fatal("sealed download", w.Code)
	}
	page, e = svc.Vouchers(ctx, voucher.Filter{Batch: batch.ID, Limit: 20})
	if e != nil {
		t.Fatal(e)
	}
	v = page.Items[0]
	if !v.Active || v.State != "SEALED" {
		t.Fatal(v)
	}
	if _, e = svc.Reset(ctx, actor, v.ID, voucher.ResetInput{VersionReason: voucher.VersionReason{Version: v.Version}, Length: 12, Format: "MIXED"}, "c2-reset-active-0001", "test"); e == nil {
		t.Fatal("active reset")
	}
	// A stale version on the second card rolls back the first card's change.
	bulk := voucher.BulkInput{IDs: []string{page.Items[0].ID, page.Items[1].ID}, Versions: []int64{page.Items[0].Version, page.Items[1].Version - 1}, Action: "DEACTIVATE", Reason: "原子性验证"}
	if _, e = svc.Bulk(ctx, actor, bulk, "c2-bulk-conflict-001", "test"); e == nil {
		t.Fatal("stale bulk accepted")
	}
	v, e = svc.GetVoucher(ctx, v.ID)
	if e != nil || !v.Active {
		t.Fatal("partial bulk", v, e)
	}
	disabled, e := svc.Bulk(ctx, actor, voucher.BulkInput{IDs: []string{v.ID}, Versions: []int64{v.Version}, Action: "DEACTIVATE"}, "c2-deactivate-000001", "test")
	if e != nil {
		t.Fatal(e)
	}
	v.Version = disabled.Items[0].Version
	// Revocation removes FK-dependent challenges and sessions together.
	th, ch := sha256.Sum256([]byte("c2-voucher-session")), sha256.Sum256([]byte("c2-voucher-csrf"))
	r, e = db.Exec("INSERT INTO sessions(token_hash,kind,scope,voucher_id,subject_version,csrf_hash,expires_at,last_seen_at) VALUES(?,'VOUCHER','WRITE',?,1,?,?,?)", th[:], v.ID, ch[:], time.Now().Add(time.Hour), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	sid, _ := r.LastInsertId()
	if _, e = db.Exec("INSERT INTO challenges(session_id,purpose,answer_hash,expires_at) VALUES(?,'LOGIN',?,?)", sid, ch[:], time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	var oldHash string
	db.QueryRow("SELECT secret_hash FROM vouchers WHERE id=?", v.ID).Scan(&oldHash)
	resetIn := voucher.ResetInput{VersionReason: voucher.VersionReason{Version: v.Version, Reason: "重置验证"}, Length: 12, Format: "MIXED"}
	delivery, e := svc.Reset(ctx, actor, v.ID, resetIn, "c2-reset-secret-0001", "test")
	if e != nil {
		t.Fatal(e)
	}
	replay, e := svc.Reset(ctx, actor, v.ID, resetIn, "c2-reset-secret-0001", "test")
	if e != nil || delivery.FileID != replay.FileID {
		t.Fatal("reset replay", replay, e)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id=?", sid).Scan(&count)
	if count != 0 {
		t.Fatal("session retained")
	}
	w = call("POST", "files/"+delivery.FileID+"/download", nil, "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	newCSV, e := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if e != nil || len(newCSV) != 2 {
		t.Fatal(e)
	}
	var newHash string
	db.QueryRow("SELECT secret_hash FROM vouchers WHERE id=?", v.ID).Scan(&newHash)
	if newHash == oldHash || !password.Verify(newCSV[1][2], newHash) {
		t.Fatal("new secret mismatch")
	}
	for _, old := range original[1:] {
		if old[1] == newCSV[1][1] && password.Verify(old[2], newHash) {
			t.Fatal("old secret still valid")
		}
	}
	if strings.Contains(mustJSON(t, delivery), "secret") {
		t.Fatal("secret in delivery JSON")
	}
	v, e = svc.GetVoucher(ctx, v.ID)
	if e != nil || v.Active || v.State != "EXPORTABLE" {
		t.Fatal(v, e)
	}
	if w = call("POST", "vouchers/"+v.ID+"/seal", voucher.VersionReason{Version: v.Version, Reason: "单卡发行"}, "c2-single-seal-0001", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("POST", "files/"+delivery.FileID+"/download", nil, "", true); w.Code != 410 {
		t.Fatal(w.Code)
	}
	if e = svc.DestroyExpired(ctx); e != nil {
		t.Fatal(e)
	}
	var storage string
	db.QueryRow("SELECT storage_key FROM files WHERE id=?", delivery.FileID).Scan(&storage)
	if _, e = os.Stat(filepath.Join(svc.Root, storage)); !os.IsNotExist(e) {
		t.Fatal("ciphertext not destroyed", e)
	}
	// Download holds authorization locks until the streaming callback finishes.
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- svc.Download(ctx, actor, maximum.FileID, "stream-lock", func(io.Reader, string) error { close(started); <-release; return nil })
	}()
	select {
	case <-started:
	case e := <-done:
		t.Fatal("stream did not start", e)
	case <-time.After(5 * time.Second):
		t.Fatal("stream timeout")
	}
	sealDone := make(chan error, 1)
	go func() {
		_, e := svc.SealBatch(ctx, actor, maximum.ID, voucher.VersionReason{Version: maximum.Version}, "c2-stream-seal-0001", "test")
		sealDone <- e
	}()
	select {
	case e := <-sealDone:
		close(release)
		t.Fatal("seal passed active stream", e)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if e = <-sealDone; e != nil {
		t.Fatal(e)
	}
	if w = call("POST", "files/"+maximum.FileID+"/download", nil, "", true); w.Code != 410 {
		t.Fatal(w.Code)
	}
	// Reset material automatically seals at TTL; future exports stay impossible.
	v, e = svc.GetVoucher(ctx, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	delivery, e = svc.Reset(ctx, actor, v.ID, voucher.ResetInput{VersionReason: voucher.VersionReason{Version: v.Version}, Length: 12, Format: "DIGITS"}, "c2-expiry-reset-0001", "test")
	if e != nil {
		t.Fatal(e)
	}

	// Rotation retains older keys while new writes use the active version.
	previousKey, previousVersion := svc.ExportKey, svc.KeyVersion
	svc.ExportKey = bytes.Repeat([]byte{4}, 32)
	svc.KeyVersion = 2
	svc.ExportKeys[2] = svc.ExportKey
	if w = call("POST", "files/"+delivery.FileID+"/download", nil, "", true); w.Code != 200 {
		t.Fatal("old key after rotation", w.Code)
	}
	var exportStorage string
	if e = db.QueryRow("SELECT storage_key FROM files WHERE id=?", delivery.FileID).Scan(&exportStorage); e != nil {
		t.Fatal(e)
	}
	filePath := filepath.Join(svc.Root, exportStorage)
	ciphertext, e := os.ReadFile(filePath)
	if e != nil {
		t.Fatal(e)
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1
	if e = os.WriteFile(filePath, tampered, 0600); e != nil {
		t.Fatal(e)
	}
	if w = call("POST", "files/"+delivery.FileID+"/download", nil, "", true); w.Code != 500 {
		t.Fatal("corrupt ciphertext accepted", w.Code)
	}
	if e = os.WriteFile(filePath, ciphertext, 0600); e != nil {
		t.Fatal(e)
	}
	svc.ExportKey, svc.KeyVersion = previousKey, previousVersion
	db.Exec("UPDATE files SET expires_at=? WHERE id=?", time.Now().Add(-time.Minute), delivery.FileID)
	if w = call("POST", "files/"+delivery.FileID+"/download", nil, "", true); w.Code != 410 {
		t.Fatal(w.Code)
	}
	if e = svc.Maintain(ctx); e != nil {
		t.Fatal(e)
	}
	v, e = svc.GetVoucher(ctx, v.ID)
	if e != nil || v.State != "SEALED" || v.Active {
		t.Fatal(v, e)
	}
	if _, e = svc.Edit(ctx, actor, v.ID, voucher.EditInput{VersionReason: voucher.VersionReason{Version: v.Version}, From: time.Now().Add(-time.Hour), Until: time.Now().Add(48 * time.Hour)}, "c2-edit-validity-001", "test"); e != nil {
		t.Fatal(e)
	}
	var auditCount int
	db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='downloadFile'").Scan(&auditCount)
	if auditCount < 3 {
		t.Fatal("missing download audit")
	}
	var fault *identity.Fault
	_, e = svc.Bulk(ctx, actor, voucher.BulkInput{IDs: []string{v.ID, v.ID}, Versions: []int64{v.Version, v.Version}, Action: "DEACTIVATE"}, "c2-duplicate-ids-001", "test")
	if !errors.As(e, &fault) || fault.Status != 422 {
		t.Fatal("duplicate IDs", e)
	}
}

// mustJSON 用途：把合成结果转为 JSON 供安全断言。
// 参数：t Go 测试上下文；v 待验证版本或结果。
// 返回值：JSON 字符串。
// 约束：序列化失败终止当前测试，不处理真实凭证输出。
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
