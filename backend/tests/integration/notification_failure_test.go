package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/infra/integrations"
)

type testHTTPNotifier struct {
	URL    string
	Client *http.Client
}

// Send 用途：通过本地 HTTP 故障服务验证适配器的确定失败与未知分类。
// 参数：ctx 上下文；key 原事件键；in 无真实联系人的模拟通知。
// 返回值：测试回执或固定错误，未明确声明未执行的 500 按 UNKNOWN 处理。
func (p testHTTPNotifier) Send(ctx context.Context, key string, in integrations.Notice) (string, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return "", integrations.ErrRejected
	}
	r, err := http.NewRequestWithContext(ctx, "POST", p.URL, strings.NewReader(string(raw)))
	if err != nil {
		return "", integrations.ErrRejected
	}
	r.Header.Set("Idempotency-Key", key)
	response, err := p.Client.Do(r)
	if err != nil {
		return "", integrations.ErrUnknown
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		if response.Header.Get("X-Test-Outcome") == "NOT_EXECUTED" {
			return "", integrations.ErrTemporary
		}
		return "", integrations.ErrUnknown
	}
	if response.StatusCode != 200 {
		return "", integrations.ErrRejected
	}
	var result struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil {
		return "", integrations.ErrUnknown
	}
	return result.ID, nil
}

// Lookup 用途：声明故障夹具无法证明未知发送结果。
// 参数：ctx 上下文；key 稳定事件键。
// 返回值：未查到结果；实际供应商应提供可靠查询或工单流程。
func (testHTTPNotifier) Lookup(ctx context.Context, key string) (string, bool, error) {
	return "", false, ctx.Err()
}

// Idempotent 用途：模拟不支持幂等重发的供应商。
// 参数：无。
// 返回值：false。
func (testHTTPNotifier) Idempotent() bool { return false }

// TestNotificationFailures 用途：验证实际 HTTP 500、超时、退避和关闭通知。
// 参数：t 测试上下文，需隔离 MySQL 与本地 httptest 服务。
// 返回值：无，确定未执行才重试，未知结果必须停止自动发送。
func TestNotificationFailures(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	ctx := context.Background()
	c, _ := f.customer(t, 0)
	original := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	var mode, calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch mode.Load() {
		case 0:
			w.Header().Set("X-Test-Outcome", "NOT_EXECUTED")
			w.WriteHeader(500)
		case 1:
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"MOCK-HTTP-RECEIPT"}`)
		case 2:
			select {
			case <-r.Context().Done():
			case <-time.After(200 * time.Millisecond):
			}
		case 3:
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	f.operations.Notifier = testHTTPNotifier{URL: server.URL, Client: &http.Client{Timeout: 50 * time.Millisecond}}
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("通知关闭仍调用外部")
	}
	var suppressed int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE state='SUPPRESSED'").Scan(&suppressed); err != nil || suppressed < 1 {
		t.Fatal("关闭通知缺少 SUPPRESSED 回执", err)
	}
	config := response[operations.Config](t, f.client.call("GET", "admin/config", nil, "admin", ""), 200)
	config.NotificationMode = "MOCK"
	response[operations.Config](t, f.client.call("PUT", "admin/config", config, "admin", testRequestKey(t)), 200)
	claim := enqueueNotice(t, f, original.ID)
	if err := f.operations.ProcessOutbox(ctx, claim); err != nil {
		t.Fatal(err)
	}
	var state string
	var attempts int
	var next time.Time
	if err := f.db.QueryRow("SELECT state,attempts,next_attempt_at FROM outbox_events WHERE id=?", claim.ID).Scan(&state, &attempts, &next); err != nil || state != "PENDING" || attempts != 1 || !next.Equal(f.now.Add(time.Minute)) {
		t.Fatal("确定失败未正确退避", err)
	}
	if n, err := f.operations.RunOutbox(ctx); err != nil || n != 0 || calls.Load() != 1 {
		t.Fatal("未到期通知被自动重发", err)
	}
	mode.Store(1)
	advanceFulfillment(t, f, 61*time.Second)
	if _, err := f.operations.RunOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow("SELECT state FROM notifications WHERE outbox_id=?", claim.ID).Scan(&state); err != nil || state != "SENT" {
		t.Fatal("退避后成功未保存回执", err)
	}
	for _, test := range []struct {
		name string
		mode int32
	}{{"实际 HTTP 超时", 2}, {"未声明未执行的 HTTP 500", 3}} {
		t.Run(test.name, func(t *testing.T) {
			mode.Store(test.mode)
			claim := enqueueNotice(t, f, original.ID)
			if err := f.operations.ProcessOutbox(ctx, claim); err != nil {
				t.Fatal(err)
			}
			if err := f.db.QueryRow("SELECT state FROM notifications WHERE outbox_id=?", claim.ID).Scan(&state); err != nil || state != "UNKNOWN" {
				t.Fatal("外部不确定结果被当作可安全重试", err)
			}
			before := calls.Load()
			if _, err := f.operations.RunOutbox(ctx); err != nil || calls.Load() != before {
				t.Fatal("未知通知被盲目重发", err)
			}
		})
	}
}
