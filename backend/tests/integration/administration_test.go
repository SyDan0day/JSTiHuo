package integration

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/redemption"
)

// managedLogin 用途：登录新建临时管理员并按真实流程完成改密和复核。
// 参数：t 测试上下文；f 夹具；login 登录名；temporary 临时密码。
// 返回值：有独立 Cookie 和 CSRF 的客户端，不记录密码。
func managedLogin(t *testing.T, f *fulfillmentFixture, login, temporary string) *fulfillmentClient {
	t.Helper()
	c := &fulfillmentClient{t: t, handler: f.handler, cookies: map[string]*http.Cookie{}, tokens: map[string]string{}}
	id, answer := c.challenge("LOGIN")
	session := response[identity.Session](t, c.call("POST", "admin/session", identity.Credentials{Login: login, Password: temporary, ChallengeID: id, Answer: answer}, "preauth", ""), 200)
	c.tokens["admin"] = session.CSRF
	if !session.ForceChange {
		t.Fatal("新管理员未强制改密")
	}
	response[map[string]any](t, c.call("GET", "admin/admins", nil, "admin", ""), 403)
	permanent := temporary + "-changed"
	response[map[string]any](t, c.call("POST", "admin/session/password", map[string]string{"current_password": temporary, "new_password": permanent}, "admin", testRequestKey(t)), 200)
	id, answer = c.challenge("LOGIN")
	session = response[identity.Session](t, c.call("POST", "admin/session", identity.Credentials{Login: login, Password: permanent, ChallengeID: id, Answer: answer}, "preauth", ""), 200)
	c.tokens["admin"] = session.CSRF
	session = response[identity.Session](t, c.call("POST", "admin/session/reauth", map[string]string{"password": permanent}, "admin", ""), 200)
	c.tokens["admin"] = session.CSRF
	return c
}

