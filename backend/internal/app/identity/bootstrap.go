package identity

import (
	"context"
	"database/sql"
	"fmt"
	crypt "jstihuo/internal/infra/crypto"
	"unicode/utf8"
)

// Initial administrator creation is an offline, empty-identity-store operation.
func (s *Service) CreateInitialAdmin(ctx context.Context, name, password string) error {
	if utf8.RuneCountInString(name) < 3 || utf8.RuneCountInString(name) > 64 || !validPassword(password) {
		return fail(422, "INVALID_ARGUMENT")
	}
	hash, e := crypt.Hash(password)
	if e != nil {
		return e
	}
	return s.transaction(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, "INSERT INTO system_configs(config_key,value) VALUES('identity_guard',JSON_OBJECT()) ON DUPLICATE KEY UPDATE config_key=config_key"); e != nil {
			return e
		}
		var guard int64
		if e := tx.QueryRowContext(ctx, "SELECT id FROM system_configs WHERE config_key='identity_guard' FOR UPDATE").Scan(&guard); e != nil {
			return e
		}
		var count int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count); e != nil {
			return e
		}
		if count != 0 {
			return fail(409, "ALREADY_INITIALIZED")
		}
		r, e := tx.ExecContext(ctx, "INSERT INTO admin_users(login_name,password_hash) VALUES(?,?)", name, hash)
		if e != nil {
			return e
		}
		n, e := r.LastInsertId()
		if e != nil {
			return e
		}
		r, e = tx.ExecContext(ctx, "INSERT INTO roles(name) VALUES('Bootstrap administrator')")
		if e != nil {
			return e
		}
		role, e := r.LastInsertId()
		if e != nil {
			return e
		}
		for _, p := range PermissionCatalogue {
			if _, e = tx.ExecContext(ctx, "INSERT INTO permissions(permission_key,description) VALUES(?,?) ON DUPLICATE KEY UPDATE description=VALUES(description)", p, p); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO role_permissions(role_id,permission_id) SELECT ?,id FROM permissions WHERE permission_key=?", role, p); e != nil {
				return e
			}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO admin_user_roles(admin_id,role_id) VALUES(?,?)", n, role); e != nil {
			return e
		}
		return audit(ctx, tx, n, "cli-bootstrap-"+fmt.Sprint(n), "admin.initial_created")
	})
}
