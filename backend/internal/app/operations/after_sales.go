package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	rules "jstihuo/internal/domain/voucher"
)

type CaseItem struct {
	OrderItemID string `json:"order_item_id"`
	Quantity    int    `json:"quantity"`
}
type AfterSaleInput struct {
	Kind   string     `json:"kind"`
	Reason string     `json:"reason"`
	Items  []CaseItem `json:"items"`
}
type AfterSale struct {
	ID         string `json:"id"`
	Version    int64  `json:"version"`
	OrderID    string `json:"order_id"`
	State      string `json:"state"`
	Resolution string `json:"resolution"`
	AfterSaleInput
}
type AfterSaleDecision struct {
	Version    int64  `json:"version"`
	Decision   string `json:"decision"`
	Resolution string `json:"resolution"`
}

// loadCase 用途：读取售后案件，不读取订单收件信息。
// 参数：ctx 上下文；q 数据源；id 案件；lock 是否锁定。
// 返回值：完整案件或不存在、JSON、数据库错误。
func loadCase(ctx context.Context, q queryer, id string, lock bool) (AfterSale, error) {
	var out AfterSale
	var items []byte
	query := "SELECT id,version,order_id,state,resolution,kind,reason,items FROM after_sales WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Version, &out.OrderID, &out.State, &out.Resolution, &out.Kind, &out.Reason, &items)
	if err == sql.ErrNoRows {
		return out, fault(404, "NOT_FOUND")
	}
	if err == nil {
		err = json.Unmarshal(items, &out.Items)
	}
	return out, err
}

// CreateAfterSale 用途：为本单 WRITE 会话创建唯一活动售后案件。
// 参数：ctx 上下文；a 顾客主体；in 原因与冻结订单项；key 幂等键；rid 请求标识。
// 返回值：案件 JSON 或范围、状态、数量、并发冲突错误。
// 事务：先锁原订单，再验证活动案件与原单商品，不改变原发库存或卡券。
func (s *Service) CreateAfterSale(ctx context.Context, a identity.Actor, in AfterSaleInput, key, rid string) (json.RawMessage, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if a.Kind != "ORDER" || !validText(in.Reason, 1, 500) || in.Items == nil || len(in.Items) > 99 ||
		(in.Kind != "COMPLAINT" && in.Kind != "REPLACEMENT") || (in.Kind == "COMPLAINT" && len(in.Items) != 0) || (in.Kind == "REPLACEMENT" && len(in.Items) == 0) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	seen := map[string]bool{}
	for _, item := range in.Items {
		if !rules.ValidID(item.OrderItemID) || item.Quantity != 1 || seen[item.OrderItemID] {
			return nil, fault(422, "VALIDATION_FAILED")
		}
		seen[item.OrderItemID] = true
	}
	id := fmt.Sprint(a.OrderID)
	command := mutation{Operation: "customerAfterSale", Entity: "after_sale", Key: key, RequestID: rid, Body: in, CustomerOrderID: id}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		if err := lockCaseOrder(ctx, tx, id); err != nil {
			return nil, "", err
		}
		var active int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM after_sales WHERE order_id=? AND state IN ('OPEN','APPROVED')", id).Scan(&active); err != nil {
			return nil, "", err
		}
		if active > 0 {
			return nil, "", fault(409, "ILLEGAL_TRANSITION")
		}
		for _, item := range in.Items {
			var quantity int
			err := tx.QueryRowContext(ctx, "SELECT quantity FROM order_items WHERE id=? AND order_id=?", item.OrderItemID, id).Scan(&quantity)
			if err == sql.ErrNoRows || err == nil && quantity < item.Quantity {
				return nil, "", fault(422, "VALIDATION_FAILED")
			}
			if err != nil {
				return nil, "", err
			}
		}
		raw, err := json.Marshal(in.Items)
		if err != nil {
			return nil, "", err
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO after_sales(order_id,kind,reason,items,resolution) VALUES(?,?,?,?,'')", id, in.Kind, in.Reason, raw)
		if err != nil {
			return nil, "", err
		}
		number, err := result.LastInsertId()
		if err != nil {
			return nil, "", err
		}
		caseID := fmt.Sprint(number)
		if err = s.orderEvent(ctx, tx, "after_sale.created", id, caseID, 1); err != nil {
			return nil, "", err
		}
		out, err := loadCase(ctx, tx, caseID, false)
		return out, caseID, err
	})
}

