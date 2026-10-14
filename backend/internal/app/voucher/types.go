package voucher

import (
	"jstihuo/internal/app/identity"
	"time"
)

type TypeInput struct {
	Name  string `json:"name"`
	Mode  string `json:"selection_mode"`
	Count int    `json:"selection_count"`
}
type Type struct {
	ID       string `json:"id"`
	Version  int64  `json:"version"`
	Revision int    `json:"revision"`
	Enabled  bool   `json:"enabled"`
	TypeInput
}
type Input struct {
	TypeID       string    `json:"type_id"`
	Label        string    `json:"label"`
	Products     []string  `json:"product_ids"`
	Quantity     int       `json:"quantity"`
	Prefix       string    `json:"code_prefix"`
	CodeLength   int       `json:"code_length"`
	CodeMode     string    `json:"code_mode"`
	Start        string    `json:"start_serial,omitempty"`
	SecretLength int       `json:"secret_length"`
	SecretFormat string    `json:"secret_format"`
	From         time.Time `json:"valid_from"`
	Until        time.Time `json:"valid_until"`
	Activate     bool      `json:"activate_on_seal"`
}
type Policy struct {
	TypeID   string   `json:"type_id"`
	Revision int      `json:"revision"`
	Mode     string   `json:"selection_mode"`
	Count    int      `json:"selection_count"`
	Products []string `json:"product_ids"`
}
type Batch struct {
	ID        string `json:"id"`
	Version   int64  `json:"version"`
	Number    string `json:"batch_no"`
	State     string `json:"state"`
	Generated int    `json:"generated_count"`
	FileID    string `json:"export_file_id,omitempty"`
	Input
}
type Voucher struct {
	ID       string    `json:"id"`
	Version  int64     `json:"version"`
	BatchID  string    `json:"batch_id"`
	Code     string    `json:"code"`
	Active   bool      `json:"active"`
	State    string    `json:"credential_state"`
	From     time.Time `json:"valid_from"`
	Until    time.Time `json:"valid_until"`
	Redeemed bool      `json:"redeemed"`
	Products []string  `json:"product_ids"`
}
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_after_id,omitempty"`
}
type Filter struct {
	After            string
	Limit            int
	Q, Status, Batch string
	Active, Redeemed *bool
}

// fault 用途：构造卡券领域错误。
// 参数：status HTTP 语义状态；code 固定业务错误码。
// 返回值：typed error。
// 约束：不包含凭证输入。
func fault(status int, code string) error { return &identity.Fault{Status: status, Code: code} }
