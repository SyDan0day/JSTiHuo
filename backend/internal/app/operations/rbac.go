package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

// GetAdmin 用途：读取已授权管理员的管理资料。
// 参数：ctx 上下文；id 管理员 ID。
// 返回值：账户与角色引用或参数、不存在、数据库错误，不返回凭据。
func (s *Service) GetAdmin(ctx context.Context, id string) (Admin, error) {
	if !rules.ValidID(id) {
		return Admin{}, fault(422, "INVALID_ARGUMENT")
	}
	return loadManagedAdmin(ctx, s.DB, id, false)
}

// GetRole 用途：读取已授权角色的完整权限定义。
// 参数：ctx 上下文；id 角色 ID。
// 返回值：角色或参数、不存在、数据库错误。
func (s *Service) GetRole(ctx context.Context, id string) (Role, error) {
	if !rules.ValidID(id) {
		return Role{}, fault(422, "INVALID_ARGUMENT")
	}
	return loadRole(ctx, s.DB, id, false)
}

// Admins 用途：分页读取管理员管理资料。
// 参数：ctx 上下文；f 固定筛选。
// 返回值：管理员页或数据库错误，不读取密码。
func (s *Service) Admins(ctx context.Context, f Filter) (Page[Admin], error) {
	page := Page[Admin]{Items: []Admin{}}
	ids, err := listManagedIDs(ctx, s.DB, "admin", f)
	if err != nil {
		return page, err
	}
	if len(ids) > f.Limit {
		ids = ids[:f.Limit]
		page.Next = ids[len(ids)-1]
	}
	for _, id := range ids {
		out, err := loadManagedAdmin(ctx, s.DB, id, false)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, out)
	}
	return page, nil
}

// Roles 用途：分页读取角色定义。
// 参数：ctx 上下文；f 固定筛选。
// 返回值：角色页或数据库错误。
func (s *Service) Roles(ctx context.Context, f Filter) (Page[Role], error) {
	page := Page[Role]{Items: []Role{}}
	ids, err := listManagedIDs(ctx, s.DB, "role", f)
	if err != nil {
		return page, err
	}
	if len(ids) > f.Limit {
		ids = ids[:f.Limit]
		page.Next = ids[len(ids)-1]
	}
	for _, id := range ids {
		out, err := loadRole(ctx, s.DB, id, false)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, out)
	}
	return page, nil
}

// replaceRolePermissions 用途：原子替换角色的固定权限引用。
// 参数：ctx 上下文；tx 身份保护事务；id 角色；keys 已验证权限键。
// 返回值：nil 或目录缺失、数据库错误。
func replaceRolePermissions(ctx context.Context, tx *sql.Tx, id string, keys []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id=?", id); err != nil {
		return err
	}
	for _, key := range keys {
		result, err := tx.ExecContext(ctx, "INSERT INTO role_permissions(role_id,permission_id) SELECT ?,id FROM permissions WHERE permission_key=?", id, key)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fault(422, "VALIDATION_FAILED")
		}
	}
	return nil
}

// CreateRole 用途：在当前权限上限内创建角色。
// 参数：ctx 上下文；a 管理员；in 完整角色；key 幂等键；rid 请求标识。
// 返回值：角色 JSON 或字段、授予上限、复核、数据库错误。
func (s *Service) CreateRole(ctx context.Context, a identity.Actor, in RoleInput, key, rid string) (json.RawMessage, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := validateRole(in); err != nil {
		return nil, err
	}
	command := mutation{Operation: "createRole", Permission: "role.write", Entity: "role", Key: key, RequestID: rid, Body: in, Status: 201, Recent: true, Identity: true}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		granted, err := actorPermissions(ctx, tx, a)
		if err != nil {
			return nil, "", err
		}
		if err = checkGrant(granted, in.Permissions); err != nil {
			return nil, "", err
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO roles(name,enabled) VALUES(?,?)", in.Name, in.Enabled)
		if err != nil {
			return nil, "", err
		}
		number, err := result.LastInsertId()
		if err != nil {
			return nil, "", err
		}
		id := fmt.Sprint(number)
		if err = replaceRolePermissions(ctx, tx, id, in.Permissions); err != nil {
			return nil, "", err
		}
		out, err := loadRole(ctx, tx, id, false)
		return out, id, err
	})
}

// revokeManagedAdmins 用途：改权后递增关联主体版本并撤销所有旧会话。
// 参数：ctx 上下文；tx 当前事务；ids 已按 ID 升序锁定的管理员。
// 返回值：nil 或数据库错误，包含当前操作者时其会话也失效。
func revokeManagedAdmins(ctx context.Context, tx *sql.Tx, ids []string) error {
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, "UPDATE admin_users SET auth_version=auth_version+1,version=version+1 WHERE id=?", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE admin_id=?", id); err != nil {
			return err
		}
	}
	return nil
}

