package operations

import (
	"context"
	"database/sql"
	"strings"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

type Admin struct {
	ID      string   `json:"id"`
	Version int64    `json:"version"`
	Login   string   `json:"login_name"`
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"role_ids"`
}
type RoleInput struct {
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	Permissions []string `json:"permission_keys"`
}
type Role struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	RoleInput
}
type RoleUpdate struct {
	Version int64 `json:"version"`
	RoleInput
}
type AdminInput struct {
	Login    string   `json:"login_name"`
	Password string   `json:"temporary_password"`
	Roles    []string `json:"role_ids"`
}
type AdminUpdate struct {
	Version int64    `json:"version"`
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"role_ids"`
}
type Permission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// stringRows 用途：读取并关闭单列字符串查询。
// 参数：ctx 上下文；q 数据源；query 服务器固定 SQL；args 绑定参数。
// 返回值：非 nil 字符串集合或数据库错误。
func stringRows(ctx context.Context, q queryer, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// loadManagedAdmin 用途：读取管理员公开管理字段及角色引用。
// 参数：ctx 上下文；q 数据源；id 管理员；lock 是否排他锁定。
// 返回值：管理员或不存在、数据库错误，不返回密码与认证版本。
func loadManagedAdmin(ctx context.Context, q queryer, id string, lock bool) (Admin, error) {
	var out Admin
	query := "SELECT id,version,login_name,enabled FROM admin_users WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Version, &out.Login, &out.Enabled)
	if err == sql.ErrNoRows {
		return out, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	out.Roles, err = stringRows(ctx, q, "SELECT role_id FROM admin_user_roles WHERE admin_id=? ORDER BY role_id", id)
	return out, err
}

// loadRole 用途：读取角色及固定操作权限键。
// 参数：ctx 上下文；q 数据源；id 角色；lock 是否排他锁定。
// 返回值：角色或不存在、数据库错误。
func loadRole(ctx context.Context, q queryer, id string, lock bool) (Role, error) {
	var out Role
	query := "SELECT id,version,name,enabled FROM roles WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Version, &out.Name, &out.Enabled)
	if err == sql.ErrNoRows {
		return out, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	out.Permissions, err = stringRows(ctx, q, "SELECT p.permission_key FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.permission_key", id)
	return out, err
}

// validIDs 用途：验证角色引用数量、唯一性与 ID 格式。
// 参数：ids 待验证集合；max 最大数量。
// 返回值：合法时 true，不接受空集合或重复项。
func validIDs(ids []string, max int) bool {
	if len(ids) < 1 || len(ids) > max {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !rules.ValidID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// actorPermissions 用途：在身份保护锁内读取当前真实可授予权限。
// 参数：ctx 上下文；q 数据源；a 已授权管理员。
// 返回值：权限集合或数据库错误，不依赖客户端或旧会话缓存。
func actorPermissions(ctx context.Context, q queryer, a identity.Actor) (map[string]bool, error) {
	keys, err := stringRows(ctx, q, `SELECT DISTINCT p.permission_key FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=?`, a.AdminID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, key := range keys {
		out[key] = true
	}
	return out, nil
}

// checkGrant 用途：验证角色或权限没有超出操作者当前可授予集合。
// 参数：granted 当前权限集合；keys 请求授予或将被修改的已有权限。
// 返回值：nil 或越权错误，不以角色名称绕过校验。
func checkGrant(granted map[string]bool, keys []string) error {
	for _, key := range keys {
		if !granted[key] {
			return fault(403, "GRANT_EXCEEDS_AUTHORITY")
		}
	}
	return nil
}

// checkLastAdmin 用途：保证变更后仍存在同时拥有两项权限的启用管理员。
// 参数：ctx 上下文；tx 已持有 identity_guard 的事务。
// 返回值：nil 或 LAST_ADMIN_REQUIRED、数据库错误。
func checkLastAdmin(ctx context.Context, tx *sql.Tx) error {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users u WHERE u.enabled=1
	AND EXISTS(SELECT 1 FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=u.id AND p.permission_key='admin.write')
	AND EXISTS(SELECT 1 FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=u.id AND p.permission_key='role.write')`).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return fault(409, "LAST_ADMIN_REQUIRED")
	}
	return nil
}

// listManagedIDs 用途：按固定角色或管理员表分页筛选引用。
// 参数：ctx 上下文；db 数据库；kind 服务器固定资源种类；f 固定筛选。
// 返回值：降序对象 ID 或数据库错误。
func listManagedIDs(ctx context.Context, db *sql.DB, kind string, f Filter) ([]string, error) {
	query := "SELECT id FROM roles WHERE 1=1"
	column := "name"
	if kind == "admin" {
		query = "SELECT id FROM admin_users WHERE 1=1"
		column = "login_name"
	}
	args := []any{}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND enabled=?"
		args = append(args, f.Status == "ENABLED")
	}
	if f.Q != "" {
		query += " AND LOCATE(?," + column + ")>0"
		args = append(args, f.Q)
	}
	args = append(args, f.Limit+1)
	return stringRows(ctx, db, query+" ORDER BY id DESC LIMIT ?", args...)
}

// Permissions 用途：分页返回当前操作者拥有的可授予权限目录。
// 参数：ctx 上下文；a 管理员；f 固定 ID 游标与权限键搜索。
// 返回值：权限页或数据库错误，隐藏不能授予的操作。
func (s *Service) Permissions(ctx context.Context, a identity.Actor, f Filter) (Page[Permission], error) {
	out := Page[Permission]{Items: []Permission{}}
	query := `SELECT DISTINCT p.id,p.permission_key,p.description FROM permissions p JOIN role_permissions rp ON rp.permission_id=p.id JOIN roles r ON r.id=rp.role_id AND r.enabled=1 JOIN admin_user_roles ur ON ur.role_id=r.id WHERE ur.admin_id=?`
	args := []any{a.AdminID}
	if f.After != "" {
		query += " AND p.id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,p.permission_key)>0"
		args = append(args, f.Q)
	}
	args = append(args, f.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY p.id DESC LIMIT ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		var v Permission
		if err = rows.Scan(&id, &v.Key, &v.Description); err != nil {
			return out, err
		}
		out.Items = append(out.Items, v)
		ids = append(ids, id)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = ids[f.Limit-1]
	}
	return out, rows.Err()
}

// validateRole 用途：校验角色名和固定权限键集合。
// 参数：in 完整角色定义。
// 返回值：nil 或字段错误，不接受空权限、重复与不存在的目录键。
func validateRole(in RoleInput) error {
	if !validText(strings.TrimSpace(in.Name), 1, 64) || len(in.Permissions) < 1 || len(in.Permissions) > 200 {
		return fault(422, "VALIDATION_FAILED")
	}
	known := map[string]bool{}
	for _, key := range identity.PermissionCatalogue {
		known[key] = true
	}
	seen := map[string]bool{}
	for _, key := range in.Permissions {
		if !known[key] || seen[key] {
			return fault(422, "VALIDATION_FAILED")
		}
		seen[key] = true
	}
	return nil
}
