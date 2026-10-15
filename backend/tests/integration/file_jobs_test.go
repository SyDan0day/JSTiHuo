package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/app/voucher"
	crypt "jstihuo/internal/infra/crypto"
)

// testRequestKey 用途：为隔离测试生成互不干扰的业务幂等键。
// 参数：t 测试上下文。
// 返回值：三十二位随机十六进制键，失败终止测试。
func testRequestKey(t *testing.T) string {
	t.Helper()
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw[:])
}

// uploadTestCSV 用途：通过真实 multipart 接口上传 CSV，恶意原文件名不能参与存储。
// 参数：t 测试上下文；f 服务夹具；raw 内存 CSV；key 固定幂等键。
// 返回值：不含原文件正文的文件回执。
func uploadTestCSV(t *testing.T, f *fulfillmentFixture, raw []byte, key string) operations.File {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("kind", "IMPORT"); err != nil {
		t.Fatal(err)
	}
	file, err := form.CreateFormFile("file", "../../credential-source.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/admin/files", &body)
	r.AddCookie(f.client.cookies["jst-admin"])
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("X-CSRF-Token", f.client.tokens["admin"])
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return response[operations.File](t, w, 201)
}

// importTestBatch 用途：创建有完整权益快照的独立草稿批次。
// 参数：t 测试上下文；f 交易夹具；quantity 期望 CSV 行数。
// 返回值：未生成、未激活的草稿批次。
func importTestBatch(t *testing.T, f *fulfillmentFixture, quantity int) voucher.Batch {
	t.Helper()
	ctx := context.Background()
	typ, err := f.vouchers.CreateType(ctx, f.admin, voucher.TypeInput{Name: "导入测试卡型", Mode: "ONE_OF", Count: 1}, testRequestKey(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	batch, err := f.vouchers.CreateBatch(ctx, f.admin, voucher.Input{TypeID: typ.ID, Label: "导入隔离测试", Products: []string{f.products[0]}, Quantity: quantity, CodeLength: 12, CodeMode: "RANDOM", SecretLength: 12, SecretFormat: "MIXED", From: f.now.Add(-time.Hour), Until: f.now.Add(365 * 24 * time.Hour)}, testRequestKey(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

// voucherCSV 用途：构造只驻留内存的合成卡券导入文件。
// 参数：codes 合成卡号集合；secret 测试口令。
// 返回值：UTF-8 CSV，调用方不得输出到日志。
func voucherCSV(codes []string, secret string) []byte {
	var out strings.Builder
	out.WriteString("code,secret,valid_from,valid_until\n")
	for _, code := range codes {
		fmt.Fprintf(&out, "%s,%s,2026-10-01T00:00:00Z,2027-10-01T00:00:00Z\n", code, secret)
	}
	return []byte(out.String())
}

// runFileJob 用途：执行一轮真实文件 worker，并要求确实领取任务。
// 参数：t 测试上下文；f 服务夹具。
// 返回值：无，任何任务错误都终止当前测试。
func runFileJob(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	worked, err := f.operations.RunOnce(context.Background())
	if err != nil || !worked {
		t.Fatal("文件 worker 未完成", worked, err)
	}
}

// getFileJob 用途：经真实后台 API 读取任务状态。
// 参数：t 测试上下文；f 服务夹具；id 任务 ID。
// 返回值：符合 Job 契约的脱敏结果。
func getFileJob(t *testing.T, f *fulfillmentFixture, id string) operations.Job {
	t.Helper()
	return response[operations.Job](t, f.client.call("GET", "admin/jobs/"+id, nil, "admin", ""), 200)
}

// TestFileJobs 用途：验证 G1 加密上传、预检、并发确认、租约、整批回滚、逐行发货和脱敏导出。
// 参数：t 测试上下文，需真实隔离 MySQL。
// 返回值：无，核对任务、库存账、源文件清理及下载权限。
func TestFileJobs(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 2)
	t.Run("卡券整批及租约", func(t *testing.T) { exerciseVoucherFileJobs(t, f) })
	t.Run("失败预检与原始文件隔离", func(t *testing.T) { exerciseInvalidFileJobs(t, f) })
	t.Run("脱敏导出", func(t *testing.T) { exerciseReportJobs(t, f) })
	t.Run("运单逐行恢复", func(t *testing.T) { exerciseShipmentJobs(t, f) })
	checkFulfillmentLedger(t, f.db)
}

// exerciseVoucherFileJobs 用途：验证重复源文件、并发 commit、旧租约与事务故障恢复。
// 参数：t 测试上下文；f 已有商品与管理员的夹具。
// 返回值：无，整批完成前不能出现部分卡券。
func exerciseVoucherFileJobs(t *testing.T, f *fulfillmentFixture) {
	ctx := context.Background()
	batch := importTestBatch(t, f, 2)
	raw := voucherCSV([]string{"IM0000000001", "IM0000000002"}, "ABCDEFGHJK23")
	defer clear(raw)
	key := testRequestKey(t)
	file := uploadTestCSV(t, f, raw, key)
	again := uploadTestCSV(t, f, raw, key)
	if file.ID != again.ID || file.Kind != "IMPORT" || file.State != "STAGED" {
		t.Fatal("上传幂等或状态不符")
	}
	var storage string
	if err := f.db.QueryRow("SELECT storage_key FROM files WHERE id=?", file.ID).Scan(&storage); err != nil {
		t.Fatal(err)
	}
	cipher, err := os.ReadFile(filepath.Join(f.vouchers.Root, storage))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cipher, []byte("ABCDEFGHJK23")) || strings.Contains(storage, "credential-source") {
		t.Fatal("导入秘密或外部文件名落盘")
	}
	in := operations.ImportInput{FileID: file.ID, Kind: "IMPORT_VOUCHERS", BatchID: batch.ID}
	job := response[operations.Job](t, f.client.call("POST", "admin/import-jobs", in, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	job = getFileJob(t, f, job.ID)
	if job.State != "READY" || job.Phase != "PREVIEW" || job.Failed != 0 || job.Processed != 2 {
		t.Fatalf("预检状态不符: %+v", job)
	}
	duplicateFile := uploadTestCSV(t, f, raw, testRequestKey(t))
	in.FileID = duplicateFile.ID
	duplicate := response[operations.Job](t, f.client.call("POST", "admin/import-jobs", in, "admin", testRequestKey(t)), 202)
	if duplicate.ID != job.ID {
		t.Fatal("同内容不同文件未去重")
	}
	var count int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE batch_id=?", batch.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("预检创建了业务卡券", err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := f.operations.CommitImport(ctx, f.admin, job.ID, operations.VersionReason{Version: job.Version, Reason: "并发确认"}, key, "test")
			failures <- err
		}(testRequestKey(t))
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = f.db.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE event_type='job.requested' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.job_id'))=?", job.ID).Scan(&count); err != nil || count != 2 {
		t.Fatal("重复创建 APPLY 事件", count, err)
	}
	old, ok, err := f.operations.ClaimJob(ctx)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err = f.operations.RenewJob(ctx, old); err != nil {
		t.Fatal("固定时钟续租失败", err)
	}
	f.now = f.now.Add(61 * time.Second)
	fresh, ok, err := f.operations.ClaimJob(ctx)
	if err != nil || !ok || fresh.Version <= old.Version {
		t.Fatal("任务未恢复租约", err)
	}
	if err = f.operations.ProcessJob(ctx, old); !errors.Is(err, operations.ErrJobLeaseLost) {
		t.Fatal("旧 worker 未被栅栏拒绝", err)
	}
	f.operations.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "voucher:2" {
			return errors.New("synthetic import failure")
		}
		return nil
	}
	if err = f.operations.ProcessJob(ctx, fresh); err == nil {
		t.Fatal("故障未注入")
	}
	if err = f.db.QueryRow("SELECT COUNT(*) FROM vouchers WHERE batch_id=?", batch.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("整批导入出现部分卡", err)
	}
	f.operations.FaultTx = nil
	if err = f.operations.ProcessJob(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	job = getFileJob(t, f, job.ID)
	if job.State != "READY" || job.Phase != "APPLY" || job.Processed != 2 || job.Failed != 0 {
		t.Fatalf("实际导入结果不符: %+v", job)
	}
	var hash, state string
	var active bool
	if err = f.db.QueryRow("SELECT secret_hash,credential_state,active FROM vouchers WHERE code='IM0000000001'").Scan(&hash, &state, &active); err != nil || !crypt.Verify("ABCDEFGHJK23", hash) || state != "SEALED" || active {
		t.Fatal("导入凭证状态或哈希错误", err)
	}
	if err = f.operations.MaintainFiles(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(f.vouchers.Root, storage)); !os.IsNotExist(err) {
		t.Fatal("已确认导入的源密文未清理")
	}
}

// exerciseInvalidFileJobs 用途：验证错误行只返回脱敏字段，无法绕过预检确认。
// 参数：t 测试上下文；f 服务夹具。
// 返回值：无，原始 CSV 和卡密都不能被下载。
func exerciseInvalidFileJobs(t *testing.T, f *fulfillmentFixture) {
	batch := importTestBatch(t, f, 2)
	raw := voucherCSV([]string{"ER0000000001", "ER0000000002"}, "invalid!")
	defer clear(raw)
	file := uploadTestCSV(t, f, raw, testRequestKey(t))
	job := response[operations.Job](t, f.client.call("POST", "admin/import-jobs", operations.ImportInput{FileID: file.ID, Kind: "IMPORT_VOUCHERS", BatchID: batch.ID}, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	job = getFileJob(t, f, job.ID)
	if job.Failed != 2 || job.ResultFileID == "" {
		t.Fatal("预检未提供错误行文件")
	}
	w := f.client.call("POST", "admin/files/"+job.ResultFileID+"/download", nil, "admin", "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("invalid!")) || bytes.Contains(w.Body.Bytes(), []byte("ER0000000001")) || !bytes.Contains(w.Body.Bytes(), []byte("INVALID_SECRET_FORMAT")) {
		t.Fatal("错误文件泄露输入或未定位错误", w.Code)
	}
	response[map[string]any](t, f.client.call("POST", "admin/import-jobs/"+job.ID+"/commit", operations.VersionReason{Version: job.Version}, "admin", testRequestKey(t)), 422)
	response[map[string]any](t, f.client.call("POST", "admin/files/"+file.ID+"/download", nil, "admin", ""), 403)
}

// exerciseReportJobs 用途：验证元数据导出掩码、受控下载及输入白名单。
// 参数：t 测试上下文；f 服务夹具。
// 返回值：无，不得导出卡密、完整卡号或任意筛选参数。
func exerciseReportJobs(t *testing.T, f *fulfillmentFixture) {
	job := response[operations.Job](t, f.client.call("POST", "admin/export-jobs", operations.ExportInput{Kind: "VOUCHER_METADATA"}, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	job = getFileJob(t, f, job.ID)
	if job.State != "READY" || job.Processed != 4 || job.ResultFileID == "" {
		t.Fatalf("报表未完成: %+v", job)
	}
	w := f.client.call("POST", "admin/files/"+job.ResultFileID+"/download", nil, "admin", "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("IM0000000001")) || bytes.Contains(w.Body.Bytes(), []byte("ABCDEFGHJK23")) || !bytes.Contains(w.Body.Bytes(), []byte("****")) {
		t.Fatal("报表泄露凭证或缺少掩码", w.Code)
	}
	response[map[string]any](t, f.client.call("POST", "admin/export-jobs", map[string]any{"kind": "ORDERS", "include_pii": true}, "admin", testRequestKey(t)), 400)
	response[map[string]any](t, f.client.call("POST", "admin/export-jobs", operations.ExportInput{Kind: "ORDERS", BatchID: "1"}, "admin", testRequestKey(t)), 422)
	response[map[string]any](t, f.client.call("GET", "admin/jobs/"+job.ID+"?q=secret", nil, "admin", ""), 400)
}

// exerciseShipmentJobs 用途：验证预检后状态变化、逐行失败与成功行恢复不重扣。
// 参数：t 测试上下文；f 服务夹具。
// 返回值：无，只有合法行发货一次，失败行明确保留。
func exerciseShipmentJobs(t *testing.T, f *fulfillmentFixture) {
	ctx := context.Background()
	orders := []redemption.Order{}
	for i := range 2 {
		c, _ := f.customer(t, i)
		orders = append(orders, response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201))
	}
	f.now = f.now.Add(24 * time.Hour)
	if _, err := f.db.Exec("UPDATE sessions SET expires_at=?,last_seen_at=? WHERE kind='ADMIN'", f.now.Add(time.Hour), f.now); err != nil {
		t.Fatal(err)
	}
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "G1", Name: "导入测试物流", Provider: "MANUAL", Enabled: true}, "admin", testRequestKey(t)), 201)
	raw := []byte(fmt.Sprintf("order_no,carrier_code,tracking_no,expected_version\n%s,G1,G1-FIRST-001,%d\n%s,G1,G1-SECOND-001,%d\n", orders[0].Number, orders[0].Version, orders[1].Number, orders[1].Version))
	file := uploadTestCSV(t, f, raw, testRequestKey(t))
	job := response[operations.Job](t, f.client.call("POST", "admin/import-jobs", operations.ImportInput{FileID: file.ID, Kind: "IMPORT_SHIPMENTS", CarrierID: carrier.ID}, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	job = getFileJob(t, f, job.ID)
	if job.Failed != 0 {
		t.Fatal("合法运单预检失败")
	}
	response[operations.Job](t, f.client.call("POST", "admin/import-jobs/"+job.ID+"/commit", operations.VersionReason{Version: job.Version}, "admin", testRequestKey(t)), 202)
	if _, err := f.orders.Cancel(ctx, f.admin, orders[1].ID, order.VersionReason{Version: orders[1].Version, Reason: "预检后取消"}, testRequestKey(t), "test"); err != nil {
		t.Fatal(err)
	}
	claim, ok, err := f.operations.ClaimJob(ctx)
	if err != nil || !ok {
		t.Fatal(err)
	}
	f.operations.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "job:finish" {
			return errors.New("synthetic job acknowledgment failure")
		}
		return nil
	}
	if err = f.operations.ProcessJob(ctx, claim); err == nil {
		t.Fatal("未注入结果提交故障")
	}
	f.operations.FaultTx = nil
	if err = f.operations.ProcessJob(ctx, claim); err != nil {
		t.Fatal(err)
	}
	job = getFileJob(t, f, job.ID)
	if job.State != "READY" || job.Phase != "APPLY" || job.Processed != 2 || job.Failed != 1 || job.ResultFileID == "" {
		t.Fatalf("部分结果不符: %+v", job)
	}
	var shipments int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM shipments WHERE order_id=?", orders[0].ID).Scan(&shipments); err != nil || shipments != 1 {
		t.Fatal("恢复任务重复创建运单", err)
	}
	if err = f.operations.ProcessJob(ctx, claim); !errors.Is(err, operations.ErrJobLeaseLost) {
		t.Fatal("完成后旧 worker 未被拒绝", err)
	}
	report := response[operations.Job](t, f.client.call("POST", "admin/export-jobs", operations.ExportInput{Kind: "ORDERS"}, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	report = getFileJob(t, f, report.ID)
	w := f.client.call("POST", "admin/files/"+report.ResultFileID+"/download", nil, "admin", "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("10000000000")) || bytes.Contains(w.Body.Bytes(), []byte("合成测试地址")) || !bytes.Contains(w.Body.Bytes(), []byte("100****0000")) {
		t.Fatal("订单报表未正确脱敏", w.Code)
	}
}

// TestPrivateFileBoundaries 用途：验证短期报表的 owner、存储路径与过期边界。
// 参数：t 测试上下文，需真实隔离 MySQL。
// 返回值：无，缺少 owner 或恶意存储键不能触发读取，过期文件被销毁。
func TestPrivateFileBoundaries(t *testing.T) {
	f := newFulfillment(t)
	ctx := context.Background()
	report := response[operations.Job](t, f.client.call("POST", "admin/export-jobs", operations.ExportInput{Kind: "ORDERS"}, "admin", testRequestKey(t)), 202)
	runFileJob(t, f)
	report = getFileJob(t, f, report.ID)
	var storage string
	if err := f.db.QueryRow("SELECT storage_key FROM files WHERE id=?", report.ResultFileID).Scan(&storage); err != nil {
		t.Fatal(err)
	}
	path := "admin/files/" + report.ResultFileID + "/download"
	if _, err := f.db.Exec("UPDATE files SET owner_admin_id=NULL WHERE id=?", report.ResultFileID); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, f.client.call("POST", path, nil, "admin", ""), 403)
	if _, err := f.db.Exec("UPDATE files SET owner_admin_id=?,storage_key='../../must-not-open.csv' WHERE id=?", f.admin.AdminID, report.ResultFileID); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, f.client.call("POST", path, nil, "admin", ""), 503)
	if _, err := f.db.Exec("UPDATE files SET storage_key=?,expires_at=? WHERE id=?", storage, f.now.Add(-time.Second), report.ResultFileID); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, f.client.call("POST", path, nil, "admin", ""), 410)
	if err := f.operations.MaintainFiles(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.vouchers.Root, storage)); !os.IsNotExist(err) {
		t.Fatal("过期报表密文仍存在")
	}
	var state string
	if err := f.db.QueryRow("SELECT state FROM files WHERE id=?", report.ResultFileID).Scan(&state); err != nil || state != "DELETED" {
		t.Fatal("文件状态未完成清理", err)
	}
}
