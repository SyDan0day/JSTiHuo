package operations

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// TestWebhookSignature 用途：验证供应商独立密钥、时间窗与原始正文完整性。
// 参数：t 测试上下文。
// 返回值：无，错误签名、过期、超长及未配置供应商必须拒绝。
func TestWebhookSignature(t *testing.T) {
	key := bytes.Repeat([]byte{23}, 32)
	now := time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC)
	s := &Service{Now: func() time.Time { return now }, WebhookKeys: map[string][]byte{"MOCK": key}}
	raw := []byte(`{"event_id":"fixture"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(timestamp + "."))
	mac.Write(raw)
	signature := hex.EncodeToString(mac.Sum(nil))
	for _, test := range []struct {
		name, provider, stamp, sig string
		body                       []byte
		valid                      bool
	}{
		{"成功", "MOCK", timestamp, signature, raw, true},
		{"改正文", "MOCK", timestamp, signature, append(append([]byte{}, raw...), 32), false},
		{"错误签名", "MOCK", timestamp, "00", raw, false},
		{"未配置", "KDNIAO", timestamp, signature, raw, false},
		{"旧时间", "MOCK", strconv.FormatInt(now.Unix()-301, 10), signature, raw, false},
		{"未来时间", "MOCK", strconv.FormatInt(now.Unix()+301, 10), signature, raw, false},
		{"非规范时间", "MOCK", "0" + timestamp, signature, raw, false},
		{"超长", "MOCK", timestamp, signature, make([]byte, 256*1024+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := s.VerifyWebhook(test.provider, test.stamp, test.sig, test.body)
			if (err == nil) != test.valid {
				t.Fatal("验签结果不符", err)
			}
		})
	}
	for _, test := range []struct {
		data  map[string]string
		valid bool
	}{
		{map[string]string{"MOCK": base64.StdEncoding.EncodeToString(key)}, true},
		{map[string]string{"OTHER": base64.StdEncoding.EncodeToString(key)}, false},
		{map[string]string{"MOCK": "invalid"}, false},
		{map[string]string{"MOCK": base64.StdEncoding.EncodeToString(key[:16])}, false},
	} {
		raw, _ := json.Marshal(test.data)
		_, err := ParseWebhookKeys(raw)
		if (err == nil) != test.valid {
			t.Fatal("密钥配置校验不符")
		}
	}
}
