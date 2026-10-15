package order

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	rules "jstihuo/internal/domain/voucher"
	crypt "jstihuo/internal/infra/crypto"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Query 用途：按标识、完整手机和挑战建立 ORDER READ 会话。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 标识、手机号和挑战命令；ip 受控请求来源地址。
// 参数：rid 审计请求标识。
// 返回值：内部会话或字段、挑战、限流、统一凭证、数据库错误。
// 事务：手机号用独立 HMAC 匹配；锁内复查后签发 READ 并撤销 PREAUTH。
func (s *Service) Query(ctx context.Context, a identity.Actor, in QueryCredentials, ip, rid string) (identity.LoginResult, error) {
	in.Identifier = strings.ToUpper(strings.TrimSpace(in.Identifier))
	in.Phone = strings.TrimSpace(in.Phone)
	if a.Kind != "PREAUTH" || len(in.Identifier) < 1 || len(in.Identifier) > 40 || !regexp.MustCompile(`^[0-9]{11}$`).MatchString(in.Phone) || !rules.ValidID(in.ChallengeID) || len(in.Answer) < 4 || len(in.Answer) > 8 {
		return identity.LoginResult{}, fault(422, "VALIDATION_FAILED")
	}
	if e := s.Auth.Limit(ctx, "query-ip:"+ip, 60, 15*time.Minute); e != nil {
		return identity.LoginResult{}, e
	}
	if e := s.Auth.Limit(ctx, "query-identifier:"+in.Identifier, 10, 15*time.Minute); e != nil {
		return identity.LoginResult{}, e
	}
	if e := s.Auth.ConsumeChallenge(ctx, a, in.ChallengeID, in.Answer, "ORDER_QUERY"); e != nil {
		return identity.LoginResult{}, e
	}
	phoneMAC := hmac.New(sha256.New, s.Redemption.LookupKey)
	phoneMAC.Write([]byte(in.Phone))
	lookup := phoneMAC.Sum(nil)
	where := "v.code=?"
	if strings.HasPrefix(in.Identifier, "O_") {
		where = "o.order_no=?"
	}
	var oid, vid, bid, cred int64
	var stored []byte
	e := s.DB.QueryRowContext(ctx, "SELECT o.id,v.id,v.batch_id,v.credential_version,o.phone_lookup FROM orders o JOIN voucher_redemptions r ON r.id=o.redemption_id JOIN vouchers v ON v.id=r.voucher_id WHERE "+where, in.Identifier).Scan(&oid, &vid, &bid, &cred, &stored)
	if e != nil && e != sql.ErrNoRows {
		return identity.LoginResult{}, e
	}
	if e != nil || !hmac.Equal(stored, lookup) {
		return identity.LoginResult{}, fault(401, "INVALID_CREDENTIALS")
	}
	session, e := s.Auth.PrepareCustomer("ORDER")
	if e != nil {
		return session, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return session, e
	}
	defer tx.Rollback()
	var version int64
	if e = tx.QueryRowContext(ctx, "SELECT id FROM voucher_batches WHERE id=? FOR SHARE", bid).Scan(&version); e != nil {
		return session, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT credential_version FROM vouchers WHERE id=? FOR SHARE", vid).Scan(&version); e != nil {
		return session, e
	}
	if version != cred {
		return session, fault(401, "INVALID_CREDENTIALS")
	}
	if e = tx.QueryRowContext(ctx, "SELECT phone_lookup FROM orders WHERE id=? FOR SHARE", oid).Scan(&stored); e != nil {
		return session, e
	}
	if !hmac.Equal(stored, lookup) {
		return session, fault(401, "INVALID_CREDENTIALS")
	}
	var expires time.Time
	if e = tx.QueryRowContext(ctx, "SELECT expires_at FROM sessions WHERE id=? AND kind='PREAUTH' FOR UPDATE", a.SessionID).Scan(&expires); e != nil || !s.Redemption.Now().Before(expires) {
		return session, fault(401, "AUTH_REQUIRED")
	}
	if e = s.Auth.InsertCustomer(ctx, tx, session, &a, "ORDER", "READ", vid, oid, cred); e != nil {
		return session, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('CUSTOMER','queryOrder','order',?,?,JSON_OBJECT('scope','READ'),'SUCCESS')", oid, rid); e != nil {
		return session, e
	}
	return session, tx.Commit()
}

