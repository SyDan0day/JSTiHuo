package identity

// Repository methods own all SQL; the application service owns transaction boundaries.
import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type admin struct {
	ID, AuthVersion, Version int64
	Password                 string
	Enabled, Force           bool
}

func scanAdmin(row *sql.Row) (admin, error) {
	var u admin
	e := row.Scan(&u.ID, &u.Password, &u.Enabled, &u.Force, &u.AuthVersion, &u.Version)
	return u, e
}

const adminColumns = "id,password_hash,enabled,force_password_change,auth_version,version"

func findAdmin(ctx context.Context, q queryer, name string) (admin, error) {
	return scanAdmin(q.QueryRowContext(ctx, "SELECT "+adminColumns+" FROM admin_users WHERE login_name=?", name))
}
func findAdminID(ctx context.Context, q queryer, n int64) (admin, error) {
	return scanAdmin(q.QueryRowContext(ctx, "SELECT "+adminColumns+" FROM admin_users WHERE id=?", n))
}
func lockAdmin(ctx context.Context, tx *sql.Tx, n int64) (admin, error) {
	return scanAdmin(tx.QueryRowContext(ctx, "SELECT "+adminColumns+" FROM admin_users WHERE id=? FOR UPDATE", n))
}
func loadPermissions(ctx context.Context, q queryer, n int64) ([]string, error) {
	rows, e := q.QueryContext(ctx, "SELECT DISTINCT p.permission_key FROM permissions p JOIN role_permissions rp ON rp.permission_id=p.id JOIN roles r ON r.id=rp.role_id AND r.enabled=1 JOIN admin_user_roles ur ON ur.role_id=r.id WHERE ur.admin_id=? ORDER BY p.permission_key", n)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	p := []string{}
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		p = append(p, v)
	}
	return p, rows.Err()
}

// loadActor 用途：按令牌摘要读取主体和作用域。
// 参数：ctx 请求上下文；db 业务数据库。
// 参数：hash 会话令牌摘要；kind 会话或资源类型。
// 返回值：主体或 SQL 错误。
// 约束：不读取令牌明文，不自行裁决当前业务可执行性。
func loadActor(ctx context.Context, db *sql.DB, hash []byte, kind string) (Actor, error) {
	var a Actor
	var raw []byte
	e := db.QueryRowContext(ctx, "SELECT id,COALESCE(admin_id,0),subject_version,kind,token_hash,expires_at,last_seen_at,csrf_hash,reauth_at,COALESCE(voucher_id,0),COALESCE(order_id,0),scope FROM sessions WHERE token_hash=? AND kind=?", hash, kind).Scan(&a.SessionID, &a.AdminID, &a.AuthVersion, &a.Kind, &raw, &a.Expires, &a.Seen, &a.CSRFHash, &a.Reauth, &a.VoucherID, &a.OrderID, &a.Scope)
	a.Hash = base64.RawStdEncoding.EncodeToString(raw)
	return a, e
}
func checkAdmin(ctx context.Context, db *sql.DB, a *Actor) error {
	u, e := findAdminID(ctx, db, a.AdminID)
	if e != nil || !u.Enabled || u.AuthVersion != a.AuthVersion {
		return fmt.Errorf("revoked")
	}
	a.Force = u.Force
	a.Permissions, e = loadPermissions(ctx, db, u.ID)
	if e != nil {
		return e
	}
	_, e = db.ExecContext(ctx, "UPDATE sessions SET last_seen_at=UTC_TIMESTAMP(6) WHERE id=?", a.SessionID)
	return e
}
func insertSession(ctx context.Context, tx *sql.Tx, t, c, kind string, n, v int64, exp, now time.Time, reauth bool) error {
	var adminID any
	if n > 0 {
		adminID = n
	}
	var at any
	if reauth {
		at = now
	}
	scope := "AUTH_ONLY"
	if kind == "ADMIN" {
		scope = "WRITE"
	}
	_, e := tx.ExecContext(ctx, "INSERT INTO sessions(token_hash,kind,scope,admin_id,subject_version,csrf_hash,expires_at,last_seen_at,reauth_at) VALUES(?,?,?,?,?,?,?,?,?)", digest(t), kind, scope, adminID, v, digest(c), exp, now, at)
	return e
}
func deleteSession(ctx context.Context, tx *sql.Tx, n int64) error {
	if _, e := tx.ExecContext(ctx, "DELETE FROM challenges WHERE session_id=?", n); e != nil {
		return e
	}
	r, e := tx.ExecContext(ctx, "DELETE FROM sessions WHERE id=?", n)
	if e != nil {
		return e
	}
	count, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if count != 1 {
		return fail(401, "AUTH_REQUIRED")
	}
	return nil
}
func refreshCSRF(ctx context.Context, db *sql.DB, n int64, hash []byte, now time.Time) error {
	_, e := db.ExecContext(ctx, "UPDATE sessions SET csrf_hash=?,last_seen_at=? WHERE id=?", hash, now, n)
	return e
}
func countLimit(ctx context.Context, db *sql.DB, key []byte, end time.Time) (int, error) {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, "INSERT INTO rate_limits(bucket_hash,window_end,attempts) VALUES(?,?,1) ON DUPLICATE KEY UPDATE attempts=attempts+1", key, end)
	if e != nil {
		return 0, e
	}
	var n int
	e = tx.QueryRowContext(ctx, "SELECT attempts FROM rate_limits WHERE bucket_hash=?", key).Scan(&n)
	if e != nil {
		return 0, e
	}
	return n, tx.Commit()
}
func checkLimit(ctx context.Context, db *sql.DB, key []byte, max int) error {
	var n int
	e := db.QueryRowContext(ctx, "SELECT attempts FROM rate_limits WHERE bucket_hash=?", key).Scan(&n)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if n >= max {
		return fail(429, "RATE_LIMITED")
	}
	return nil
}
func insertChallenge(ctx context.Context, db *sql.DB, n int64, p string, hash []byte, exp time.Time) (int64, error) {
	r, e := db.ExecContext(ctx, "INSERT INTO challenges(session_id,purpose,answer_hash,expires_at) VALUES(?,?,?,?)", n, p, hash, exp)
	if e != nil {
		return 0, e
	}
	return r.LastInsertId()
}

