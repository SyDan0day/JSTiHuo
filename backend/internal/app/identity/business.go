package identity

import (
	"context"
	"database/sql"
)

// AuthorizeAdmin 用途：在业务事务中重新验证管理员及指定权限。
// 参数：ctx 请求上下文；tx 当前业务事务；actor 已认证管理员；permission 固定权限键。
// 返回值：nil 或身份失效、强制改密、权限不足、数据库错误。
// 并发：共享锁保护管理员版本，权限变更须同步撤销关联主体。
func AuthorizeAdmin(
	ctx context.Context, tx *sql.Tx, actor Actor, permission string,
) error {
	if actor.Kind != "ADMIN" {
		return fail(403, "FORBIDDEN")
	}
	var enabled, force bool
	var version int64
	err := tx.QueryRowContext(ctx,
		"SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR SHARE",
		actor.AdminID,
	).Scan(&enabled, &force, &version)
	if err != nil {
		return err
	}
	if !enabled || version != actor.AuthVersion {
		return fail(401, "AUTH_REQUIRED")
	}
	if force {
		return fail(403, "PASSWORD_CHANGE_REQUIRED")
	}
	var count int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?",
		actor.AdminID, permission,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fail(403, "FORBIDDEN")
	}
	return nil
}