// lockCaseOrder 用途：以原订单锁串行化案件、补发和签收。
// 参数：ctx 上下文；tx 当前事务；id 原订单。
// 返回值：nil 或非已发配送单、已匿名化、不存在、数据库错误。
func lockCaseOrder(ctx context.Context, tx *sql.Tx, id string) error {
	var status, mode string
	var deleted, retired bool
	err := tx.QueryRowContext(ctx, "SELECT status,fulfillment_mode,deleted_at IS NOT NULL,phone_lookup=UNHEX(REPEAT('00',32)) FROM orders WHERE id=? FOR UPDATE", id).Scan(&status, &mode, &deleted, &retired)
	if err == sql.ErrNoRows {
		return fault(404, "NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if deleted || retired || mode != "DELIVERY" || (status != "SHIPPED" && status != "COMPLETED") {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	return nil
}

// AfterSales 用途：按管理员或顾客本单作用域分页读取售后。
// 参数：ctx 上下文；a 已认证主体；f 固定降序筛选。
// 返回值：案件页或权限、数据库错误。
func (s *Service) AfterSales(ctx context.Context, a identity.Actor, f Filter) (Page[AfterSale], error) {
	page := Page[AfterSale]{Items: []AfterSale{}}
	query := "SELECT id FROM after_sales WHERE 1=1"
	args := []any{}
	if a.Kind == "ORDER" {
		query += " AND order_id=?"
		args = append(args, a.OrderID)
	} else if a.Kind != "ADMIN" || !a.Can("after_sale.read") {
		return page, fault(403, "FORBIDDEN")
	}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND state=?"
		args = append(args, f.Status)
	}
	if f.Q != "" {
		query += " AND CAST(order_id AS CHAR)=?"
		args = append(args, f.Q)
	}
	args = append(args, f.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return page, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return page, err
	}
	for _, id := range ids {
		out, err := loadCase(ctx, s.DB, id, false)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, out)
	}
	if len(page.Items) > f.Limit {
		page.Items = page.Items[:f.Limit]
		page.Next = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}

// GetAfterSale 用途：读取后台案件详情。
// 参数：ctx 上下文；id 已校验的案件 ID。
// 返回值：案件或参数、不存在、数据库错误。
func (s *Service) GetAfterSale(ctx context.Context, id string) (AfterSale, error) {
	if !rules.ValidID(id) {
		return AfterSale{}, fault(422, "INVALID_ARGUMENT")
	}
	return loadCase(ctx, s.DB, id, false)
}

// DecideAfterSale 用途：批准补发、拒绝申请或关闭投诉。
// 参数：ctx 上下文；a 管理员；id 案件；in 完整版本与处理说明；key 幂等键；rid 请求标识。
// 返回值：案件 JSON 或非法状态、版本、权限错误。
// 约束：补发批准后仅实际签收关闭，不允许人工跳过履约。
func (s *Service) DecideAfterSale(ctx context.Context, a identity.Actor, id string, in AfterSaleDecision, key, rid string) (json.RawMessage, error) {
	in.Resolution = strings.TrimSpace(in.Resolution)
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !validText(in.Resolution, 1, 1000) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	prior, err := loadCase(ctx, s.DB, id, false)
	if err != nil {
		return nil, err
	}
	command := mutation{Operation: "decideAfterSale", Target: id, Permission: "after_sale.write", Entity: "after_sale", Key: key, RequestID: rid, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		if err := lockCaseOrder(ctx, tx, prior.OrderID); err != nil {
			return nil, "", err
		}
		out, err := loadCase(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if out.Version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		state := ""
		if out.State == "OPEN" {
			switch in.Decision {
			case "APPROVE":
				if out.Kind == "REPLACEMENT" {
					state = "APPROVED"
				}
			case "REJECT":
				state = "REJECTED"
			case "CLOSE":
				if out.Kind == "COMPLAINT" {
					state = "CLOSED"
				}
			}
		}
		if state == "" {
			return nil, "", fault(409, "ILLEGAL_TRANSITION")
		}
		var closed any
		if state == "CLOSED" || state == "REJECTED" {
			closed = s.Now().UTC()
		}
		_, err = tx.ExecContext(ctx, "UPDATE after_sales SET state=?,resolution=?,handled_by=?,closed_at=?,version=version+1 WHERE id=?", state, in.Resolution, a.AdminID, closed, id)
		if err != nil {
			return nil, "", err
		}
		if err = s.orderEvent(ctx, tx, "after_sale.decided", out.OrderID, id, out.Version+1); err != nil {
			return nil, "", err
		}
		out, err = loadCase(ctx, tx, id, false)
		return out, id, err
	})
}

// ReplacementShipment 用途：为已批准案件原子出库并创建唯一补发运单。
// 参数：ctx 上下文；a 管理员；id 案件；in 案件版本及承运信息；key 幂等键；rid 请求标识。
// 返回值：补发运单 JSON 或库存、状态、权限错误。
func (s *Service) ReplacementShipment(ctx context.Context, a identity.Actor, id string, in order.ShipInput, key, rid string) (json.RawMessage, error) {
	command := mutation{Operation: "replacementShipment", Target: id, Permission: "order.ship", Entity: "shipment", Key: key, RequestID: rid, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		out, err := s.Orders.ShipReplacementTx(ctx, tx, id, in)
		if err != nil {
			return nil, "", err
		}
		if err = s.orderEvent(ctx, tx, "replacement.shipped", out.OrderID, id, 1); err != nil {
			return nil, "", err
		}
		return out, out.ID, nil
	})
}

// orderEvent 用途：在业务事务内发布仅含引用的可靠运营事件。
// 参数：ctx 上下文；tx 事务；kind 模板；orderID 原订单；entity 对象；version 版本。
// 返回值：nil 或 SQL 错误，不保存联系人与正文。
func (s *Service) orderEvent(ctx context.Context, tx *sql.Tx, kind, orderID, entity string, version int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,?,JSON_OBJECT('order_id',?,'entity_id',?,'version',?),?)`, fmt.Sprintf("%s:%s:%d", kind, entity, version), kind, orderID, entity, version, s.Now().UTC())
	return err
}
