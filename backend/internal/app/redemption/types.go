package redemption

import (
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/voucher"
	"time"
)

type Credentials struct {
	Code        string `json:"code"`
	Secret      string `json:"secret"`
	ChallengeID string `json:"challenge_id"`
	Answer      string `json:"challenge_answer"`
}
type Options struct {
	PolicyVersion string           `json:"policy_version"`
	Mode          string           `json:"selection_mode"`
	Count         int              `json:"selection_count"`
	Products      []catalog.Public `json:"products"`
}
type Verification struct {
	Authentic bool             `json:"authentic"`
	Active    bool             `json:"active"`
	Redeemed  bool             `json:"redeemed"`
	From      time.Time        `json:"valid_from"`
	Until     time.Time        `json:"valid_until"`
	Mode      string           `json:"selection_mode"`
	Count     int              `json:"selection_count"`
	Products  []catalog.Public `json:"products"`
	CSRF      string           `json:"csrf_token"`
	Expires   time.Time        `json:"expires_at"`
}
type VerifyResult struct {
	Value   Verification
	Session identity.LoginResult
}
type Card struct {
	ID, BatchID, Hash          string
	Version, CredentialVersion int64
	Active, Redeemed, Deleted  bool
	State, BatchState          string
	From, Until                time.Time
	Policy                     voucher.Policy
}
type Day struct {
	Date      string `json:"date"`
	Remaining int    `json:"remaining"`
	Available bool   `json:"available"`
}
type Calendar struct {
	Items []Day `json:"items"`
}
type Address struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Province string `json:"province_code"`
	City     string `json:"city_code"`
	District string `json:"district_code"`
	Detail   string `json:"detail"`
}
type Selection struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}
type Input struct {
	Policy  string      `json:"policy_version"`
	Items   []Selection `json:"items"`
	Address Address     `json:"address"`
	Date    string      `json:"ship_date"`
	Note    string      `json:"customer_note"`
}

// fault 用途：构造领域错误。
// 参数：status HTTP 语义状态；code 固定业务错误码。
// 返回值：typed error。
// 约束：不记录输入或秘密。
func fault(status int, code string) error { return &identity.Fault{Status: status, Code: code} }