// TestAdministration 用途：验证权限授予上限、最后管理员与改权会话撤销。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，越权、缺失复核和并发绕过必须拒绝。
func TestAdministration(t *testing.T) {
	f := newFulfillment(t)
	c := f.client
	ctx := context.Background()
	admins := response[operations.Page[operations.Admin]](t, c.call("GET", "admin/admins", nil, "admin", ""), 200)
	roles := response[operations.Page[operations.Role]](t, c.call("GET", "admin/roles", nil, "admin", ""), 200)
	if len(admins.Items) != 1 || len(roles.Items) != 1 {
		t.Fatal("初始身份目录不完整")
	}
	initial, bootstrap := admins.Items[0], roles.Items[0]
	if read := response[operations.Admin](t, c.call("GET", "admin/admins/"+initial.ID, nil, "admin", ""), 200); read.ID != initial.ID || read.Version != initial.Version {
		t.Fatal("管理员详情与列表不一致")
	}
	if read := response[operations.Role](t, c.call("GET", "admin/roles/"+bootstrap.ID, nil, "admin", ""), 200); read.ID != bootstrap.ID || len(read.Permissions) != len(bootstrap.Permissions) {
		t.Fatal("角色详情与列表不一致")
	}
	response[map[string]any](t, c.call("PATCH", "admin/admins/"+initial.ID, operations.AdminUpdate{Version: initial.Version, Enabled: false, Roles: initial.Roles}, "admin", testRequestKey(t)), 409)
	response[map[string]any](t, c.call("PATCH", "admin/roles/"+bootstrap.ID, operations.RoleUpdate{Version: bootstrap.Version, RoleInput: operations.RoleInput{Name: bootstrap.Name, Enabled: false, Permissions: bootstrap.Permissions}}, "admin", testRequestKey(t)), 409)
	roleIn := operations.RoleInput{Name: "受限权限管理员", Enabled: true, Permissions: []string{"admin.read", "admin.write", "role.read", "role.write"}}
	if _, err := f.db.Exec("UPDATE sessions SET reauth_at=NULL WHERE kind='ADMIN'"); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, c.call("POST", "admin/roles", roleIn, "admin", testRequestKey(t)), 403)
	if _, err := f.db.Exec("UPDATE sessions SET reauth_at=? WHERE kind='ADMIN'", f.now); err != nil {
		t.Fatal(err)
	}
	key := testRequestKey(t)
	role := response[operations.Role](t, c.call("POST", "admin/roles", roleIn, "admin", key), 201)
	if again := response[operations.Role](t, c.call("POST", "admin/roles", roleIn, "admin", key), 201); again.ID != role.ID {
		t.Fatal("角色幂等重放重复创建")
	}
	password := "Synthetic-Temporary-Password-123"
	user := response[operations.Admin](t, c.call("POST", "admin/admins", operations.AdminInput{Login: "limited-admin", Password: password, Roles: []string{role.ID}}, "admin", testRequestKey(t)), 201)
	limited := managedLogin(t, f, user.Login, password)
	directory := response[operations.Page[operations.Permission]](t, limited.call("GET", "admin/permissions?limit=100", nil, "admin", ""), 200)
	for _, p := range directory.Items {
		if p.Key == "config.write" {
			t.Fatal("权限目录显示不可授予权限")
		}
	}
	response[map[string]any](t, limited.call("POST", "admin/roles", operations.RoleInput{Name: "越权配置", Enabled: true, Permissions: []string{"config.write"}}, "admin", testRequestKey(t)), 403)
	response[map[string]any](t, limited.call("POST", "admin/admins", operations.AdminInput{Login: "escalated-admin", Password: password, Roles: []string{bootstrap.ID}}, "admin", testRequestKey(t)), 403)
	response[map[string]any](t, limited.call("PATCH", "admin/roles/"+bootstrap.ID, operations.RoleUpdate{Version: bootstrap.Version, RoleInput: roleIn}, "admin", testRequestKey(t)), 403)
	response[map[string]any](t, limited.call("GET", "admin/config", nil, "admin", ""), 403)
	updated := response[operations.Role](t, c.call("PATCH", "admin/roles/"+role.ID, operations.RoleUpdate{Version: role.Version, RoleInput: operations.RoleInput{Name: role.Name, Enabled: true, Permissions: []string{"admin.read"}}}, "admin", testRequestKey(t)), 200)
	if updated.Version != role.Version+1 {
		t.Fatal("角色版本未递增")
	}
	response[map[string]any](t, limited.call("GET", "admin/admins", nil, "admin", ""), 401)
	var authVersion int
	var sessions int
	if err := f.db.QueryRow("SELECT auth_version FROM admin_users WHERE id=?", user.ID).Scan(&authVersion); err != nil || authVersion < 3 {
		t.Fatal("改权未递增认证版本", err)
	}
	if err := f.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE admin_id=?", user.ID).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("改权未撤销会话", err)
	}
	for _, q := range []string{"status=ACTIVE", "sort=name", "limit=101", "q=x&q=y"} {
		response[map[string]any](t, c.call("GET", "admin/admins?"+q, nil, "admin", ""), 400)
	}
	// 恢复前检查原子失败：审计故障必须连同角色与关系回滚。
	if _, err := f.db.Exec("CREATE TRIGGER fail_role_audit BEFORE INSERT ON audit_logs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected'"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.operations.CreateRole(ctx, f.admin, operations.RoleInput{Name: "应回滚角色", Enabled: true, Permissions: []string{"admin.read"}}, testRequestKey(t), "test"); err == nil {
		t.Fatal("审计失败仍保存角色")
	}
	if _, err := f.db.Exec("DROP TRIGGER fail_role_audit"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM roles WHERE name='应回滚角色'").Scan(&count); err != nil || count != 0 {
		t.Fatal("角色审计失败未整体回滚")
	}
}

