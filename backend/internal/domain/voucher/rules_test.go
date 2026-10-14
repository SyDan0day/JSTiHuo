package voucher

import (
	"bytes"
	"testing"
	"time"
)

// TestRules 用途：验证卡券数量、随机采样、编号溢出和有效期边界。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，不连接业务库。
func TestRules(t *testing.T) {
	for _, c := range []struct {
		m    string
		n, s int
		ok   bool
	}{{"ONE_OF", 1, 1, true}, {"ONE_OF", 2, 3, false}, {"N_OF", 1, 3, false}, {"N_OF", 2, 1, false}, {"N_OF", 2, 2, true}, {"N_OF", 99, 99, true}} {
		if ValidSelection(c.m, c.n, c.s) != c.ok {
			t.Fatalf("selection: %+v", c)
		}
	}
	if v, ok := Serial("JT", "999", 12, 1); !ok || v != "JT0000001000" {
		t.Fatal(v, ok)
	}
	if _, ok := Serial("JT", "9999999999", 12, 1); ok {
		t.Fatal("overflow accepted")
	}
	for _, n := range []int{0, 5, 7, 13, 33} {
		if ValidSecret(n, "MIXED") {
			t.Fatal(n)
		}
	}
	for _, id := range []string{"0", "01", "-1", "18446744073709551616"} {
		if ValidID(id) {
			t.Fatal(id)
		}
	}
	s, e := Random(bytes.NewReader(bytes.Repeat([]byte{255, 0}, 32)), Mixed, 12)
	if e != nil || len(s) != 12 {
		t.Fatal(s, e)
	}
	from := time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC)
	until := from.Add(time.Hour)
	if !ValidAt(from, from, until) || !ValidAt(until.Add(-time.Microsecond), from, until) || ValidAt(until, from, until) {
		t.Fatal("[from, until) violated")
	}
}
