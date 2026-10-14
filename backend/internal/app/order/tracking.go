package order

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

type TrackingInput struct {
	ShipmentID, ProviderID, Type, Description string
	Occurred                                  time.Time
}

// Shipments 用途：分页读取已授权订单的全部运单。
// 参数：ctx 请求上下文；id 订单标识；filter 固定游标与运单状态筛选。
// 返回值：运单页或 SQL 错误，不返回其他订单运单，不触发供应商调用。
func (s *Service) Shipments(ctx context.Context, id string, filter Filter) (ShipmentPage, error) {
	query := "SELECT id,order_id,carrier_id,tracking_no,kind,status,shipped_at FROM shipments WHERE order_id=?"
	args := []any{id}
	if filter.After != "" {
		query += " AND id<?"
		args = append(args, filter.After)
	}
	if filter.Status != "" {
		query += " AND status=?"
		args = append(args, filter.Status)
	}
	args = append(args, filter.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return ShipmentPage{}, err
	}
	defer rows.Close()
	page := ShipmentPage{Items: []Shipment{}}
	for rows.Next() {
		var value Shipment
		if err = rows.Scan(&value.ID, &value.OrderID, &value.CarrierID, &value.Tracking, &value.Kind, &value.Status, &value.Shipped); err != nil {
			return page, err
		}
		page.Items = append(page.Items, value)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		page.Next = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}

// Events 用途：读取运单轨迹并在顾客访问时验证订单归属。
// 参数：ctx 请求上下文；actor 管理员或 ORDER 主体；id 运单标识；filter 固定游标。
// 返回值：实际轨迹页或作用域、归属、不存在及 SQL 错误，未收到事件时为空。
func (s *Service) Events(ctx context.Context, actor identity.Actor, id string, filter Filter) (TrackingPage, error) {
	var orderID string
	err := s.DB.QueryRowContext(ctx, "SELECT order_id FROM shipments WHERE id=?", id).Scan(&orderID)
	if err == sql.ErrNoRows {
		return TrackingPage{}, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return TrackingPage{}, err
	}
	if actor.Kind == "ORDER" {
		if orderID != strconv.FormatInt(actor.OrderID, 10) {
			return TrackingPage{}, fault(404, "NOT_FOUND")
		}
	} else if actor.Kind != "ADMIN" || !actor.Can("order.read") {
		return TrackingPage{}, fault(403, "FORBIDDEN")
	}
	query := "SELECT id,event_type,description,occurred_at FROM shipment_events WHERE shipment_id=?"
	args := []any{id}
	if filter.After != "" {
		query += " AND id<?"
		args = append(args, filter.After)
	}
	args = append(args, filter.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return TrackingPage{}, err
	}
	defer rows.Close()
	page := TrackingPage{Items: []TrackingEvent{}}
	for rows.Next() {
		var value TrackingEvent
		if err = rows.Scan(&value.ID, &value.Type, &value.Description, &value.Occurred); err != nil {
			return page, err
		}
		page.Items = append(page.Items, value)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		page.Next = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}

// RecordTracking 用途：记录已被可信适配器验证的物流事件并单调推进状态。
// 参数：ctx 请求上下文；input 已授权运单、供应商事件标识、类型、说明和发生时间。
// 返回值：nil 或字段、对象、重复内容冲突及 SQL 错误。
// 约束：只能由 MOCK 或验签适配器调用，不提供无鉴权浏览器写入口。
// 事务：事件去重、轨迹状态、原发订单完成和审计一次提交，库存不变。
func (s *Service) RecordTracking(ctx context.Context, input TrackingInput) error {
	if !rules.ValidID(input.ShipmentID) || strings.TrimSpace(input.ProviderID) == "" || len(input.ProviderID) > 120 || utf8.RuneCountInString(input.Description) > 300 || input.Occurred.IsZero() {
		return fault(422, "VALIDATION_FAILED")
	}
	switch input.Type {
	case "IN_TRANSIT", "DELIVERED", "EXCEPTION":
	default:
		return fault(422, "VALIDATION_FAILED")
	}
	input.Occurred = input.Occurred.UTC().Truncate(time.Microsecond)
	var orderID string
	if err := s.DB.QueryRowContext(ctx, "SELECT order_id FROM shipments WHERE id=?", input.ShipmentID).Scan(&orderID); err != nil {
		if err == sql.ErrNoRows {
			return fault(404, "NOT_FOUND")
		}
		return err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	locked, err := s.lock(ctx, tx, orderID)
	if err != nil {
		return err
	}
	var status, kind string
	var shipped time.Time
	var caseID sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT status,kind,shipped_at,after_sale_id FROM shipments WHERE id=? FOR UPDATE", input.ShipmentID).Scan(&status, &kind, &shipped, &caseID)
	if err != nil {
		return err
	}
	if input.Occurred.Before(shipped) {
		return fault(422, "VALIDATION_FAILED")
	}
	duplicate, err := insertTrackingEvent(ctx, tx, input)
	if err != nil {
		return err
	}
	if duplicate {
		return tx.Commit()
	}
	var latest sql.NullTime
	if err = tx.QueryRowContext(ctx, "SELECT MAX(occurred_at) FROM shipment_events WHERE shipment_id=? AND provider_event_id<>?", input.ShipmentID, input.ProviderID).Scan(&latest); err != nil {
		return err
	}
	current := !latest.Valid || !input.Occurred.Before(latest.Time)
	if status != "DELIVERED" && current {
		if err = s.advanceTracking(ctx, tx, locked, input, kind, caseID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','shipment.event','shipment',?,'tracking-adapter',JSON_OBJECT('event_type',?),'SUCCESS')", input.ShipmentID, input.Type)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// insertTrackingEvent 用途：按供应商事件标识验证并去重轨迹记录。
// 参数：ctx 请求上下文；tx 当前事务；input 已验证的事件内容。
// 返回值：是否为相同重复事件或内容冲突、SQL 错误；同标识不能覆盖旧事实。
func insertTrackingEvent(ctx context.Context, tx *sql.Tx, input TrackingInput) (bool, error) {
	var eventType, description string
	var occurred time.Time
	err := tx.QueryRowContext(ctx, "SELECT event_type,description,occurred_at FROM shipment_events WHERE shipment_id=? AND provider_event_id=?", input.ShipmentID, input.ProviderID).Scan(&eventType, &description, &occurred)
	if err == nil {
		if eventType != input.Type || description != input.Description || !occurred.Equal(input.Occurred) {
			return false, fault(409, "IDEMPOTENCY_CONFLICT")
		}
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO shipment_events(shipment_id,provider_event_id,event_type,description,occurred_at) VALUES(?,?,?,?,?)", input.ShipmentID, input.ProviderID, input.Type, input.Description, input.Occurred)
	return false, err
}

// advanceTracking 用途：应用未过时的物流状态及合法完成转换。
// 参数：ctx 请求上下文；tx 当前事务；locked 订单快照；input 新事件；kind 运单类型；caseID 可空售后标识。
// 返回值：nil 或 SQL 错误；原发只使 SHIPPED 变 COMPLETED，补发签收只关闭已批准案件。
func (s *Service) advanceTracking(ctx context.Context, tx *sql.Tx, locked Locked, input TrackingInput, kind string, caseID sql.NullString) error {
	var delivered any
	if input.Type == "DELIVERED" {
		delivered = input.Occurred
	}
	_, err := tx.ExecContext(ctx, "UPDATE shipments SET status=?,delivered_at=COALESCE(?,delivered_at),version=version+1 WHERE id=?", input.Type, delivered, input.ShipmentID)
	if err != nil {
		return err
	}
	if input.Type != "DELIVERED" {
		return nil
	}
	if kind == "PRIMARY" && locked.Status == "SHIPPED" {
		_, err = tx.ExecContext(ctx, "UPDATE orders SET status='COMPLETED',completed_at=?,version=version+1 WHERE id=?", input.Occurred, locked.ID)
		if err != nil {
			return err
		}
		return s.event(ctx, tx, "order.completed", locked.ID, locked.Version+1)
	}
	if kind == "REPLACEMENT" && caseID.Valid {
		_, err = tx.ExecContext(ctx, "UPDATE after_sales SET state='CLOSED',closed_at=?,version=version+1 WHERE id=? AND state='APPROVED'", input.Occurred, caseID.String)
	}
	return err
}
