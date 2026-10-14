package calendar

import (
	"testing"
	"time"
)

// TestWindow 用途：验证上海预约窗口、截止小时和 blackout。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，不连接业务库。
func TestWindow(t *testing.T) {
	c := Default()
	for _, test := range []struct {
		hour int
		date string
		ok   bool
	}{{16, "2026-10-13", true}, {17, "2026-10-13", false}, {17, "2026-10-14", true}, {16, "2026-11-11", true}, {16, "2026-11-12", false}, {16, "2026-10-12", false}} {
		now := time.Date(2026, 10, 12, test.hour, 0, 0, 0, Shanghai)
		if c.Allowed(now, test.date) != test.ok {
			t.Fatal(test)
		}
	}
	c.Blackouts = []string{"2026-10-14"}
	if c.Allowed(time.Date(2026, 10, 12, 17, 0, 0, 0, Shanghai), "2026-10-14") {
		t.Fatal("blackout")
	}
}
