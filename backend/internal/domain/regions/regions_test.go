package regions

import (
	"testing"
)

// TestHierarchy 用途：验证版本化地区层级、不设区城市和 API 列表上限。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，不连接业务库。
func TestHierarchy(t *testing.T) {
	d, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if len(d.ByParent["000000"]) < 31 || len(d.ByCode) < 3000 {
		t.Fatal("incomplete dataset")
	}
	if !d.Validate("110000", "110100", "110101") || d.Validate("310000", "110100", "110101") {
		t.Fatal("parent validation")
	}
	if !d.Validate("440000", "441900", "441900") {
		t.Fatal("county-less city")
	}
	for _, children := range d.ByParent {
		if len(children) > 100 {
			t.Fatal("API page bound exceeded")
		}
	}
}
