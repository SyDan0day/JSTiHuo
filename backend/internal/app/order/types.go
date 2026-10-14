package order

import (
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"time"
)

type QueryCredentials struct {
	Identifier  string `json:"identifier"`
	Phone       string `json:"phone"`
	ChallengeID string `json:"challenge_id"`
	Answer      string `json:"challenge_answer"`
}
type EditInput struct {
	Version int64              `json:"version"`
	Address redemption.Address `json:"address"`
	Date    string             `json:"ship_date"`
	Note    string             `json:"customer_note"`
}
type VersionReason struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}
type Private struct {
	OrderID string             `json:"order_id"`
	Address redemption.Address `json:"address"`
}
type Locked struct {
	ID, Number, Status, Mode, CalendarID string
	Version                              int64
	Deleted                              bool
	Date                                 time.Time
	Cipher                               []byte
	KeyVersion                           int
}

// fault 用途：构造订单领域错误。
// 参数：status HTTP 语义状态；code 固定业务错误码。
// 返回值：typed error。
// 约束：不包含凭证或 PII。
func fault(status int, code string) error { return &identity.Fault{Status: status, Code: code} }
