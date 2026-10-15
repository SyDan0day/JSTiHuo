package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

type FeedbackInput struct {
	Name        string `json:"contact_name"`
	Phone       string `json:"phone"`
	Email       string `json:"email,omitempty"`
	OrderNo     string `json:"order_no,omitempty"`
	Message     string `json:"message"`
	ChallengeID string `json:"challenge_id"`
	Answer      string `json:"challenge_answer"`
}

type Feedback struct {
	ID         string `json:"id"`
	Version    int64  `json:"version"`
	Message    string `json:"message"`
	State      string `json:"state"`
	Resolution string `json:"resolution"`
}

type FeedbackResolve struct {
	Version    int64  `json:"version"`
	Resolution string `json:"resolution"`
}

// validateFeedback 用途：规范化并校验公开反馈的联系人与正文。
// 参数：in 原始反馈；test 是否允许专用测试手机号。
// 返回值：规范化输入或字段错误，不修改挑战或业务记录。
func validateFeedback(in FeedbackInput, test bool) (FeedbackInput, error) {
	in.Name, in.Phone = strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone)
	in.Email, in.OrderNo = strings.TrimSpace(in.Email), strings.ToUpper(strings.TrimSpace(in.OrderNo))
	in.Message = strings.TrimSpace(in.Message)
	phoneOK := regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(in.Phone) || (test && in.Phone == "10000000000")
	if !phoneOK || !validText(in.Name, 1, 40) || !validText(in.Message, 1, 200) ||
		!validText(in.Email, 0, 100) || !validText(in.OrderNo, 0, 40) || !rules.ValidID(in.ChallengeID) || !validText(in.Answer, 4, 8) {
		return in, fault(422, "INVALID_ARGUMENT")
	}
	if in.Email != "" {
		address, err := mail.ParseAddress(in.Email)
		if err != nil || address.Address != in.Email {
			return in, fault(422, "INVALID_ARGUMENT")
		}
	}
	return in, nil
}

