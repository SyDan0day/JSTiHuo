package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"jstihuo/internal/infra/integrations"
)

type Notification struct {
	ID        string `json:"id"`
	OrderID   string `json:"order_id,omitempty"`
	Channel   string `json:"channel"`
	State     string `json:"state"`
	Template  string `json:"template_key"`
	Version   int64  `json:"version"`
	ErrorCode string `json:"last_error_code,omitempty"`
}
type OutboxClaim struct {
	ID, Key, Kind, OrderID, NotificationID string
	Version                                int64
	Attempts                               int
	Started                                bool
}

// notificationMode 用途：读取通知开关，缺省关闭且拒绝损坏配置。
// 参数：ctx 上下文；q 数据源。
// 返回值：DISABLED、MOCK、LIVE 或读取错误。
func notificationMode(ctx context.Context, q queryer) (string, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT value FROM system_configs WHERE config_key='shipping_calendar'").Scan(&raw)
	if err == sql.ErrNoRows {
		return "DISABLED", nil
	}
	if err != nil {
		return "", err
	}
	var config struct {
		Mode string `json:"notification_mode"`
	}
	if err = json.Unmarshal(raw, &config); err != nil {
		return "", err
	}
	if config.Mode == "" {
		return "DISABLED", nil
	}
	switch config.Mode {
	case "DISABLED", "MOCK", "LIVE":
		return config.Mode, nil
	}
	return "", fault(503, "INTEGRATION_DISABLED")
}

// loadNotification 用途：读取通知回执的非敏感字段。
// 参数：ctx 上下文；q 数据源；id 通知；lock 是否排他锁定。
// 返回值：通知或不存在、数据库错误。
func loadNotification(ctx context.Context, q queryer, id string, lock bool) (Notification, error) {
	var out Notification
	var orderID, code sql.NullString
	query := "SELECT id,order_id,channel,state,template_key,version,last_error_code FROM notifications WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &orderID, &out.Channel, &out.State, &out.Template, &out.Version, &code)
	if err == sql.ErrNoRows {
		return out, fault(404, "NOT_FOUND")
	}
	out.OrderID, out.ErrorCode = orderID.String, code.String
	return out, err
}

// ClaimOutbox 用途：领取单个到期事件并为通知建立唯一回执。
// 参数：ctx worker 上下文。
// 返回值：带租约版本的凭证、是否领取成功或数据库错误。
// 约束：不处理凭证服务拥有的 file.destroy；事务内不访问外部供应商。
func (s *Service) ClaimOutbox(ctx context.Context) (OutboxClaim, bool, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return OutboxClaim{}, false, err
	}
	defer tx.Rollback()
	var out OutboxClaim
	var raw []byte
	var previous sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,event_key,event_type,payload,version,attempts,last_error_code FROM outbox_events
		WHERE event_type IN ('job.requested','order.visibility','order.created','order.updated','order.cancelled','shipment.created','order.completed','after_sale.created','after_sale.decided','replacement.shipped','tracking.poll')
		AND (state='PENDING' AND next_attempt_at<=? OR state='PROCESSING' AND lease_until<=?)
		ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`, s.Now().UTC(), s.Now().UTC()).Scan(&out.ID, &out.Key, &out.Kind, &raw, &out.Version, &out.Attempts, &previous)
	if err == sql.ErrNoRows {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	out.Started = previous.String == "CALL_STARTED"
	if out.Kind != "job.requested" && out.Kind != "order.visibility" && out.Kind != "tracking.poll" {
		var payload map[string]json.RawMessage
		if err = json.Unmarshal(raw, &payload); err != nil {
			return out, false, err
		}
		var id string
		if json.Unmarshal(payload["order_id"], &id) != nil {
			var number json.Number
			if json.Unmarshal(payload["order_id"], &number) == nil {
				id = string(number)
			}
		}
		out.OrderID = id
		var orderID any
		if id != "" {
			orderID = id
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO notifications(outbox_id,order_id,channel,template_key) VALUES(?,?,'MOCK',?) ON DUPLICATE KEY UPDATE outbox_id=outbox_id`, out.ID, orderID, out.Kind)
		if err != nil {
			return out, false, err
		}
		if err = tx.QueryRowContext(ctx, "SELECT id FROM notifications WHERE outbox_id=? AND channel='MOCK'", out.ID).Scan(&out.NotificationID); err != nil {
			return out, false, err
		}
	}
	out.Version++
	out.Attempts++
	_, err = tx.ExecContext(ctx, "UPDATE outbox_events SET state='PROCESSING',version=?,attempts=?,lease_until=? WHERE id=?", out.Version, out.Attempts, s.Now().UTC().Add(time.Minute), out.ID)
	if err != nil {
		return out, false, err
	}
	return out, true, tx.Commit()
}

// lockOutbox 用途：复查当前工作者仍持有未过期租约。
// 参数：ctx 上下文；tx 当前事务；claim 领取凭证。
// 返回值：nil 或租约、数据库错误。
func (s *Service) lockOutbox(ctx context.Context, tx *sql.Tx, claim OutboxClaim) error {
	var state string
	var version int64
	var lease sql.NullTime
	err := tx.QueryRowContext(ctx, "SELECT state,version,lease_until FROM outbox_events WHERE id=? FOR UPDATE", claim.ID).Scan(&state, &version, &lease)
	if err != nil {
		return err
	}
	if state != "PROCESSING" || version != claim.Version || !lease.Valid || !lease.Time.After(s.Now()) {
		return ErrJobLeaseLost
	}
	return nil
}

