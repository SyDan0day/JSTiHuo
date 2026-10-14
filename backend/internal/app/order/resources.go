package order

import (
	"context"
	"database/sql"
)

type reservedItem struct {
	ID       string
	Product  string
	Quantity int
}

// orderItems 用途：读取冻结订单项，供改期、取消和后续履约锁定资源。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识。
// 返回值：按商品 ID 升序的订单项或 SQL、一致性错误。
// 约束：关闭 rows，不修改商品集合。
func orderItems(ctx context.Context, tx *sql.Tx, id string) ([]reservedItem, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id,product_id,quantity FROM order_items WHERE order_id=? ORDER BY product_id", id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []reservedItem{}
	for rows.Next() {
		var item reservedItem
		if err = rows.Scan(&item.ID, &item.Product, &item.Quantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fault(409, "CONSISTENCY_CONFLICT")
	}
	return items, nil
}

// lockReservedStock 用途：按商品顺序锁定履约产品及其预占余额。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：items 按商品顺序排列的冻结订单项。
// 返回值：nil 或 SQL、预占不足错误。
// 约束：先全部产品再全部余额，不拒绝已下架的既有履约。
func lockReservedStock(ctx context.Context, tx *sql.Tx, items []reservedItem) error {
	for _, item := range items {
		var productID string
		err := tx.QueryRowContext(ctx,
			"SELECT id FROM products WHERE id=? FOR UPDATE", item.Product,
		).Scan(&productID)
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		var reserved int
		err := tx.QueryRowContext(ctx,
			"SELECT reserved FROM inventory_balances WHERE product_id=? FOR UPDATE", item.Product,
		).Scan(&reserved)
		if err != nil {
			return err
		}
		if reserved < item.Quantity {
			return fault(409, "CONSISTENCY_CONFLICT")
		}
	}
	return nil
}

// releaseItem 用途：把一项 ACTIVE 预占原子改为 RELEASED 并写对应流水。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：orderID 已授权订单标识；item 冻结订单项。
// 参数：reason 业务操作原因。
// 返回值：nil 或状态、数量、SQL 错误。
// 约束：要求余额已锁，唯一 release 事件阻止重复释放。
func releaseItem(
	ctx context.Context, tx *sql.Tx, orderID string, item reservedItem, reason string,
) error {
	var state string
	var quantity int
	err := tx.QueryRowContext(ctx,
		"SELECT state,quantity FROM inventory_reservations WHERE order_item_id=? FOR UPDATE", item.ID,
	).Scan(&state, &quantity)
	if err != nil {
		return err
	}
	if state != "ACTIVE" || quantity != item.Quantity {
		return fault(409, "CONSISTENCY_CONFLICT")
	}
	result, err := tx.ExecContext(ctx,
		"UPDATE inventory_balances SET reserved=reserved-?,version=version+1 WHERE product_id=? AND reserved>=?",
		quantity, item.Product, quantity,
	)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fault(409, "CONSISTENCY_CONFLICT")
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE inventory_reservations SET state='RELEASED',version=version+1 WHERE order_item_id=?", item.ID,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO inventory_transactions(product_id,reserved_delta,event_key,order_id,reason) VALUES(?,?,?,?,?)",
		item.Product, -quantity, "release:"+orderID+":"+item.Product, orderID, reason,
	)
	return err
}