type challengeRow struct {
	Purpose  string
	Answer   []byte
	Attempts int
	Expires  time.Time
	Consumed sql.NullTime
}

func lockChallenge(ctx context.Context, tx *sql.Tx, n, sessionID int64) (challengeRow, error) {
	var c challengeRow
	e := tx.QueryRowContext(ctx, "SELECT purpose,answer_hash,attempts,expires_at,consumed_at FROM challenges WHERE id=? AND session_id=? FOR UPDATE", n, sessionID).Scan(&c.Purpose, &c.Answer, &c.Attempts, &c.Expires, &c.Consumed)
	return c, e
}
func updateChallenge(ctx context.Context, tx *sql.Tx, n int64, valid bool, now time.Time) error {
	var at any
	if valid {
		at = now
	}
	_, e := tx.ExecContext(ctx, "UPDATE challenges SET attempts=attempts+1,solved_at=?,consumed_at=? WHERE id=?", at, at, n)
	return e
}
func audit(ctx context.Context, tx *sql.Tx, n int64, requestID, action string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN',?,'admin',?,?,JSON_OBJECT(),'SUCCESS')", n, action, n, requestID)
	return e
}
func auditFailure(ctx context.Context, db *sql.DB, requestID, action string) error {
	_, e := db.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,request_id,diff,result) VALUES('ANONYMOUS',?,'admin',?,JSON_OBJECT(),'DENIED')", action, requestID)
	return e
}
func changePassword(ctx context.Context, tx *sql.Tx, n int64, hash string) error {
	_, e := tx.ExecContext(ctx, "UPDATE admin_users SET password_hash=?,force_password_change=0,auth_version=auth_version+1,version=version+1 WHERE id=?", hash, n)
	return e
}
func revokeAdmin(ctx context.Context, tx *sql.Tx, n int64) error {
	_, e := tx.ExecContext(ctx, "DELETE FROM sessions WHERE admin_id=?", n)
	return e
}
func recordPasswordKey(ctx context.Context, tx *sql.Tx, a Actor, key string, hash []byte, version int64, now time.Time) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,response_status,result_ref,expires_at) VALUES(?,?,?,200,JSON_OBJECT('entity_type','admin','id',?,'version',?,'http_status',200),?)", "password:"+a.Hash, key, hash, fmt.Sprint(a.AdminID), version, now.Add(24*time.Hour))
	return e
}
