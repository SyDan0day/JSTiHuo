package redemption

import (
	"context"
	"database/sql"
	"encoding/json"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/domain/calendar"
	"jstihuo/internal/domain/regions"
	"strconv"
	"strings"
	"time"
)

type OrderItem struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Name      string `json:"name"`
	Code      string `json:"code"`
}
type Recipient struct {
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Region string `json:"region"`
}
type Order struct {
	ID        string      `json:"id"`
	Version   int64       `json:"version"`
	Number    string      `json:"order_no"`
	Status    string      `json:"status"`
	Mode      string      `json:"fulfillment_mode"`
	Date      string      `json:"ship_date,omitempty"`
	Items     []OrderItem `json:"items"`
	Recipient Recipient   `json:"recipient"`
	Note      string      `json:"customer_note"`
	AdminNote *string     `json:"admin_note,omitempty"`
	Deleted   bool        `json:"deleted"`
}
type RecipientData struct {
	Address
	Region string `json:"region"`
}

// OrderAAD 用途：把订单业务号绑定到 PII 密文认证上下文。
// 参数：number 服务器生成的订单号。
// 返回值：固定格式的 AAD 字节。
func OrderAAD(number string) []byte { return []byte("jst:pii:v1:order:" + number) }

// GetOrder 用途：读取冻结订单项与脱敏收件人。
// 参数：ctx 请求上下文；id 目标对象标识。
// 参数：admin 是否包含后台备注。
// 返回值：订单或查询、解密、快照解析错误。
// 约束：调用方必须先验证对象归属，解密缓冲使用后清零，不返回详细地址。
func (s *Service) GetOrder(ctx context.Context, id string, admin bool) (Order, error) {
	o, e := s.readOrderRow(s.DB.QueryRowContext(ctx, "SELECT o.id,o.version,o.order_no,o.status,o.fulfillment_mode,c.ship_date,o.recipient_cipher,o.pii_key_version,o.customer_note,o.admin_note,o.deleted_at IS NOT NULL FROM orders o LEFT JOIN shipping_calendars c ON c.id=o.calendar_id WHERE o.id=?", id), admin)
	if e != nil {
		return o, e
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT id,product_id,quantity,product_snapshot FROM order_items WHERE order_id=? ORDER BY product_id", id)
	if e != nil {
		return o, e
	}
	defer rows.Close()
	o.Items = []OrderItem{}
	for rows.Next() {
		var item OrderItem
		var raw []byte
		if e = rows.Scan(&item.ID, &item.ProductID, &item.Quantity, &raw); e != nil {
			return o, e
		}
		var p struct {
			Name string `json:"name"`
			Code string `json:"code"`
		}
		if e = json.Unmarshal(raw, &p); e != nil {
			return o, e
		}
		item.Name = p.Name
		item.Code = p.Code
		o.Items = append(o.Items, item)
	}
	return o, rows.Err()
}

type orderScanner interface{ Scan(...any) error }

// readOrderRow 用途：解析订单行并按同一策略解密、脱敏收件摘要。
// 参数：row 为数据库行；admin 是否包含后台备注。
// 返回值：脱敏订单或读取、解密错误；解密缓冲在返回前清零。
func (s *Service) readOrderRow(row orderScanner, admin bool) (Order, error) {
	var o Order
	var encrypted []byte
	var keyVersion int
	var date sql.NullTime
	var note string
	e := row.Scan(&o.ID, &o.Version, &o.Number, &o.Status, &o.Mode, &date, &encrypted, &keyVersion, &o.Note, &note, &o.Deleted)
	if e == sql.ErrNoRows {
		return o, fault(404, "NOT_FOUND")
	}
	if e != nil {
		return o, e
	}
	if date.Valid {
		o.Date = date.Time.Format("2006-01-02")
	}
	if admin {
		o.AdminNote = &note
	}
	plain, e := s.PII.Decrypt(encrypted, OrderAAD(o.Number), keyVersion)
	if e != nil {
		return o, e
	}
	defer clear(plain)
	var address RecipientData
	if e = json.Unmarshal(plain, &address); e != nil {
		return o, e
	}
	name := []rune(address.Name)
	if len(name) > 0 {
		o.Recipient.Name = string(name[0]) + "**"
	}
	if len(address.Phone) == 11 {
		o.Recipient.Phone = address.Phone[:3] + "****" + address.Phone[7:]
	}
	o.Recipient.Region = address.Region
	return o, nil
}

// Restricted 用途：判断配送地区是否命中商品禁配集合。
// 参数：excluded 商品禁配代码集合；address 已规范化的完整地址。
// 返回值：任一级命中时为 true。
func Restricted(excluded []string, address Address) bool {
	for _, code := range excluded {
		if code == address.Province || code == address.City || code == address.District {
			return true
		}
	}
	return false
}

// RegionNames 用途：组合可信地区名称。
// 参数：data 部署版本地区数据；a 已验证的省市区地址。
// 返回值：省市区文本。
// 约束：只供订单密文快照使用。
func RegionNames(data regions.Dataset, a Address) string {
	return strings.TrimSpace(data.Label(a.Province, a.City, a.District))
}

// OrderCalendar 用途：提供既有待发配送订单的改期建议。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 返回值：日历或作用域、状态、截止、SQL 错误。
// 约束：只读取自身订单，不借已消费卡重新兑换，不释放旧名额。
func (s *Service) OrderCalendar(ctx context.Context, a identity.Actor) (Calendar, error) {
	if a.Kind != "ORDER" {
		return Calendar{}, fault(403, "SCOPE_REQUIRED")
	}
	o, e := s.GetOrder(ctx, strconv.FormatInt(a.OrderID, 10), false)
	if e != nil {
		return Calendar{}, e
	}
	if o.Status != "PENDING" || o.Mode != "DELIVERY" || o.Deleted {
		return Calendar{}, fault(409, "ILLEGAL_TRANSITION")
	}
	date, e := time.ParseInLocation("2006-01-02", o.Date, calendar.Shanghai)
	if e != nil {
		return Calendar{}, e
	}
	if !s.Now().Before(date.AddDate(0, 0, -1).Add(17 * time.Hour)) {
		return Calendar{}, fault(409, "ILLEGAL_TRANSITION")
	}
	return s.CalendarDays(ctx, o.Date)
}