// StepUp 用途：校验原卡密后轮换 ORDER READ 为 WRITE。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：pre 新 PREAUTH 主体；in 原卡号、原卡密和挑战命令。
// 参数：ip 受控请求来源地址；rid 审计请求标识。
// 返回值：WRITE 会话或挑战、凭证、归属、SQL 错误。
// 事务：按批次、卡券、订单顺序复查原凭证，再撤销旧两种会话并签发 WRITE。
func (s *Service) StepUp(ctx context.Context, a, pre identity.Actor, in redemption.Credentials, ip, rid string) (identity.LoginResult, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if a.Kind != "ORDER" || pre.Kind != "PREAUTH" || !regexp.MustCompile(`^[A-Z0-9]{6,32}$`).MatchString(in.Code) || len(in.Secret) < 6 || len(in.Secret) > 32 || !rules.ValidID(in.ChallengeID) || len(in.Answer) < 4 || len(in.Answer) > 8 {
		return identity.LoginResult{}, fault(422, "VALIDATION_FAILED")
	}
	if e := s.Auth.Limit(ctx, "stepup-ip:"+ip, 60, 15*time.Minute); e != nil {
		return identity.LoginResult{}, e
	}
	if e := s.Auth.Limit(ctx, fmt.Sprintf("stepup-order:%d", a.OrderID), 10, 15*time.Minute); e != nil {
		return identity.LoginResult{}, e
	}
	if e := s.Auth.ConsumeChallenge(ctx, pre, in.ChallengeID, in.Answer, "VOUCHER"); e != nil {
		return identity.LoginResult{}, e
	}
	var vid, bid, cred int64
	var hash, code string
	e := s.DB.QueryRowContext(ctx, "SELECT v.id,v.batch_id,v.credential_version,v.secret_hash,v.code FROM orders o JOIN voucher_redemptions r ON r.id=o.redemption_id JOIN vouchers v ON v.id=r.voucher_id WHERE o.id=?", a.OrderID).Scan(&vid, &bid, &cred, &hash, &code)
	checkHash := s.Redemption.Dummy
	if e == nil {
		checkHash = hash
	}
	valid := crypt.Verify(in.Secret, checkHash)
	if e != nil && e != sql.ErrNoRows {
		return identity.LoginResult{}, e
	}
	if !valid || e != nil || code != in.Code || cred != a.AuthVersion {
		return identity.LoginResult{}, fault(401, "INVALID_CREDENTIALS")
	}
	result, e := s.Auth.PrepareCustomer("ORDER")
	if e != nil {
		return result, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return result, e
	}
	defer tx.Rollback()
	var n int64
	if e = tx.QueryRowContext(ctx, "SELECT id FROM voucher_batches WHERE id=? FOR SHARE", bid).Scan(&n); e != nil {
		return result, e
	}
	var currentHash string
	if e = tx.QueryRowContext(ctx, "SELECT credential_version,secret_hash FROM vouchers WHERE id=? FOR SHARE", vid).Scan(&n, &currentHash); e != nil {
		return result, e
	}
	if n != cred || currentHash != hash {
		return result, fault(401, "INVALID_CREDENTIALS")
	}
	if e = tx.QueryRowContext(ctx, "SELECT id FROM orders WHERE id=? FOR SHARE", a.OrderID).Scan(&n); e != nil {
		return result, e
	}
	var expires time.Time
	if e = tx.QueryRowContext(ctx, "SELECT expires_at FROM sessions WHERE id=? AND order_id=? AND kind='ORDER' FOR UPDATE", a.SessionID, a.OrderID).Scan(&expires); e != nil || !s.Redemption.Now().Before(expires) {
		return result, fault(401, "AUTH_REQUIRED")
	}
	if e = tx.QueryRowContext(ctx, "SELECT expires_at FROM sessions WHERE id=? AND kind='PREAUTH' FOR UPDATE", pre.SessionID).Scan(&expires); e != nil || !s.Redemption.Now().Before(expires) {
		return result, fault(401, "AUTH_REQUIRED")
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM challenges WHERE session_id=?", pre.SessionID); e != nil {
		return result, e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sessions WHERE id=?", pre.SessionID); e != nil {
		return result, e
	}
	if e = s.Auth.InsertCustomer(ctx, tx, result, &a, "ORDER", "WRITE", vid, a.OrderID, cred); e != nil {
		return result, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('CUSTOMER','orderStepUp','order',?,?,JSON_OBJECT('scope','WRITE'),'SUCCESS')", a.OrderID, rid); e != nil {
		return result, e
	}
	return result, tx.Commit()
}

