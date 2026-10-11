package identity

import (
	"context"
	"database/sql"
)

// Shared by future administrator editing; no role-name shortcuts.
func (s *Service) SetEnabled(ctx context.Context, actor Actor, target int64, enabled bool, requestID string) error {
	if !actor.Can("admin.write") {
		return fail(403, "PERMISSION_DENIED")
	}
	if !actor.Recent(s.Now()) {
		return fail(403, "REAUTH_REQUIRED")
	}
	return s.transaction(ctx, func(tx *sql.Tx) error {
		var guard int64
		if e := tx.QueryRowContext(ctx, "SELECT id FROM system_configs WHERE config_key='identity_guard' FOR UPDATE").Scan(&guard); e != nil {
			return e
		}
		fresh, e := lockAdmin(ctx, tx, actor.AdminID)
		if e != nil || !fresh.Enabled || fresh.AuthVersion != actor.AuthVersion {
			return fail(401, "AUTH_REQUIRED")
		}
		p, e := loadPermissions(ctx, tx, actor.AdminID)
		if e != nil {
			return e
		}
		freshActor := actor
		freshActor.Permissions = p
		freshActor.Force = fresh.Force
		if !freshActor.Can("admin.write") {
			return fail(403, "PERMISSION_DENIED")
		}
		if _, e = lockAdmin(ctx, tx, target); e != nil {
			return fail(404, "NOT_FOUND")
		}
		if _, e = tx.ExecContext(ctx, "UPDATE admin_users SET enabled=?,auth_version=auth_version+1,version=version+1 WHERE id=?", enabled, target); e != nil {
			return e
		}
		var n int
		e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users u WHERE u.enabled=1 AND EXISTS(SELECT 1 FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=u.id AND p.permission_key='admin.write') AND EXISTS(SELECT 1 FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=u.id AND p.permission_key='role.write')`).Scan(&n)
		if e != nil {
			return e
		}
		if n == 0 {
			return fail(409, "LAST_ADMIN_REQUIRED")
		}
		if e = revokeAdmin(ctx, tx, target); e != nil {
			return e
		}
		return audit(ctx, tx, actor.AdminID, requestID, "admin.enabled_changed")
	})
}