// UpdateRole 用途：在身份保护锁中更新角色并撤销受影响会话。
// 参数：ctx 上下文；a 管理员；id 角色；in 完整版本化角色；key 幂等键；rid 请求标识。
// 返回值：角色 JSON 或最后管理员、授予上限、版本、复核错误。
// 并发：identity_guard 先于关联管理员升序锁，权限与版本原子变更。
func (s *Service) UpdateRole(ctx context.Context, a identity.Actor, id string, in RoleUpdate, key, rid string) (json.RawMessage, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	if err := validateRole(in.RoleInput); err != nil {
		return nil, err
	}
	command := mutation{Operation: "updateRole", Permission: "role.write", Entity: "role", Target: id, Key: key, RequestID: rid, Body: in, Recent: true, Identity: true}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		granted, err := actorPermissions(ctx, tx, a)
		if err != nil {
			return nil, "", err
		}
		if err = checkGrant(granted, in.Permissions); err != nil {
			return nil, "", err
		}
		ids, err := stringRows(ctx, tx, "SELECT u.id FROM admin_users u JOIN admin_user_roles ur ON ur.admin_id=u.id WHERE ur.role_id=? ORDER BY u.id FOR UPDATE", id)
		if err != nil {
			return nil, "", err
		}
		out, err := loadRole(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if out.Version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		if err = checkGrant(granted, out.Permissions); err != nil {
			return nil, "", err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE roles SET name=?,enabled=?,version=version+1 WHERE id=?", in.Name, in.Enabled, id); err != nil {
			return nil, "", err
		}
		if err = replaceRolePermissions(ctx, tx, id, in.Permissions); err != nil {
			return nil, "", err
		}
		if err = checkLastAdmin(ctx, tx); err != nil {
			return nil, "", err
		}
		if err = revokeManagedAdmins(ctx, tx, ids); err != nil {
			return nil, "", err
		}
		out, err = loadRole(ctx, tx, id, false)
		return out, id, err
	})
}

// checkRolesGrant 用途：校验所选角色启用且全部权限属于操作者。
// 参数：ctx 上下文；tx 身份保护事务；granted 当前上限；ids 请求角色；enabledOnly 是否拒绝停用角色。
// 返回值：nil 或角色、授予上限错误。
func checkRolesGrant(ctx context.Context, tx *sql.Tx, granted map[string]bool, ids []string, enabledOnly bool) error {
	for _, id := range ids {
		role, err := loadRole(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if enabledOnly && !role.Enabled {
			return fault(422, "VALIDATION_FAILED")
		}
		if err = checkGrant(granted, role.Permissions); err != nil {
			return err
		}
	}
	return nil
}

// replaceAdminRoles 用途：原子替换管理员角色关系。
// 参数：ctx 上下文；tx 身份保护事务；id 管理员；roles 已验证角色集合。
// 返回值：nil 或数据库错误。
func replaceAdminRoles(ctx context.Context, tx *sql.Tx, id string, roles []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM admin_user_roles WHERE admin_id=?", id); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err := tx.ExecContext(ctx, "INSERT INTO admin_user_roles(admin_id,role_id) VALUES(?,?)", id, role); err != nil {
			return err
		}
	}
	return nil
}

// CreateAdmin 用途：以临时密码创建强制改密管理员，禁止超出自身权限授予角色。
// 参数：ctx 上下文；a 管理员；in 登录名、临时密码和角色；key 幂等键；rid 请求标识。
// 返回值：管理员 JSON 或字段、授予、复核、数据库错误；不返回或审计密码。
func (s *Service) CreateAdmin(ctx context.Context, a identity.Actor, in AdminInput, key, rid string) (json.RawMessage, error) {
	in.Login = strings.TrimSpace(in.Login)
	if !validText(in.Login, 3, 64) || !validText(in.Password, 12, 128) || !validIDs(in.Roles, 20) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	hash, err := s.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	command := mutation{Operation: "createAdmin", Permission: "admin.write", Entity: "admin", Key: key, RequestID: rid, Body: in, Status: 201, Recent: true, Identity: true}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		granted, err := actorPermissions(ctx, tx, a)
		if err != nil {
			return nil, "", err
		}
		if err = checkRolesGrant(ctx, tx, granted, in.Roles, true); err != nil {
			return nil, "", err
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO admin_users(login_name,password_hash) VALUES(?,?)", in.Login, hash)
		if err != nil {
			return nil, "", err
		}
		number, err := result.LastInsertId()
		if err != nil {
			return nil, "", err
		}
		id := fmt.Sprint(number)
		if err = replaceAdminRoles(ctx, tx, id, in.Roles); err != nil {
			return nil, "", err
		}
		out, err := loadManagedAdmin(ctx, tx, id, false)
		return out, id, err
	})
}

// UpdateAdmin 用途：原子修改管理员启停和角色并撤销旧会话。
// 参数：ctx 上下文；a 管理员；id 目标；in 完整版本化定义；key 幂等键；rid 请求标识。
// 返回值：管理员 JSON 或最后管理员、越权授予、版本、复核错误。
func (s *Service) UpdateAdmin(ctx context.Context, a identity.Actor, id string, in AdminUpdate, key, rid string) (json.RawMessage, error) {
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !validIDs(in.Roles, 20) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	command := mutation{Operation: "updateAdmin", Permission: "admin.write", Entity: "admin", Target: id, Key: key, RequestID: rid, Body: in, Recent: true, Identity: true}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		granted, err := actorPermissions(ctx, tx, a)
		if err != nil {
			return nil, "", err
		}
		if err = checkRolesGrant(ctx, tx, granted, in.Roles, true); err != nil {
			return nil, "", err
		}
		out, err := loadManagedAdmin(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if out.Version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		if err = checkRolesGrant(ctx, tx, granted, out.Roles, false); err != nil {
			return nil, "", err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE admin_users SET enabled=? WHERE id=?", in.Enabled, id); err != nil {
			return nil, "", err
		}
		if err = replaceAdminRoles(ctx, tx, id, in.Roles); err != nil {
			return nil, "", err
		}
		if err = checkLastAdmin(ctx, tx); err != nil {
			return nil, "", err
		}
		if err = revokeManagedAdmins(ctx, tx, []string{id}); err != nil {
			return nil, "", err
		}
		out, err = loadManagedAdmin(ctx, tx, id, false)
		return out, id, err
	})
}