// CreateFeedback 用途：验证一次性挑战后加密保存公开反馈。
// 参数：ctx 上下文；a 预认证主体；order 可选已认证订单主体；in 反馈；ip 请求源；rid 请求标识。
// 返回值：不含联系人或订单授权的反馈回执；挑战、限流、字段或数据库错误。
// 约束：一次挑战只消费一次；联系人与参考订单号均只存密文，不自动发送消息。
func (s *Service) CreateFeedback(ctx context.Context, a identity.Actor, order *identity.Actor, in FeedbackInput, ip, rid string) (Feedback, error) {
	if a.Kind != "PREAUTH" {
		return Feedback{}, fault(403, "FORBIDDEN")
	}
	clean, err := validateFeedback(in, s.Test)
	if err != nil {
		return Feedback{}, err
	}
	if err = s.Auth.Limit(ctx, "feedback:ip:"+ip, 10, time.Hour); err != nil {
		return Feedback{}, err
	}
	if err = s.Auth.ConsumeChallenge(ctx, a, clean.ChallengeID, clean.Answer, "FEEDBACK"); err != nil {
		return Feedback{}, err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Feedback{}, err
	}
	defer tx.Rollback()
	var orderID any
	if order != nil && order.Kind == "ORDER" && clean.OrderNo != "" {
		var id string
		err = tx.QueryRowContext(ctx, "SELECT id FROM orders WHERE id=? AND order_no=?", order.OrderID, clean.OrderNo).Scan(&id)
		if err != nil && err != sql.ErrNoRows {
			return Feedback{}, err
		}
		if err == nil {
			orderID = id
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO feedback(order_id,contact_cipher,pii_key_version,message,resolution)
		VALUES(?,?,?,?, '')`, orderID, []byte{}, s.PII.Active, clean.Message)
	if err != nil {
		return Feedback{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Feedback{}, err
	}
	if err = s.storeContact(ctx, tx, id, clean); err != nil {
		return Feedback{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result)
		VALUES('CUSTOMER','createFeedback','feedback',?,?,JSON_OBJECT(),'SUCCESS')`, id, rid)
	if err != nil {
		return Feedback{}, err
	}
	return Feedback{fmt.Sprint(id), 1, clean.Message, "OPEN", ""}, tx.Commit()
}

// storeContact 用途：以反馈 ID 绑定密文并保存联系人。
// 参数：ctx 上下文；tx 当前事务；id 新反馈 ID；in 已校验联系人。
// 返回值：加密或数据库错误；不记录明文。
func (s *Service) storeContact(ctx context.Context, tx *sql.Tx, id int64, in FeedbackInput) error {
	plain, err := json.Marshal(struct{ Name, Phone, Email, OrderNo string }{in.Name, in.Phone, in.Email, in.OrderNo})
	if err != nil {
		return err
	}
	defer clear(plain)
	cipher, version, err := s.PII.Encrypt(plain, []byte(fmt.Sprintf("feedback:%d", id)))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE feedback SET contact_cipher=?,pii_key_version=? WHERE id=?", cipher, version, id)
	return err
}

// FeedbackList 用途：读取不含联系人的反馈分页。
// 参数：ctx 上下文；f 已校验分页、正文搜索及处理状态。
// 返回值：固定降序分页或数据库错误。
func (s *Service) FeedbackList(ctx context.Context, f Filter) (Page[Feedback], error) {
	if f.Limit < 1 || f.Limit > 100 || (f.After != "" && !rules.ValidID(f.After)) ||
		!validText(f.Q, 0, 100) || (f.Status != "" && f.Status != "OPEN" && f.Status != "RESOLVED") {
		return Page[Feedback]{}, fault(400, "INVALID_ARGUMENT")
	}
	query, args := "SELECT id,version,message,state,resolution FROM feedback WHERE 1=1", []any{}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND state=?"
		args = append(args, f.Status)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,message)>0"
		args = append(args, f.Q)
	}
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", append(args, f.Limit+1)...)
	out := Page[Feedback]{Items: []Feedback{}}
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Feedback
		if err = rows.Scan(&item.ID, &item.Version, &item.Message, &item.State, &item.Resolution); err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	return out, rows.Err()
}

// ResolveFeedback 用途：将开放反馈按版本标记为已处理。
// 参数：ctx 上下文；a 管理员；id 反馈 ID；in 版本与说明；key 幂等键；rid 请求标识。
// 返回值：脱敏反馈 JSON，重复同键重放；版本或状态冲突返回 409。
// 事务：处理状态、幂等与审计一起提交，不发送通知。
func (s *Service) ResolveFeedback(ctx context.Context, a identity.Actor, id string, in FeedbackResolve, key, rid string) (json.RawMessage, error) {
	in.Resolution = strings.TrimSpace(in.Resolution)
	if !rules.ValidID(id) || in.Version < 1 || in.Version >= 9007199254740991 || !validText(in.Resolution, 1, 500) {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	command := mutation{Operation: "resolveFeedback", Permission: "feedback.write", Entity: "feedback", Key: key, RequestID: rid, Body: struct {
		ID    string
		Input FeedbackResolve
	}{id, in}}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		var out Feedback
		err := tx.QueryRowContext(ctx, "SELECT id,version,message,state,resolution FROM feedback WHERE id=? FOR UPDATE", id).
			Scan(&out.ID, &out.Version, &out.Message, &out.State, &out.Resolution)
		if err == sql.ErrNoRows {
			return nil, "", fault(404, "NOT_FOUND")
		}
		if err != nil {
			return nil, "", err
		}
		if out.Version != in.Version || out.State != "OPEN" {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		_, err = tx.ExecContext(ctx, "UPDATE feedback SET state='RESOLVED',resolution=?,version=version+1 WHERE id=?", in.Resolution, id)
		out.State, out.Resolution, out.Version = "RESOLVED", in.Resolution, out.Version+1
		return out, id, err
	})
}
