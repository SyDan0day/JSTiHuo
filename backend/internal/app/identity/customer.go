package identity

import (
	"context"
	"database/sql"
	"time"
)

// PrepareCustomer 用途：准备 opaque 顾客会话令牌和 CSRF。
// 参数：kind 会话或资源类型。
// 返回值：内存会话材料或随机源错误。
// 约束：不写数据库、不记录秘密。
func (s *Service) PrepareCustomer(kind string) (LoginResult, error) {
	t, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	csrf, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	ttl := 30 * time.Minute
	if kind == "VOUCHER" {
		ttl = 15 * time.Minute
	}
	return LoginResult{Session: Session{CSRF: csrf, Expires: s.Now().Add(ttl), Permissions: []string{}}, Token: t}, nil
}

// InsertCustomer 用途：在调用方事务中插入顾客会话并可撤销旧会话。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：result 已准备的会话材料；previous 可空的待撤销会话。
// 参数：kind 会话或资源类型；scope 会话权限范围。
// 参数：voucherID 卡券标识；orderID 已授权订单标识。
// 参数：version 期望版本。
// 返回值：nil 或类型、作用域、删除、SQL 错误。
// 约束：旧挑战先于会话删除，不自行提交事务。
func (s *Service) InsertCustomer(ctx context.Context, tx *sql.Tx, result LoginResult, previous *Actor, kind, scope string, voucherID, orderID, version int64) error {
	if kind != "VOUCHER" && kind != "ORDER" {
		return fail(422, "INVALID_ARGUMENT")
	}
	if scope != "READ" && scope != "WRITE" {
		return fail(422, "INVALID_ARGUMENT")
	}
	if previous != nil {
		if e := deleteSession(ctx, tx, previous.SessionID); e != nil {
			return e
		}
	}
	var voucherRef, orderRef any
	if kind == "VOUCHER" {
		voucherRef = voucherID
	} else {
		orderRef = orderID
	}
	_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,kind,scope,voucher_id,order_id,subject_version,csrf_hash,expires_at,last_seen_at) VALUES(?,?,?,?,?,?,?,?,?)", digest(result.Token), kind, scope, voucherRef, orderRef, version, digest(result.Session.CSRF), result.Session.Expires, s.Now())
	return e
}

// ConsumeChallenge 用途：消费匹配用途的一次性挑战。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；answer 用户填写的挑战答案。
// 参数：purpose 规定挑战用途。
// 返回值：nil 或挑战过期、用途不符、已消费、尝试耗尽、数据库错误。
// 约束：成功答案即持久标记消费，后续业务失败也不重复使用。
func (s *Service) ConsumeChallenge(ctx context.Context, a Actor, id, answer, purpose string) error {
	return s.consumePurpose(ctx, a, id, answer, purpose)
}

// Limit 用途：调用共享数据库限流。
// 参数：ctx 请求上下文；label 内部限流桶标识。
// 参数：max 限流窗口额度；window 限流窗口长度。
// 返回值：nil 或限流、SQL 错误。
// 约束：标识由独立 HMAC 保护，每次调用递增计数。
func (s *Service) Limit(ctx context.Context, label string, max int, window time.Duration) error {
	return s.limit(ctx, label, max, window)
}
