package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	transport "jstihuo/internal/transport/http"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
)

// exerciseOrderManagement 用途：验证订单查询提权、编辑、取消、回收与 PII 边界。
// 参数：t Go 测试上下文；db 业务数据库。
// 参数：svc 隔离库业务服务；admin 测试管理员主体。
// 参数：setClock 固定测试时钟的控制回调。
// 返回值：无。
// 约束：断言失败报告错误，所有变更仅作用于当前测试子库。
func exerciseOrderManagement(
	t *testing.T, db *sql.DB, svc *order.Service, admin identity.Actor,
	setClock func(time.Time),
) {
	t.Helper()
	ctx := context.Background()
	var roleID int64
	if e := db.QueryRow("SELECT role_id FROM admin_user_roles WHERE admin_id=?", admin.AdminID).Scan(&roleID); e != nil {
		t.Fatal(e)
	}
	for _, permission := range []string{"order.write", "order.cancel", "order.pii", "order.trash", "order.restore"} {
		if _, e := db.Exec("INSERT INTO permissions(permission_key,description) VALUES(?,?)", permission, permission); e != nil {
			t.Fatal(e)
		}
		if _, e := db.Exec("INSERT INTO role_permissions(role_id,permission_id) SELECT ?,id FROM permissions WHERE permission_key=?", roleID, permission); e != nil {
			t.Fatal(e)
		}
	}
	page, e := svc.List(ctx, order.Filter{Limit: 20, Status: "PENDING"})
	if e != nil || len(page.Items) < 5 {
		t.Fatal("fixture pending orders", e)
	}
	first, second := page.Items[0], page.Items[1]
	query := func(identifier, phone string) (identity.LoginResult, error) {
		pre, e := svc.Auth.Bootstrap(ctx)
		if e != nil {
			return pre, e
		}
		actor, e := svc.Auth.Authenticate(ctx, pre.Token, "PREAUTH")
		if e != nil {
			return pre, e
		}
		challenge, e := svc.Auth.Challenge(ctx, actor, "ORDER_QUERY", "e1-query")
		if e != nil {
			return pre, e
		}
		input := order.QueryCredentials{Identifier: identifier, Phone: phone, ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}
		return svc.Query(ctx, actor, input, "e1-query", "query-test")
	}
	if _, e = query(first.Number, "13800000001"); e == nil {
		t.Fatal("wrong phone accepted")
	}
	session, e := query(first.Number, "10000000000")
	if e != nil {
		t.Fatal(e)
	}
	read, e := svc.Auth.Authenticate(ctx, session.Token, "ORDER")
	if e != nil || read.Scope != "READ" {
		t.Fatal("READ", e)
	}
	edited := order.EditInput{Version: first.Version, Address: redemption.Address{
		Name: "新的合成收件人", Phone: "10000000000", Province: "110000", City: "110100", District: "110101", Detail: "合成新地址，请勿寄送",
	}, Date: "2026-10-17", Note: "replacement address"}
	if _, e = svc.Edit(ctx, read, first.ID, edited, "e1-read-cannot-write", "test"); e == nil {
		t.Fatal("READ wrote")
	}
	var code string
	if e = db.QueryRow("SELECT v.code FROM vouchers v JOIN voucher_redemptions r ON r.voucher_id=v.id JOIN orders o ON o.redemption_id=r.id WHERE o.id=?", first.ID).Scan(&code); e != nil {
		t.Fatal(e)
	}
	pre, e := svc.Auth.Bootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	pa, e := svc.Auth.Authenticate(ctx, pre.Token, "PREAUTH")
	if e != nil {
		t.Fatal(e)
	}
	challenge, e := svc.Auth.Challenge(ctx, pa, "VOUCHER", "e1-stepup")
	if e != nil {
		t.Fatal(e)
	}
	upgraded, e := svc.StepUp(ctx, read, pa, redemption.Credentials{Code: code, Secret: "TESTONLY1234", ChallengeID: challenge.ID, Answer: solve(t, challenge.Image)}, "e1-stepup", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Auth.Authenticate(ctx, session.Token, "ORDER"); e == nil {
		t.Fatal("old session retained")
	}
	write, e := svc.Auth.Authenticate(ctx, upgraded.Token, "ORDER")
	if e != nil || write.Scope != "WRITE" {
		t.Fatal("WRITE", e)
	}
	if _, e = svc.Edit(ctx, write, second.ID, edited, "e1-other-order-edit", "test"); e == nil {
		t.Fatal("cross order write")
	}
	var reservedBefore int
	if e = db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reservedBefore); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO shipping_calendars(ship_date,capacity,used_slots) VALUES('2026-10-17',1,1) ON DUPLICATE KEY UPDATE capacity=1,used_slots=1"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Edit(ctx, write, first.ID, edited, "e1-full-new-calendar", "test"); e == nil {
		t.Fatal("full day edit")
	}
	current, e := svc.Redemption.GetOrder(ctx, first.ID, false)
	if e != nil || current.Date != first.Date || current.Version != first.Version {
		t.Fatal("old slot lost", current, e)
	}
	if _, e = db.Exec("UPDATE shipping_calendars SET used_slots=0 WHERE ship_date='2026-10-17'"); e != nil {
		t.Fatal(e)
	}
	svc.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "calendar" {
			return errors.New("calendar fault")
		}
		return nil
	}
	if _, e = svc.Edit(ctx, write, first.ID, edited, "e1-fault-calendar-01", "test"); e == nil {
		t.Fatal("calendar fault ignored")
	}
	svc.FaultTx = nil
	var targetSlots int
	if e = db.QueryRow("SELECT used_slots FROM shipping_calendars WHERE ship_date='2026-10-17'").Scan(&targetSlots); e != nil || targetSlots != 0 {
		t.Fatal("target slot partial", e)
	}
	current, e = svc.Edit(ctx, write, first.ID, edited, "e1-edit-success-0001", "test")
	if e != nil || current.Date != edited.Date {
		t.Fatal(current, e)
	}
	replay, e := svc.Edit(ctx, write, first.ID, edited, "e1-edit-success-0001", "test")
	if e != nil || replay.Version != current.Version {
		t.Fatal("edit replay", e)
	}
	if _, e = svc.Edit(ctx, write, first.ID, edited, "e1-stale-version-01", "test"); e == nil {
		t.Fatal("stale version")
	}
	var reservedAfter int
	if e = db.QueryRow("SELECT SUM(reserved) FROM inventory_balances").Scan(&reservedAfter); e != nil || reservedAfter != reservedBefore {
		t.Fatal("edit changed stock", e)
	}

	// 关闭新预约不应取消既有日期，也不能阻止截止前只替换原日地址。
	if _, e = db.Exec("INSERT INTO system_configs(config_key,value) VALUES('shipping_calendar',JSON_OBJECT('blackout_dates',JSON_ARRAY('2026-10-17'))) ON DUPLICATE KEY UPDATE value=VALUES(value)"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE shipping_calendars SET closed=1 WHERE ship_date='2026-10-17'"); e != nil {
		t.Fatal(e)
	}
	sameDay := edited
	sameDay.Version = current.Version
	sameDay.Note = "same date retained"
	current, e = svc.Edit(ctx, write, first.ID, sameDay, "e1-keep-existing-day", "test")
	if e != nil {
		t.Fatal("existing day lost", e)
	}
	advice, e := svc.Redemption.OrderCalendar(ctx, write)
	if e != nil {
		t.Fatal(e)
	}
	retained := false
	for _, day := range advice.Items {
		if day.Date == "2026-10-17" && day.Available {
			retained = true
		}
	}
	if !retained {
		t.Fatal("existing date unavailable in advice")
	}
	if _, e = db.Exec("DELETE FROM system_configs WHERE config_key='shipping_calendar'"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE shipping_calendars SET closed=0 WHERE ship_date='2026-10-17'"); e != nil {
		t.Fatal(e)
	}
	view, e := svc.PrivateView(ctx, admin, first.ID, order.VersionReason{Version: current.Version, Reason: "delivery check"}, "e1-private-view-001", "test")
	if e != nil || view.Address.Detail != edited.Address.Detail {
		t.Fatal("private view", e)
	}
	view, e = svc.PrivateView(ctx, admin, first.ID, order.VersionReason{Version: current.Version, Reason: "delivery check"}, "e1-private-view-001", "test")
	if e != nil || view.Address.Detail != edited.Address.Detail {
		t.Fatal("private replay", e)
	}
	var reads int
	if e = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='privateOrder' AND entity_id=?", first.ID).Scan(&reads); e != nil || reads != 2 {
		t.Fatal("PII access audit", reads, e)
	}
	// Same target capacity and crossed dates must conserve old and new slots.
	editSecond := edited
	editSecond.Version = second.Version
	if _, e = svc.Edit(ctx, admin, second.ID, editSecond, "e1-second-full-date", "test"); e == nil {
		t.Fatal("capacity overbooking")
	}
	cancelInput := order.VersionReason{Version: current.Version, Reason: "synthetic cancellation"}
	svc.FaultTx = func(_ context.Context, _ *sql.Tx, step string) error {
		if step == "audit" {
			return errors.New("audit unavailable")
		}
		return nil
	}
	if _, e = svc.Cancel(ctx, admin, first.ID, cancelInput, "e1-cancel-audit-fail", "test"); e == nil {
		t.Fatal("audit failure ignored")
	}
	svc.FaultTx = nil
	current, e = svc.Redemption.GetOrder(ctx, first.ID, false)
	if e != nil || current.Status != "PENDING" {
		t.Fatal("audit failure committed", e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := svc.Cancel(ctx, admin, first.ID, cancelInput, fmt.Sprintf("e1-concurrent-cancel-%d", i), "test")
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success, failed := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else {
			failed++
		}
	}
	if success != 1 || failed != 1 {
		t.Fatal("cancel count", success, failed)
	}
	current, e = svc.Redemption.GetOrder(ctx, first.ID, false)
	if e != nil || current.Status != "CANCELLED" {
		t.Fatal(current, e)
	}
	var released, slot, consumed int
	if e = db.QueryRow("SELECT COUNT(*) FROM inventory_reservations r JOIN order_items i ON i.id=r.order_item_id WHERE i.order_id=? AND r.state='RELEASED'", first.ID).Scan(&released); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow("SELECT used_slots FROM shipping_calendars WHERE ship_date='2026-10-17'").Scan(&slot); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow("SELECT COUNT(*) FROM vouchers v JOIN voucher_redemptions r ON r.voucher_id=v.id JOIN orders o ON o.redemption_id=r.id WHERE o.id=? AND v.redeemed_at IS NOT NULL", first.ID).Scan(&consumed); e != nil {
		t.Fatal(e)
	}
	if released != 2 || slot != 0 || consumed != 1 {
		t.Fatal("cancel invariants", released, slot, consumed)
	}

	var replayKey string
	if e = db.QueryRow("SELECT request_key FROM idempotency_keys WHERE scope=? AND request_key LIKE 'e1-concurrent-cancel-%'", fmt.Sprintf("admin:%d:orderCancel", admin.AdminID)).Scan(&replayKey); e != nil {
		t.Fatal(e)
	}
	replayCancel, e := svc.Cancel(ctx, admin, first.ID, cancelInput, replayKey, "replay-cancel")
	if e != nil || replayCancel.Status != "CANCELLED" {
		t.Fatal("cancel replay", e)
	}
	var releaseEvents int
	if e = db.QueryRow("SELECT COUNT(*) FROM inventory_transactions WHERE event_key LIKE ?", "release:"+first.ID+":%").Scan(&releaseEvents); e != nil || releaseEvents != 2 {
		t.Fatal("replay double release", releaseEvents, e)
	}
	trashed, e := svc.Visibility(ctx, admin, first.ID, order.VersionReason{Version: current.Version, Reason: "archive"}, false, "e1-trash-order-0001", "test")
	if e != nil || !trashed.Deleted || trashed.Status != "CANCELLED" {
		t.Fatal(trashed, e)
	}
	restored, e := svc.Visibility(ctx, admin, first.ID, order.VersionReason{Version: trashed.Version, Reason: "restore"}, true, "e1-restore-order-01", "test")
	if e != nil || restored.Deleted || restored.Status != "CANCELLED" {
		t.Fatal(restored, e)
	}
	if _, e = svc.Visibility(ctx, admin, second.ID, order.VersionReason{Version: second.Version, Reason: "invalid"}, false, "e1-pending-trash-01", "test"); e == nil {
		t.Fatal("pending trashed")
	}

	// 两个订单以相反方向改期；锁序相同，成功或失败都不能丢失总预约名额。
	livePage, e := svc.List(ctx, order.Filter{Limit: 20, Status: "PENDING"})
	if e != nil {
		t.Fatal(e)
	}
	left, right := livePage.Items[0], livePage.Items[1]
	var slotsBefore, slotsAfter int
	if e = db.QueryRow("SELECT SUM(used_slots) FROM shipping_calendars").Scan(&slotsBefore); e != nil {
		t.Fatal(e)
	}
	swapErrors := make(chan error, 2)
	for index, current := range []redemption.Order{left, right} {
		wg.Add(1)
		go func(index int, current redemption.Order) {
			defer wg.Done()
			newDate := left.Date
			if index == 0 {
				newDate = right.Date
			}
			_, e := svc.Edit(ctx, admin, current.ID, order.EditInput{Version: current.Version, Address: edited.Address, Date: newDate, Note: "cross date edit"}, fmt.Sprintf("e1-cross-date-%06d", index), "test")
			swapErrors <- e
		}(index, current)
	}
	wg.Wait()
	close(swapErrors)
	for e := range swapErrors {
		if e != nil {
			var f *identity.Fault
			if !errors.As(e, &f) || f.Code != "CAPACITY_EXCEEDED" {
				t.Fatal("cross-date error", e)
			}
		}
	}
	if e = db.QueryRow("SELECT SUM(used_slots) FROM shipping_calendars").Scan(&slotsAfter); e != nil || slotsAfter != slotsBefore {
		t.Fatal("cross-date slot loss", slotsBefore, slotsAfter, e)
	}
	second, e = svc.Redemption.GetOrder(ctx, second.ID, false)
	if e != nil {
		t.Fatal(e)
	}
	editSecond.Version = second.Version
	originalClock := svc.Redemption.Now()
	defer setClock(originalClock)
	setClock(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	editSecond.Date = second.Date
	if _, e = svc.Edit(ctx, admin, second.ID, editSecond, "e1-edit-after-cutoff", "test"); e == nil {
		t.Fatal("deadline ignored")
	}
	setClock(originalClock)
	exerciseOrderHTTP(t, svc, second, edited.Address)

}

// exerciseOrderHTTP 用途：验证顾客订单查询、提权和编辑的真实 HTTP 边界。
// 参数：t Go 测试上下文；svc 隔离库业务服务。
// 参数：target 待发测试订单；address 已规范化的完整地址。
// 返回值：无。
// 约束：只通过本地 Handler 请求，维护两类 HttpOnly Cookie 和对应 CSRF，失败报告断言。
func exerciseOrderHTTP(t *testing.T, svc *order.Service, target redemption.Order, address redemption.Address) {
	t.Helper()
	auth := transport.IdentityHandler{Service: svc.Auth, Origin: "http://localhost:8080", Development: true}
	handler := transport.Router(func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)), transport.WithOrders(svc, auth, transport.WithRedemption(svc.Redemption, auth, transport.API(auth, svc.Redemption.Catalog))))
	cookies := map[string]*http.Cookie{}
	tokens := map[string]string{}
	call := func(method, path string, input any, scope, key string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if input != nil {
			if e := json.NewEncoder(&body).Encode(input); e != nil {
				t.Fatal(e)
			}
		}
		r := httptest.NewRequest(method, "/api/v1/"+path, &body)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", tokens[scope])
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		for _, cookie := range w.Result().Cookies() {
			if cookie.MaxAge < 0 {
				delete(cookies, cookie.Name)
			} else {
				cookies[cookie.Name] = cookie
			}
		}
		if token := w.Header().Get("X-CSRF-Token"); token != "" {
			tokens[w.Header().Get("X-CSRF-Scope")] = token
		}
		return w
	}
	challenge := func(purpose string) identity.Challenge {
		w := call("GET", "session/bootstrap", nil, "", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var session identity.Session
		if e := json.Unmarshal(w.Body.Bytes(), &session); e != nil {
			t.Fatal(e)
		}
		tokens["preauth"] = session.CSRF
		w = call("POST", "challenges", map[string]string{"purpose": purpose}, "preauth", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var c identity.Challenge
		if e := json.Unmarshal(w.Body.Bytes(), &c); e != nil {
			t.Fatal(e)
		}
		return c
	}
	c := challenge("ORDER_QUERY")
	w := call("POST", "order-sessions", order.QueryCredentials{Identifier: target.Number, Phone: "10000000000", ChallengeID: c.ID, Answer: solve(t, c.Image)}, "preauth", "")
	if w.Code != 200 || w.Header().Get("X-Order-Scope") != "READ" {
		t.Fatal("query HTTP", w.Code, w.Body.String())
	}
	var session identity.Session
	if e := json.Unmarshal(w.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	tokens["order"] = session.CSRF
	input := order.EditInput{Version: target.Version, Address: address, Date: target.Date, Note: "HTTP replacement"}
	w = call("PATCH", "order", input, "order", "e1-http-read-denied")
	if w.Code != 403 {
		t.Fatal("READ HTTP wrote", w.Code, w.Body.String())
	}
	c = challenge("VOUCHER")
	var code string
	if e := svc.DB.QueryRow("SELECT v.code FROM vouchers v JOIN voucher_redemptions r ON r.voucher_id=v.id JOIN orders o ON o.redemption_id=r.id WHERE o.id=?", target.ID).Scan(&code); e != nil {
		t.Fatal(e)
	}
	w = call("POST", "order-session/step-up", redemption.Credentials{Code: code, Secret: "TESTONLY1234", ChallengeID: c.ID, Answer: solve(t, c.Image)}, "order", "")
	if w.Code != 200 || w.Header().Get("X-Order-Scope") != "WRITE" {
		t.Fatal("stepup HTTP", w.Code, w.Body.String())
	}
	if e := json.Unmarshal(w.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	tokens["order"] = session.CSRF
	w = call("GET", "redemption/calendar", nil, "order", "")
	if w.Code != 200 {
		t.Fatal("order calendar HTTP", w.Code, w.Body.String())
	}
	w = call("PATCH", "order", input, "order", "e1-http-write-ok-01")
	if w.Code != 200 {
		t.Fatal("WRITE HTTP", w.Code, w.Body.String())
	}
	w = call("PATCH", "order", input, "order", "e1-http-write-ok-01")
	if w.Code != 200 {
		t.Fatal("HTTP edit replay", w.Code, w.Body.String())
	}
}