// markOutboxCall 用途：外部发送前持久化调用意图，防止崩溃后盲目重发。
// 参数：ctx 上下文；claim 租约凭证。
// 返回值：nil 或租约、数据库错误。
func (s *Service) markOutboxCall(ctx context.Context, claim OutboxClaim) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockOutbox(ctx, tx, claim); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE outbox_events SET last_error_code='CALL_STARTED' WHERE id=?", claim.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ProcessOutbox 用途：在有界事务外发送 MOCK 通知或记录关闭状态。
// 参数：ctx 上下文；claim 当前租约凭证。
// 返回值：结果持久化或基础设施错误；未知发送结果不自动重发。
func (s *Service) ProcessOutbox(ctx context.Context, claim OutboxClaim) error {
	if claim.Kind == "tracking.poll" {
		return s.processTrackingPoll(ctx, claim)
	}
	if claim.NotificationID == "" {
		return s.finishOutbox(ctx, claim, "", "", "")
	}
	mode, err := notificationMode(ctx, s.DB)
	if err != nil {
		return err
	}
	if mode == "DISABLED" || claim.OrderID == "" {
		return s.finishOutbox(ctx, claim, "SUPPRESSED", "", "NOTIFICATION_DISABLED")
	}
	if mode != "MOCK" || s.Notifier == nil {
		return s.finishOutbox(ctx, claim, "FAILED", "", "INTEGRATION_DISABLED")
	}
	if claim.Attempts > 5 {
		return s.finishOutbox(ctx, claim, "FAILED", "", "RETRY_EXHAUSTED")
	}
	if err = s.markOutboxCall(ctx, claim); err != nil {
		return err
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var message string
	var found bool
	if claim.Started {
		message, found, err = s.Notifier.Lookup(work, claim.Key)
		if err != nil || !found && !s.Notifier.Idempotent() {
			return s.finishOutbox(ctx, claim, "UNKNOWN", "", "RESULT_UNKNOWN")
		}
	}
	if !found {
		message, err = s.Notifier.Send(work, claim.Key, integrations.Notice{Template: claim.Kind, OrderID: claim.OrderID, Recipient: "mock:order:" + claim.OrderID})
	}
	state, code := "SENT", ""
	if err != nil {
		state, code = "UNKNOWN", "RESULT_UNKNOWN"
		if errors.Is(err, integrations.ErrRejected) {
			state, code = "FAILED", "PROVIDER_REJECTED"
		} else if errors.Is(err, integrations.ErrTemporary) {
			state, code = "FAILED", "PROVIDER_UNAVAILABLE"
		}
	}
	if err == nil && (message == "" || len(message) > 120) {
		state, code = "UNKNOWN", "INVALID_PROVIDER_RESULT"
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.finishOutbox(ctx, claim, state, message, code)
}

// finishOutbox 用途：用租约版本栅栏原子保存通知与事件结果。
// 参数：ctx 上下文；claim 凭证；state 通知状态或内部事件空值；message 供应商回执；code 固定错误码。
// 返回值：nil 或租约、SQL 错误；确定失败按退避重试，UNKNOWN 进入 DEAD。
func (s *Service) finishOutbox(ctx context.Context, claim OutboxClaim, state, message, code string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockOutbox(ctx, tx, claim); err != nil {
		return err
	}
	if claim.NotificationID != "" {
		if _, err = loadNotification(ctx, tx, claim.NotificationID, true); err != nil {
			return err
		}
		var receipt, errorCode any
		if state == "SENT" {
			receipt = message
		}
		if code != "" {
			errorCode = code
		}
		_, err = tx.ExecContext(ctx, "UPDATE notifications SET state=?,provider_message_id=?,last_error_code=?,version=version+1 WHERE id=?", state, receipt, errorCode, claim.NotificationID)
		if err != nil {
			return err
		}
	}
	outState := "DONE"
	next := s.Now().UTC()
	if state == "UNKNOWN" {
		outState = "DEAD"
	}
	if state == "FAILED" {
		outState = "DEAD"
		if claim.Attempts < 5 && code == "PROVIDER_UNAVAILABLE" {
			outState = "PENDING"
			delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
			next = next.Add(delays[claim.Attempts-1])
		}
	}
	if err = s.inject(ctx, tx, "outbox:finish"); err != nil {
		return err
	}
	var errorCode any
	if code != "" {
		errorCode = code
	}
	_, err = tx.ExecContext(ctx, "UPDATE outbox_events SET state=?,last_error_code=?,next_attempt_at=?,lease_until=NULL,version=version+1 WHERE id=?", outState, errorCode, next, claim.ID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','outbox.result','outbox',?,'outbox-worker',JSON_OBJECT('state',?,'code',?),'SUCCESS')`, claim.ID, outState, code)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RunOutbox 用途：每轮处理至多二十个通知或内部任务事件。
// 参数：ctx 可取消 worker 上下文。
// 返回值：处理数及首个错误；取消后不继续领取。
func (s *Service) RunOutbox(ctx context.Context) (int, error) {
	for n := 0; n < 20; n++ {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		claim, ok, err := s.ClaimOutbox(ctx)
		if err != nil || !ok {
			return n, err
		}
		if err = s.ProcessOutbox(ctx, claim); err != nil {
			return n, err
		}
	}
	return 20, nil
}

// notificationOutbox 用途：读取通知所属事件引用，为事务建立固定锁顺序。
// 参数：ctx 上下文；db 数据库；id 通知 ID。
// 返回值：事件 ID 或不存在、数据库错误。
func notificationOutbox(ctx context.Context, db *sql.DB, id string) (string, error) {
	var out string
	err := db.QueryRowContext(ctx, "SELECT outbox_id FROM notifications WHERE id=?", id).Scan(&out)
	if err == sql.ErrNoRows {
		return "", fault(404, "NOT_FOUND")
	}
	return out, err
}
