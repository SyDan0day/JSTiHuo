package calendar

import (
	"time"
)

var Shanghai = time.FixedZone("Asia/Shanghai", 8*3600)

type Config struct {
	MinLead   int      `json:"min_lead_days"`
	Cutoff    int      `json:"cutoff_hour"`
	Horizon   int      `json:"horizon_days"`
	Capacity  int      `json:"daily_capacity"`
	Blackouts []string `json:"blackout_dates"`
}

// Default 用途：提供既定预约默认值。
// 参数：无。
// 返回值：提前一天、17 点截止、30 天窗口及每日 100 单配置。
func Default() Config {
	return Config{MinLead: 1, Cutoff: 17, Horizon: 30, Capacity: 100, Blackouts: []string{}}
}

// Valid 用途：验证日历配置边界及 blackout 日期。
// 参数：无。
// 返回值：所有字段合法且日期不重复时为 true。
func (c Config) Valid() bool {
	if c.MinLead < 0 || c.MinLead > 30 || c.Cutoff < 0 || c.Cutoff > 23 || c.Horizon < 1 || c.Horizon > 30 || c.Capacity < 0 || c.Capacity > 100000 || len(c.Blackouts) > 366 {
		return false
	}
	seen := map[string]bool{}
	for _, d := range c.Blackouts {
		if _, e := time.Parse("2006-01-02", d); e != nil || seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}

// Day 用途：计算上海业务日零点。
// 参数：now 当前时刻。
// 返回值：当天上海零点。
// 约束：不使用主机时区。
func Day(now time.Time) time.Time {
	t := now.In(Shanghai)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Shanghai)
}

// Window 用途：计算预约最早和最晚业务日。
// 参数：now 当前时刻。
// 返回值：包含端点的日期窗口。
// 约束：达到截止小时额外延后一天。
func (c Config) Window(now time.Time) (time.Time, time.Time) {
	today := Day(now)
	lead := c.MinLead
	if now.In(Shanghai).Hour() >= c.Cutoff {
		lead++
	}
	return today.AddDate(0, 0, lead), today.AddDate(0, 0, c.Horizon)
}

// Allowed 用途：判断预约日期是否在窗口且不为 blackout。
// 参数：now 当前时刻；date 上海业务日期。
// 返回值：合法可预约时为 true。
// 约束：不检查数据库容量。
func (c Config) Allowed(now time.Time, date string) bool {
	day, e := time.ParseInLocation("2006-01-02", date, Shanghai)
	if e != nil {
		return false
	}
	first, last := c.Window(now)
	if day.Before(first) || day.After(last) {
		return false
	}
	for _, blackout := range c.Blackouts {
		if date == blackout {
			return false
		}
	}
	return true
}
