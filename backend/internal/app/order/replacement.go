package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	rules "jstihuo/internal/domain/voucher"
)

type replacementItem struct {
	OrderItemID string `json:"order_item_id"`
	Quantity    int    `json:"quantity"`
}

// ShipReplacementTx 用途：在调用方授权事务中为已批准案件创建唯一补发并出库。
// 参数：ctx 上下文；tx 当前事务；id 案件；in 案件版本与承运信息。
// 返回值：补发运单或状态、版本、库存、数据库错误；不改变原订单与卡券。
// 并发：先锁原订单和案件，再按商品 ID 锁产品与余额，整单原子扣实存。
func (s *Service) ShipReplacementTx(ctx context.Context, tx *sql.Tx, id string, in ShipInput) (Shipment, error) {
	in.Tracking = strings.TrimSpace(in.Tracking)
	if !rules.ValidID(id) || !rules.ValidID(in.CarrierID) || in.Version < 1 || in.Version > 9007199254740991 || !regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(in.Tracking) {
		return Shipment{}, fault(422, "VALIDATION_FAILED")
	}
	var orderID string
	if err := tx.QueryRowContext(ctx, "SELECT order_id FROM after_sales WHERE id=?", id).Scan(&orderID); err != nil {
		if err == sql.ErrNoRows {
			return Shipment{}, fault(404, "NOT_FOUND")
		}
		return Shipment{}, err
	}
	locked, err := s.lock(ctx, tx, orderID)
	if err != nil {
		return Shipment{}, err
	}
	if locked.Deleted || locked.Mode != "DELIVERY" || (locked.Status != "SHIPPED" && locked.Status != "COMPLETED") {
		return Shipment{}, fault(409, "ILLEGAL_TRANSITION")
	}
	items, err := lockReplacementCase(ctx, tx, id, in.Version)
	if err != nil {
		return Shipment{}, err
	}
	var enabled bool
	err = tx.QueryRowContext(ctx, "SELECT enabled FROM carriers WHERE id=? FOR SHARE", in.CarrierID).Scan(&enabled)
	if err == sql.ErrNoRows || err == nil && !enabled {
		return Shipment{}, fault(422, "VALIDATION_FAILED")
	}
	if err != nil {
		return Shipment{}, err
	}
	stock, err := replacementStock(ctx, tx, orderID, items)
	if err != nil {
		return Shipment{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO shipments(order_id,after_sale_id,shipment_key,kind,carrier_id,tracking_no,shipped_at) VALUES(?,?,?,'REPLACEMENT',?,?,?)`, orderID, id, "after-sale:"+id, in.CarrierID, in.Tracking, s.Now().UTC())
	if err != nil {
		return Shipment{}, err
	}
	number, err := result.LastInsertId()
	if err != nil {
		return Shipment{}, err
	}
	shipmentID := fmt.Sprint(number)
	for index, item := range stock {
		_, err = tx.ExecContext(ctx, "UPDATE inventory_balances SET on_hand=on_hand-?,version=version+1 WHERE product_id=?", item.Quantity, item.Product)
		if err != nil {
			return Shipment{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO inventory_transactions(product_id,on_hand_delta,event_key,order_id,shipment_id,reason) VALUES(?,?,?,?,?,'replacement')`, item.Product, -item.Quantity, "replacement:"+id+":"+item.Product, orderID, shipmentID)
		if err != nil {
			return Shipment{}, err
		}
		if err = s.inject(ctx, tx, fmt.Sprintf("replacement:%d", index+1)); err != nil {
			return Shipment{}, err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE after_sales SET version=version+1 WHERE id=?", id)
	if err != nil {
		return Shipment{}, err
	}
	return Shipment{ID: shipmentID, OrderID: orderID, CarrierID: in.CarrierID, Tracking: in.Tracking, Kind: "REPLACEMENT", Status: "SHIPPED", Shipped: s.Now().UTC()}, nil
}

// lockReplacementCase 用途：锁定已批准且尚未出库的补发案件。
// 参数：ctx 上下文；tx 事务；id 案件；version 期望版本。
// 返回值：冻结补发项或状态、版本、数据错误。
func lockReplacementCase(ctx context.Context, tx *sql.Tx, id string, version int64) ([]replacementItem, error) {
	var state, kind string
	var actual int64
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT state,kind,version,items FROM after_sales WHERE id=? FOR UPDATE", id).Scan(&state, &kind, &actual, &raw)
	if err != nil {
		return nil, err
	}
	if version != actual {
		return nil, fault(409, "VERSION_CONFLICT")
	}
	if state != "APPROVED" || kind != "REPLACEMENT" {
		return nil, fault(409, "ILLEGAL_TRANSITION")
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM shipments WHERE after_sale_id=?", id).Scan(&count); err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, fault(409, "ILLEGAL_TRANSITION")
	}
	var items []replacementItem
	if err = json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fault(409, "CONSISTENCY_CONFLICT")
	}
	return items, nil
}

// replacementStock 用途：核对补发冻结项并按序锁定可用实存。
// 参数：ctx 上下文；tx 事务；orderID 原订单；items 案件冻结项。
// 返回值：按商品排序的出库项或范围、数量、库存错误。
func replacementStock(ctx context.Context, tx *sql.Tx, orderID string, items []replacementItem) ([]reservedItem, error) {
	stock := []reservedItem{}
	seen := map[string]bool{}
	for _, item := range items {
		var original reservedItem
		err := tx.QueryRowContext(ctx, "SELECT id,product_id,quantity FROM order_items WHERE id=? AND order_id=?", item.OrderItemID, orderID).Scan(&original.ID, &original.Product, &original.Quantity)
		if err == sql.ErrNoRows {
			return nil, fault(409, "CONSISTENCY_CONFLICT")
		}
		if err != nil {
			return nil, err
		}
		if item.Quantity < 1 || item.Quantity > original.Quantity || seen[original.Product] {
			return nil, fault(409, "CONSISTENCY_CONFLICT")
		}
		seen[original.Product] = true
		original.Quantity = item.Quantity
		stock = append(stock, original)
	}
	sort.Slice(stock, func(i, j int) bool {
		return len(stock[i].Product) < len(stock[j].Product) || len(stock[i].Product) == len(stock[j].Product) && stock[i].Product < stock[j].Product
	})
	for _, item := range stock {
		var product string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM products WHERE id=? FOR UPDATE", item.Product).Scan(&product); err != nil {
			return nil, err
		}
	}
	for _, item := range stock {
		var available int64
		if err := tx.QueryRowContext(ctx, "SELECT on_hand-reserved FROM inventory_balances WHERE product_id=? FOR UPDATE", item.Product).Scan(&available); err != nil {
			return nil, err
		}
		if available < int64(item.Quantity) {
			return nil, fault(409, "OUT_OF_STOCK")
		}
	}
	return stock, nil
}
