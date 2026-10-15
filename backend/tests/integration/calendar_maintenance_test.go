package integration

import (
	"context"
	"testing"
	"time"

	"jstihuo/internal/app/redemption"
)

// TestCalendarWindowReplenishment 用途：验证跨日补建完整窗口且不改变原订单名额。
// 参数：t 测试上下文，需真实隔离 MySQL。
// 返回值：无；幂等补建及跨日扩展必须保留原容量与已占名额。
func TestCalendarWindowReplenishment(t *testing.T) {
	f := newFulfillment(t)
	f.seedFulfillment(t, 1)
	customer, _ := f.customer(t, 0)
	original := response[redemption.Order](t, customer.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	ctx := context.Background()
	service := f.operations.Orders.Redemption
	for i := 0; i < 2; i++ {
		days, err := service.CalendarDays(ctx, "")
		if err != nil || len(days.Items) != 31 {
			t.Fatal("日历窗口不完整", err)
		}
	}
	var before int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM shipping_calendars").Scan(&before); err != nil || before != 31 {
		t.Fatal("日历重复补建", before, err)
	}
	advanceFulfillment(t, f, 24*time.Hour)
	days, err := service.CalendarDays(ctx, "")
	if err != nil || len(days.Items) != 31 {
		t.Fatal("跨日窗口不完整", err)
	}
	var after, used int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM shipping_calendars").Scan(&after); err != nil || after != 32 {
		t.Fatal("跨日未补建新日", after, err)
	}
	if err = f.db.QueryRow("SELECT used_slots FROM shipping_calendars WHERE ship_date=?", original.Date).Scan(&used); err != nil || used != 1 {
		t.Fatal("补建改变原名额", used, err)
	}
	checkFulfillmentLedger(t, f.db)
}