// TestConfigurationOperations 用途：验证固定参数、已有日占用保护和审计筛选。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，不能降低已占用容量、保存任意字段或假启用 LIVE。
func TestConfigurationOperations(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	c := f.client
	customer, _ := f.customer(t, 0)
	original := response[redemption.Order](t, customer.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	config := response[operations.Config](t, c.call("GET", "admin/config", nil, "admin", ""), 200)
	config.Capacity = 0
	response[map[string]any](t, c.call("PUT", "admin/config", config, "admin", testRequestKey(t)), 409)
	config.Capacity = 1
	config.MinLead = 0
	config.Blackouts = []string{f.input().Date}
	saved := response[operations.Config](t, c.call("PUT", "admin/config", config, "admin", testRequestKey(t)), 200)
	if saved.Version != 2 || saved.MinLead != 0 {
		t.Fatal("配置版本或提前天数未保存")
	}
	current := response[redemption.Order](t, customer.call("GET", "order", nil, "order", ""), 200)
	if current.Date != original.Date {
		t.Fatal("配置改变既有订单日期")
	}
	var used, capacity int
	var closed bool
	if err := f.db.QueryRow("SELECT used_slots,capacity,closed FROM shipping_calendars WHERE ship_date=?", f.input().Date).Scan(&used, &capacity, &closed); err != nil || used != 1 || capacity != 1 || !closed {
		t.Fatal("黑名单破坏既有占用", err)
	}
	response[map[string]any](t, c.call("PUT", "admin/config", config, "admin", testRequestKey(t)), 409)
	saved.NotificationMode = "LIVE"
	response[map[string]any](t, c.call("PUT", "admin/config", saved, "admin", testRequestKey(t)), 503)
	saved.NotificationMode = "DISABLED"
	saved.Blackouts = []string{}
	f.operations.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "calendar:10" {
			return errors.New("injected")
		}
		return nil
	}
	response[map[string]any](t, c.call("PUT", "admin/config", saved, "admin", testRequestKey(t)), 500)
	f.operations.FaultTx = nil
	unchanged := response[operations.Config](t, c.call("GET", "admin/config", nil, "admin", ""), 200)
	if len(unchanged.Blackouts) != 1 || unchanged.Version != 2 {
		t.Fatal("配置故障未整体回滚")
	}
	dashboard := response[operations.Dashboard](t, c.call("GET", "admin/dashboard", nil, "admin", ""), 200)
	if dashboard.Products != 3 || dashboard.Pending != 1 || dashboard.Vouchers != 1 {
		t.Fatal("统计口径不符")
	}
	page := response[operations.Page[operations.Audit]](t, c.call("GET", "admin/audit-logs?entity_type=Order&entity_id="+original.ID, nil, "admin", ""), 200)
	if len(page.Items) == 0 {
		t.Fatal("订单审计筛选为空")
	}
	for _, item := range page.Items {
		if !strings.EqualFold(item.Entity, "order") || item.EntityID != original.ID {
			t.Fatal("审计对象筛选失效")
		}
	}
	for _, q := range []string{"entity_type=arbitrary", "entity_id=01", "status=OPEN", "entity_id=1&entity_id=2"} {
		response[map[string]any](t, c.call("GET", "admin/audit-logs?"+q, nil, "admin", ""), 400)
	}
}

// TestLastAdminRace 用途：验证两个并发停用请求不能删除最后管理能力。
// 参数：t 测试上下文，需真实 MySQL。
// 返回值：无，身份保护锁串行化最终管理员不变量。
func TestLastAdminRace(t *testing.T) {
	f := newFulfillment(t)
	ctx := context.Background()
	roles := response[operations.Page[operations.Role]](t, f.client.call("GET", "admin/roles", nil, "admin", ""), 200)
	second := response[operations.Admin](t, f.client.call("POST", "admin/admins", operations.AdminInput{Login: "second-administrator", Password: "Synthetic-Temporary-123", Roles: []string{roles.Items[0].ID}}, "admin", testRequestKey(t)), 201)
	admins := response[operations.Page[operations.Admin]](t, f.client.call("GET", "admin/admins", nil, "admin", ""), 200)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, admin := range admins.Items {
		wg.Add(1)
		go func(target operations.Admin) {
			defer wg.Done()
			_, err := f.operations.UpdateAdmin(ctx, f.admin, target.ID, operations.AdminUpdate{Version: target.Version, Enabled: false, Roles: target.Roles}, testRequestKey(t), "test")
			errs <- err
		}(admin)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			var fault *identity.Fault
			if !errors.As(err, &fault) || (fault.Status != 409 && fault.Status != 401) {
				t.Fatal(err)
			}
		}
	}
	if success != 1 {
		t.Fatal("并发停用胜者数量", success)
	}
	var enabled int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM admin_users WHERE enabled=1 AND id IN (?,?)", f.admin.AdminID, second.ID).Scan(&enabled); err != nil || enabled != 1 {
		t.Fatal("最后管理员不变量被破坏", err)
	}
}
