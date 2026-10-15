package operations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

type ResolveNotification struct {
	Version    int64  `json:"version"`
	Resolution string `json:"resolution"`
	Evidence   string `json:"evidence_reference"`
	Reason     string `json:"reason"`
}

// Notifications 用途：分页读取不含联系人和消息正文的通知状态。
// 参数：ctx 上下文；f 固定状态与模板搜索。
// 返回值：通知页或数据库错误。
func (s *Service) Notifications(ctx context.Context, f Filter) (Page[Notification], error) {
	page := Page[Notification]{Items: []Notification{}}
	query := "SELECT id FROM notifications WHERE 1=1"
	args := []any{}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND state=?"
		args = append(args, f.Status)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,template_key)>0"
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
		out, err := loadNotification(ctx, s.DB, id, false)
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

// changeNotification 用途：在事件先于通知的固定锁顺序下执行人工处置。
// 参数：ctx 上下文；a 管理员；id 通知；command 授权幂等命令；fn 处置回调。
// 返回值：通知 JSON 或权限、版本、SQL 错误。
func (s *Service) changeNotification(ctx context.Context, a identity.Actor, id string, command mutation, fn func(*sql.Tx, Notification, string, string) error) (json.RawMessage, error) {
	outboxID, err := notificationOutbox(ctx, s.DB, id)
	if err != nil {
		return nil, err
	}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT state FROM outbox_events WHERE id=? FOR UPDATE", outboxID).Scan(&state); err != nil {
			return nil, "", err
		}
		out, err := loadNotification(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if err = fn(tx, out, outboxID, state); err != nil {
			return nil, "", err
		}
		out, err = loadNotification(ctx, tx, id, false)
		return out, id, err
	})
}

// RetryNotification 用途：显式重试已确认失败的通知。
// 参数：ctx 上下文；a 管理员；id 通知；in 版本和原因；key 幂等键；rid 请求标识。
// 返回值：PENDING 通知或版本、状态、权限错误；UNKNOWN 禁止直接重发。
func (s *Service) RetryNotification(ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string) (json.RawMessage, error) {
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !validText(strings.TrimSpace(in.Reason), 1, 200) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	command := mutation{Operation: "retryNotification", Target: id, Permission: "notification.retry", Entity: "notification", Key: key, RequestID: rid, Body: in, Status: 202}
	return s.changeNotification(ctx, a, id, command, func(tx *sql.Tx, out Notification, eventID, state string) error {
		if out.Version != in.Version {
			return fault(409, "VERSION_CONFLICT")
		}
		if out.State != "FAILED" || (state != "DEAD" && state != "PENDING") {
			return fault(409, "ILLEGAL_TRANSITION")
		}
		_, err := tx.ExecContext(ctx, "UPDATE outbox_events SET state='PENDING',attempts=0,next_attempt_at=?,lease_until=NULL,last_error_code=NULL,version=version+1 WHERE id=?", s.Now().UTC(), eventID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE notifications SET state='PENDING',last_error_code=NULL,version=version+1 WHERE id=?", id)
		return err
	})
}

// ResolveNotice 用途：经近期复核核对未知通知，确认未发后才允许重试。
// 参数：ctx 上下文；a 管理员；id 通知；in 核对版本与证据；key 幂等键；rid 请求标识。
// 返回值：已核对通知或状态、复核、权限错误。
func (s *Service) ResolveNotice(ctx context.Context, a identity.Actor, id string, in ResolveNotification, key, rid string) (json.RawMessage, error) {
	in.Evidence = strings.TrimSpace(in.Evidence)
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !validText(in.Evidence, 1, 200) || !validText(in.Reason, 0, 200) || (in.Resolution != "CONFIRMED_SENT" && in.Resolution != "CONFIRMED_NOT_SENT") {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	digest := sha256.Sum256([]byte(in.Evidence))
	command := mutation{Operation: "resolveNotification", Target: id, Permission: "notification.manage", Entity: "notification", Key: key, RequestID: rid, Body: in, Recent: true, Diff: map[string]any{"resolution": in.Resolution, "evidence_sha256": hex.EncodeToString(digest[:])}}
	return s.changeNotification(ctx, a, id, command, func(tx *sql.Tx, out Notification, eventID, state string) error {
		if out.Version != in.Version {
			return fault(409, "VERSION_CONFLICT")
		}
		if out.State != "UNKNOWN" || state != "DEAD" {
			return fault(409, "ILLEGAL_TRANSITION")
		}
		next, event := "FAILED", "DEAD"
		if in.Resolution == "CONFIRMED_SENT" {
			next, event = "SENT", "DONE"
		}
		_, err := tx.ExecContext(ctx, "UPDATE outbox_events SET state=?,last_error_code=NULL,lease_until=NULL,version=version+1 WHERE id=?", event, eventID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE notifications SET state=?,last_error_code=NULL,version=version+1 WHERE id=?", next, id)
		return err
	})
}
