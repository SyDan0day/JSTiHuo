package integration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"jstihuo/internal/app/operations"
)

// exerciseTrackingWebhook 用途：验证真实回调签名、持久去重与描述隐私。
// 参数：t 测试上下文；f 夹具；tracking 已发 MOCK 运单号。
// 返回值：无，重复事件不重复入库，同 ID 改事实冲突，错误承运范围拒绝。
func exerciseTrackingWebhook(t *testing.T, f *fulfillmentFixture, tracking string) {
	t.Helper()
	key := bytes.Repeat([]byte{21}, 32)
	f.operations.WebhookKeys = map[string][]byte{"MOCK": key}
	in := operations.ProviderEvent{EventID: "signed-event-1", CarrierCode: "MOCK", Tracking: tracking, Type: "DELIVERED", Description: "不应公开的详细地址 10000000000", Occurred: f.now.Add(time.Minute)}
	call := func(event operations.ProviderEvent, timestamp string, valid bool) int {
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(timestamp + "."))
		mac.Write(raw)
		sig := hex.EncodeToString(mac.Sum(nil))
		if !valid {
			sig = strings.Repeat("0", 64)
		}
		r := httptest.NewRequest("POST", "/api/v1/integrations/MOCK/tracking-events", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Timestamp", timestamp)
		r.Header.Set("X-Signature", sig)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		return w.Code
	}
	timestamp := strconv.FormatInt(f.now.Unix(), 10)
	if code := call(in, timestamp, false); code != 401 {
		t.Fatal("错误签名未拒绝", code)
	}
	if code := call(in, strconv.FormatInt(f.now.Unix()-301, 10), true); code != 401 {
		t.Fatal("过期回调未拒绝", code)
	}
	for i := 0; i < 2; i++ {
		if code := call(in, timestamp, true); code != 204 {
			t.Fatal("回调或重复回调失败", code)
		}
	}
	var count int
	var description string
	if err := f.db.QueryRow("SELECT COUNT(*),MAX(description) FROM shipment_events WHERE provider_event_id LIKE 'MOCK:%'").Scan(&count, &description); err != nil || count != 1 || strings.Contains(description, "10000000000") {
		t.Fatal("回调重复或描述泄露", err)
	}
	in.Occurred = in.Occurred.Add(time.Second)
	if code := call(in, timestamp, true); code != 409 {
		t.Fatal("重复事件改变事实未冲突", code)
	}
	in.EventID = "cross-carrier"
	in.CarrierCode = "OTHER"
	if code := call(in, timestamp, true); code != 404 {
		t.Fatal("回调越承运商读取", code)
	}
}
