package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/domain/calendar"
	rules "jstihuo/internal/domain/voucher"
)

type ShipInput struct {
	Version   int64  `json:"version"`
	CarrierID string `json:"carrier_id"`
	Tracking  string `json:"tracking_no"`
}
type Shipment struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	CarrierID string    `json:"carrier_id"`
	Tracking  string    `json:"tracking_no"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Shipped   time.Time `json:"shipped_at"`
}
type ShipmentPage struct {
	Items []Shipment `json:"items"`
	Next  string     `json:"next_after_id,omitempty"`
}
type TrackingEvent struct {
	ID          string    `json:"id"`
	Type        string    `json:"event_type"`
	Description string    `json:"description"`
	Occurred    time.Time `json:"occurred_at"`
}
type TrackingPage struct {
	Items []TrackingEvent `json:"items"`
	Next  string          `json:"next_after_id,omitempty"`
}

// Ship 用途：幂等确认待发配送订单的全单人工发货。
// 参数：ctx 请求上下文；actor 管理员；id 订单标识；input 版本、承运商与单号。
// 参数：key 幂等键；requestID 审计请求标识。
// 返回值：唯一主运单或权限、版本、日期、状态、单号冲突及数据库错误。
// 事务：实存和预占双减、预占消费、运单、订单状态、流水与 Outbox 一次提交。
func (s *Service) Ship(
	ctx context.Context, actor identity.Actor, id string, input ShipInput, key, requestID string,
) (Shipment, error) {
	input.Tracking = strings.TrimSpace(input.Tracking)
	validVersion := input.Version >= 1 && input.Version <= 9007199254740991
	validTracking := regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(input.Tracking)
	if !rules.ValidID(id) || !rules.ValidID(input.CarrierID) || !validVersion || !validTracking {
		return Shipment{}, fault(422, "VALIDATION_FAILED")
	}
	err := s.mutate(ctx, actor, "shipOrder", "order.ship", id, input, key, requestID, func(tx *sql.Tx) error { return s.shipTx(ctx, tx, id, input) })
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		err = fault(409, "CONFLICT")
	}
	if err != nil {
		return Shipment{}, err
	}
	return s.PrimaryShipment(ctx, id)
}

// shipTx 用途：协调订单发货的状态校验、库存消费与主运单发布。
// 参数：ctx 请求上下文；tx 当前事务；id 已授权订单；input 发货命令。
// 返回值：nil 或状态、日期、资源、SQL 错误，调用方负责整体提交。
// 并发：订单、售后、产品、余额、预占与运单按固定顺序锁定，禁止部分发货。
func (s *Service) shipTx(ctx context.Context, tx *sql.Tx, id string, input ShipInput) error {
	locked, err := s.lock(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = checkPending(locked, input.Version); err != nil {
		return err
	}
	labelID, err := checkShippingLabel(ctx, tx, id, input)
	if err != nil {
		return err
	}
	if calendar.Day(s.Redemption.Now()).Format("2006-01-02") < locked.Date.Format("2006-01-02") {
		return fault(409, "SHIP_DATE_NOT_REACHED")
	}
	var cases int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM after_sales WHERE order_id=? AND state IN ('OPEN','APPROVED')", id).Scan(&cases)
	if err != nil {
		return err
	}
	if cases > 0 {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	var enabled bool
	err = tx.QueryRowContext(ctx, "SELECT enabled FROM carriers WHERE id=? FOR SHARE", input.CarrierID).Scan(&enabled)
	if err == sql.ErrNoRows {
		return fault(422, "VALIDATION_FAILED")
	}
	if err != nil {
		return err
	}
	if !enabled {
		return fault(422, "VALIDATION_FAILED")
	}
	items, err := orderItems(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = lockReservedStock(ctx, tx, items); err != nil {
		return err
	}
	for index, item := range items {
		if err = consumeReservedItem(ctx, tx, id, item); err != nil {
			return err
		}
		if err = s.inject(ctx, tx, fmt.Sprintf("ship:%d", index+1)); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx,
		"INSERT INTO shipments(order_id,shipment_key,kind,carrier_id,tracking_no,shipped_at) VALUES(?,?,'PRIMARY',?,?,?)",
		id, "primary:"+id, input.CarrierID, input.Tracking, s.Redemption.Now().UTC(),
	)
	if err != nil {
		return err
	}
	shipmentID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE inventory_transactions SET shipment_id=? WHERE order_id=? AND reason='shipping' AND event_key LIKE ?",
		shipmentID, id, "ship:"+id+":%",
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE orders SET status='SHIPPED',shipped_at=?,version=version+1 WHERE id=?", s.Redemption.Now().UTC(), id)
	if err != nil {
		return err
	}
	if labelID != "" {
		if _, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET options=JSON_SET(options,'$.confirmed',JSON_EXTRACT('true','$')),version=version+1 WHERE id=?", labelID); err != nil {
			return err
		}
	}
	return s.event(ctx, tx, "shipment.created", id, locked.Version+1)
}

// consumeReservedItem 用途：消费一项 ACTIVE 预占并记录真实出库流水。
// 参数：ctx 请求上下文；tx 当前事务；orderID 订单标识；item 冻结订单项。
// 返回值：nil 或预占一致性、余额、SQL 错误，要求产品与库存已按序锁定。
func consumeReservedItem(ctx context.Context, tx *sql.Tx, orderID string, item reservedItem) error {
	var state string
	var quantity int
	err := tx.QueryRowContext(ctx, "SELECT state,quantity FROM inventory_reservations WHERE order_item_id=? FOR UPDATE", item.ID).Scan(&state, &quantity)
	if err != nil {
		return err
	}
	if state != "ACTIVE" || quantity != item.Quantity {
		return fault(409, "CONSISTENCY_CONFLICT")
	}
	result, err := tx.ExecContext(ctx, "UPDATE inventory_balances SET on_hand=on_hand-?,reserved=reserved-?,version=version+1 WHERE product_id=? AND on_hand>=? AND reserved>=?", quantity, quantity, item.Product, quantity, quantity)
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
	_, err = tx.ExecContext(ctx, "UPDATE inventory_reservations SET state='CONSUMED',version=version+1 WHERE order_item_id=?", item.ID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO inventory_transactions(product_id,on_hand_delta,reserved_delta,event_key,order_id,reason) VALUES(?,?,?,?,?,'shipping')", item.Product, -quantity, -quantity, "ship:"+orderID+":"+item.Product, orderID)
	return err
}

// PrimaryShipment 用途：读取配送订单的唯一主运单。
// 参数：ctx 请求上下文；id 已授权订单标识。
// 返回值：主运单或不存在、数据库错误，不触发外部物流查询。
func (s *Service) PrimaryShipment(ctx context.Context, id string) (Shipment, error) {
	var shipment Shipment
	err := s.DB.QueryRowContext(ctx, "SELECT id,order_id,carrier_id,tracking_no,kind,status,shipped_at FROM shipments WHERE shipment_key=?", "primary:"+id).Scan(&shipment.ID, &shipment.OrderID, &shipment.CarrierID, &shipment.Tracking, &shipment.Kind, &shipment.Status, &shipment.Shipped)
	if err == sql.ErrNoRows {
		err = fault(404, "NOT_FOUND")
	}
	return shipment, err
}

// Complete 用途：有理由地人工确认已发配送订单完成。
// 参数：ctx 请求上下文；actor 管理员；id 订单；input 版本与确认原因；key 幂等键；requestID 请求标识。
// 返回值：脱敏订单或权限、版本、状态、SQL 错误；不虚构签收轨迹、不再次扣库存。
func (s *Service) Complete(
	ctx context.Context, actor identity.Actor, id string, input VersionReason, key, requestID string,
) (redemption.Order, error) {
	if !rules.ValidID(id) || !validReason(input) {
		return redemption.Order{}, fault(422, "VALIDATION_FAILED")
	}
	err := s.mutate(ctx, actor, "orderComplete", "order.complete", id, input, key, requestID, func(tx *sql.Tx) error {
		locked, err := s.lock(ctx, tx, id)
		if err != nil {
			return err
		}
		if locked.Version != input.Version {
			return fault(409, "VERSION_CONFLICT")
		}
		if locked.Status != "SHIPPED" || locked.Mode != "DELIVERY" || locked.Deleted {
			return fault(409, "ILLEGAL_TRANSITION")
		}
		var shipmentID string
		if err = tx.QueryRowContext(ctx, "SELECT id FROM shipments WHERE shipment_key=? FOR UPDATE", "primary:"+id).Scan(&shipmentID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE orders SET status='COMPLETED',completed_at=?,version=version+1 WHERE id=?", s.Redemption.Now().UTC(), id)
		if err != nil {
			return err
		}
		return s.event(ctx, tx, "order.completed", id, locked.Version+1)
	})
	if err != nil {
		return redemption.Order{}, err
	}
	return s.Redemption.GetOrder(ctx, id, true)
}
