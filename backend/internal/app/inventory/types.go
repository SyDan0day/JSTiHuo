package inventory

import (
	"jstihuo/internal/app/identity"
	"regexp"
	"time"
)

type Balance struct {
	ProductID string `json:"product_id"`
	Version   int64  `json:"version"`
	OnHand    int64  `json:"on_hand"`
	Reserved  int64  `json:"reserved"`
	Available int64  `json:"available"`
	Warning   int64  `json:"warning_level"`
}
type Line struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
}
type Input struct {
	Direction string `json:"direction"`
	Kind      string `json:"stock_kind"`
	Items     []Line `json:"items"`
	Note      string `json:"note"`
	Proof     string `json:"proof_file_id,omitempty"`
}
type Document struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	Number  string `json:"document_no"`
	State   string `json:"state"`
	Input
}
type Review struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}
type Warning struct {
	Version int64 `json:"version"`
	Level   int64 `json:"warning_level"`
}
type Ledger struct {
	ID            string    `json:"id"`
	ProductID     string    `json:"product_id"`
	Delta         int64     `json:"on_hand_delta"`
	ReservedDelta int64     `json:"reserved_delta"`
	Event         string    `json:"event_key"`
	Reason        string    `json:"reason"`
	Created       time.Time `json:"created_at"`
}
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_after_id,omitempty"`
}
type Filter struct {
	After                         int64
	Limit                         int
	Q, Status, Direction, Product string
}

var idPattern = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

func ValidID(s string) bool       { return idPattern.MatchString(s) }
func fault(s int, c string) error { return &identity.Fault{Status: s, Code: c} }
