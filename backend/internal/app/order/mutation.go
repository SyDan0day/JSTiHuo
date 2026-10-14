package order

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"regexp"
	"time"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// mutate 用途：执行带权限复查、审计和重放引用的订单写用例。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：op 内部操作标识；permission 所需管理员权限。
// 参数：id 目标对象标识；body 已规范化业务命令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 参数：fn 当前事务业务回调。
// 返回值：nil 或权限、摘要冲突、SQL、回调错误。
// 事务：权限、审计和成功引用与业务同事务；失败整体回滚。
func (s *Service) mutate(ctx context.Context, a identity.Actor, op, permission, id string, body any, key, rid string, fn func(*sql.Tx) error) error {
	if !keyPattern.MatchString(key) {
		return fault(400, "INVALID_ARGUMENT")
	}
	raw, e := json.Marshal(struct {
		ID    string `json:"id"`
		Input any    `json:"input"`
	}{id, body})
	if e != nil {
		return e
	}
	mac := hmac.New(sha256.New, s.Redemption.RequestKey)
	mac.Write(raw)
	hash := mac.Sum(nil)
	scope := fmt.Sprintf("admin:%d:%s", a.AdminID, op)
	if a.Kind == "ORDER" {
		scope = fmt.Sprintf("order:%d:%s", a.OrderID, op)
	}
	for attempt := 0; attempt < 3; attempt++ {
		e = s.mutateTx(ctx, a, op, permission, id, body, key, rid, scope, hash, fn)
		if e == nil {
			return nil
		}
		var mysqlError *mysql.MySQLError
		if !errors.As(e, &mysqlError) || (mysqlError.Number != 1205 && mysqlError.Number != 1213) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return e
}

// mutateTx 用途：执行一轮订单写事务。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：op 内部操作标识；permission 所需管理员权限。
// 参数：id 目标对象标识；body 已规范化业务命令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 参数：scope 主体和操作的幂等命名空间；hash 请求 HMAC 摘要。
// 参数：fn 当前事务业务回调。
// 返回值：nil 或权限、重放冲突、业务、SQL 错误。
// 事务：授权、业务及幂等引用一起提交；审计仅含白名单字段。
func (s *Service) mutateTx(ctx context.Context, a identity.Actor, op, permission, id string, body any, key, rid, scope string, hash []byte, fn func(*sql.Tx) error) error {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	now := s.Redemption.Now().UTC()
	if _, e = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour)); e != nil {
		return e
	}
	var stored, ref []byte
	var expiry time.Time
	if e = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &ref, &expiry); e != nil {
		return e
	}
	if e = s.authorize(ctx, tx, a, permission, id); e != nil {
		return e
	}
	if expiry.After(now) {
		if !hmac.Equal(stored, hash) {
			return fault(409, "IDEMPOTENCY_CONFLICT")
		}
		var previous struct {
			ID string `json:"id"`
		}
		if e = json.Unmarshal(ref, &previous); e != nil {
			return e
		}
		if previous.ID != "" {
			if op == "privateOrder" {
				if e = fn(tx); e != nil {
					return e
				}
				if e = s.appendAudit(ctx, tx, a, op, id, rid, body); e != nil {
					return e
				}
			}
			return tx.Commit()
		}
	}
	if e = fn(tx); e != nil {
		return e
	}
	if e = s.appendAudit(ctx, tx, a, op, id, rid, body); e != nil {
		return e
	}
	encoded, _ := json.Marshal(map[string]string{"id": id})
	if _, e = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=200,expires_at=? WHERE scope=? AND request_key=?", hash, encoded, now.Add(24*time.Hour), scope, key); e != nil {
		return e
	}
	if e = s.inject(ctx, tx, "commit"); e != nil {
		return e
	}
	return tx.Commit()
}

// appendAudit 用途：在订单事务中记录白名单动作证据。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：a 已认证操作主体；op 内部操作标识。
// 参数：id 目标对象标识；rid 审计请求标识。
// 参数：body 已规范化业务命令。
// 返回值：nil 或测试故障、SQL 错误。
// 约束：只记录版本、日期、承运商和原因，绝不记录地址、手机号、卡密。
func (s *Service) appendAudit(ctx context.Context, tx *sql.Tx, a identity.Actor, op, id, rid string, body any) error {
	diff := map[string]any{}
	switch in := body.(type) {
	case VersionReason:
		diff["reason"] = in.Reason
		diff["version"] = in.Version
	case EditInput:
		diff["version"] = in.Version
		diff["ship_date"] = in.Date
	case ShipInput:
		diff["version"] = in.Version
		diff["carrier_id"] = in.CarrierID
	}
	raw, e := json.Marshal(diff)
	if e != nil {
		return e
	}
	kind := "CUSTOMER"
	var admin any
	if a.Kind == "ADMIN" {
		kind = "ADMIN"
		admin = a.AdminID
	}
	if e = s.inject(ctx, tx, "audit"); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,?,?,'order',?,?,?,'SUCCESS')", admin, kind, op, id, rid, raw)
	return e
}

// inject 用途：执行仅测试装配的事务边界故障。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：step 测试故障边界名称。
// 返回值：钩子错误或 nil。
// 约束：生产不设置钩子，不修改业务规则。
func (s *Service) inject(ctx context.Context, tx *sql.Tx, step string) error {
	if s.FaultTx != nil {
		return s.FaultTx(ctx, tx, step)
	}
	return nil
}

// lock 用途：锁定订单主记录并读取关联日期。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识。
// 返回值：锁定快照或不存在、SQL 错误。
// 约束：只锁订单行，日历由用例按日期升序另外锁定。
func (s *Service) lock(ctx context.Context, tx *sql.Tx, id string) (Locked, error) {
	var o Locked
	var calendarID sql.NullString
	e := tx.QueryRowContext(ctx, "SELECT id,version,order_no,status,fulfillment_mode,calendar_id,recipient_cipher,pii_key_version,deleted_at IS NOT NULL FROM orders WHERE id=? FOR UPDATE", id).Scan(&o.ID, &o.Version, &o.Number, &o.Status, &o.Mode, &calendarID, &o.Cipher, &o.KeyVersion, &o.Deleted)
	if e == sql.ErrNoRows {
		e = fault(404, "NOT_FOUND")
	}
	if e != nil {
		return o, e
	}
	o.CalendarID = calendarID.String
	if o.CalendarID != "" {
		e = tx.QueryRowContext(ctx, "SELECT ship_date FROM shipping_calendars WHERE id=?", o.CalendarID).Scan(&o.Date)
	}
	return o, e
}

// event 用途：同事务写入仅对象标识的订单事件。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：name 固定事件类型；id 目标对象标识。
// 参数：version 期望版本。
// 返回值：nil 或 SQL 错误。
// 约束：唯一事件键防重，不调用外部通知。
func (s *Service) event(ctx context.Context, tx *sql.Tx, name, id string, version int64) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,?,JSON_OBJECT('order_id',?),?)", fmt.Sprintf("%s:%s:%d", name, id, version), name, id, s.Now().UTC())
	return e
}