// AuthorizeCustomerWrite 用途：在跨模块事务中复用本单 WRITE 与凭证版本复查。
// 参数：ctx 上下文；tx 调用方事务；a ORDER 主体；id 目标订单。
// 返回值：nil 或类型、归属、作用域、会话及凭证失效错误。
func (s *Service) AuthorizeCustomerWrite(ctx context.Context, tx *sql.Tx, a identity.Actor, id string) error {
	if a.Kind != "ORDER" {
		return fault(403, "SCOPE_REQUIRED")
	}
	return s.authorize(ctx, tx, a, "", id)
}

// authorize 用途：事务内重查订单写权限并防止凭证轮换竞争。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：a 已认证操作主体；permission 所需管理员权限。
// 参数：id 目标对象标识。
// 返回值：nil 或权限、会话、凭证版本、SQL 错误。
// 约束：顾客按批次、卡券、会话锁序持共享锁，调用方随后锁订单。
func (s *Service) authorize(ctx context.Context, tx *sql.Tx, a identity.Actor, permission, id string) error {
	if a.Kind == "ADMIN" {
		var enabled, force bool
		var version int64
		if e := tx.QueryRowContext(ctx, "SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR SHARE", a.AdminID).Scan(&enabled, &force, &version); e != nil {
			return e
		}
		if !enabled || version != a.AuthVersion {
			return fault(401, "AUTH_REQUIRED")
		}
		if force {
			return fault(403, "PASSWORD_CHANGE_REQUIRED")
		}
		var count int
		if permission == "" {
			permission = "order.write"
		}
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?", a.AdminID, permission).Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			return fault(403, "FORBIDDEN")
		}
		return nil
	}
	if a.Kind != "ORDER" || permission != "" || a.Scope != "WRITE" || strconv.FormatInt(a.OrderID, 10) != id {
		return fault(403, "SCOPE_REQUIRED")
	}
	var bid, vid, version int64
	if e := tx.QueryRowContext(ctx, "SELECT v.batch_id,v.id FROM orders o JOIN voucher_redemptions r ON r.id=o.redemption_id JOIN vouchers v ON v.id=r.voucher_id WHERE o.id=?", id).Scan(&bid, &vid); e != nil {
		return e
	}
	if e := tx.QueryRowContext(ctx, "SELECT id FROM voucher_batches WHERE id=? FOR SHARE", bid).Scan(&version); e != nil {
		return e
	}
	if e := tx.QueryRowContext(ctx, "SELECT credential_version FROM vouchers WHERE id=? FOR SHARE", vid).Scan(&version); e != nil {
		return e
	}
	if version != a.AuthVersion {
		return fault(401, "AUTH_REQUIRED")
	}
	var expiry time.Time
	var scope string
	if e := tx.QueryRowContext(ctx, "SELECT subject_version,expires_at,scope FROM sessions WHERE id=? AND order_id=? AND kind='ORDER' FOR SHARE", a.SessionID, id).Scan(&version, &expiry, &scope); e != nil {
		return fault(401, "AUTH_REQUIRED")
	}
	if version != a.AuthVersion || scope != "WRITE" || !s.Redemption.Now().Before(expiry) {
		return fault(403, "SCOPE_REQUIRED")
	}
	return nil
}
