package operations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"time"

	"jstihuo/internal/app/order"
)

type ProviderEvent struct {
	EventID     string    `json:"event_id"`
	CarrierCode string    `json:"carrier_code"`
	Tracking    string    `json:"tracking_no"`
	Type        string    `json:"event_type"`
	Description string    `json:"description"`
	Occurred    time.Time `json:"occurred_at"`
}

// ParseWebhookKeys 用途：解析部署提供的独立供应商签名密钥白名单。
// 参数：raw 为 provider 到 Base64 密钥的 JSON 对象。
// 返回值：至少 32 字节的密钥映射或配置错误；不输出密钥内容。
func ParseWebhookKeys(raw []byte) (map[string][]byte, error) {
	var encoded map[string]string
	if json.Unmarshal(raw, &encoded) != nil || len(encoded) == 0 {
		return nil, fault(503, "INVALID_WEBHOOK_KEYS")
	}
	keys := map[string][]byte{}
	for provider, value := range encoded {
		if provider != "MOCK" && provider != "KDNIAO" && provider != "KUAIDI100" {
			return nil, fault(503, "INVALID_WEBHOOK_KEYS")
		}
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) < 32 || len(key) > 128 {
			return nil, fault(503, "INVALID_WEBHOOK_KEYS")
		}
		keys[provider] = key
	}
	return keys, nil
}

// VerifyWebhook 用途：对有界原始请求体进行供应商独立 HMAC 验签。
// 参数：provider 固定供应商；timestamp Unix 秒；signature 十六进制签名；raw 原始正文。
// 返回值：nil 或未配置、超长、过期、签名错误；不依赖浏览器 Cookie。
func (s *Service) VerifyWebhook(provider, timestamp, signature string, raw []byte) error {
	key := s.WebhookKeys[provider]
	if len(key) < 32 {
		return fault(503, "INTEGRATION_DISABLED")
	}
	if len(raw) > 256*1024 {
		return fault(413, "PAYLOAD_TOO_LARGE")
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strconv.FormatInt(seconds, 10) != timestamp {
		return fault(401, "INVALID_SIGNATURE")
	}
	now := s.Now().Unix()
	if seconds < now-300 || seconds > now+300 {
		return fault(401, "INVALID_SIGNATURE")
	}
	decoded, err := hex.DecodeString(signature)
	if err != nil || len(decoded) != sha256.Size {
		return fault(401, "INVALID_SIGNATURE")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(timestamp + "."))
	mac.Write(raw)
	if !hmac.Equal(mac.Sum(nil), decoded) {
		return fault(401, "INVALID_SIGNATURE")
	}
	return nil
}

// RecordProviderEvent 用途：将验签回调映射为公开安全的物流事实。
// 参数：ctx 上下文；provider 已验签供应商；in 严格解析的事件。
// 返回值：nil 或字段、承运商归属、重复内容冲突错误。
// 约束：丢弃自由描述，使用固定事件文案，防止详细地址进入公开轨迹。
func (s *Service) RecordProviderEvent(ctx context.Context, provider string, in ProviderEvent) error {
	if !validText(in.EventID, 1, 120) || !validText(in.CarrierCode, 1, 40) || !validText(in.Description, 0, 300) || !regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(in.Tracking) || in.Occurred.IsZero() {
		return fault(422, "VALIDATION_FAILED")
	}
	descriptions := map[string]string{"IN_TRANSIT": "运输中", "DELIVERED": "已签收", "EXCEPTION": "物流异常，请联系承运商"}
	description, ok := descriptions[in.Type]
	if !ok {
		return fault(422, "VALIDATION_FAILED")
	}
	if provider == "MOCK" {
		description = "MOCK 模拟轨迹：" + description
	}
	var shipmentID string
	err := s.DB.QueryRowContext(ctx, "SELECT s.id FROM shipments s JOIN carriers c ON c.id=s.carrier_id WHERE c.provider=? AND c.code=? AND s.tracking_no=?", provider, in.CarrierCode, in.Tracking).Scan(&shipmentID)
	if err == sql.ErrNoRows {
		return fault(404, "NOT_FOUND")
	}
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(in.EventID))
	return s.Orders.RecordTracking(ctx, order.TrackingInput{ShipmentID: shipmentID, ProviderID: provider + ":" + hex.EncodeToString(digest[:]), Type: in.Type, Description: description, Occurred: in.Occurred})
}
