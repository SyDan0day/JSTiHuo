package redemption

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	rules "jstihuo/internal/domain/voucher"
)

// GetOrders 用途：以固定次数批量查询读取列表所需的订单与冻结明细。
// 参数：ctx 上下文；ids 已授权订单引用，最多一百；admin 是否包含后台备注。
// 返回值：按输入顺序排列的脱敏订单或字段、查询、解密错误。
// 事务：只读一致快照，关闭订单 rows 后再查询明细，不逐单往返数据库。
func (s *Service) GetOrders(ctx context.Context, ids []string, admin bool) ([]Order, error) {
	out := []Order{}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > 100 {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		if !rules.ValidID(id) {
			return nil, fault(422, "INVALID_ARGUMENT")
		}
		args[i] = id
		marks[i] = "?"
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := `SELECT o.id,o.version,o.order_no,o.status,o.fulfillment_mode,c.ship_date,o.recipient_cipher,o.pii_key_version,o.customer_note,o.admin_note,o.deleted_at IS NOT NULL
		FROM orders o LEFT JOIN shipping_calendars c ON c.id=o.calendar_id WHERE o.id IN (` + strings.Join(marks, ",") + ")"
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	orders := map[string]*Order{}
	for rows.Next() {
		value, e := s.readOrderRow(rows, admin)
		if e != nil {
			err = e
			break
		}
		value.Items = []OrderItem{}
		orders[value.ID] = &value
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT order_id,id,product_id,quantity,product_snapshot FROM order_items WHERE order_id IN ("+strings.Join(marks, ",")+") ORDER BY order_id,product_id", args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var orderID string
		var item OrderItem
		var raw []byte
		if err = rows.Scan(&orderID, &item.ID, &item.ProductID, &item.Quantity, &raw); err != nil {
			break
		}
		var snapshot struct {
			Name string `json:"name"`
			Code string `json:"code"`
		}
		if err = json.Unmarshal(raw, &snapshot); err != nil {
			break
		}
		item.Name, item.Code = snapshot.Name, snapshot.Code
		if target := orders[orderID]; target != nil {
			target.Items = append(target.Items, item)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		target := orders[id]
		if target == nil {
			return nil, fault(404, "NOT_FOUND")
		}
		out = append(out, *target)
	}
	return out, tx.Commit()
}
