package voucher

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// TestBatchValidation 用途：验证批次配置规范化及非法字段拒绝。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，不连接业务库。
func TestBatchValidation(t *testing.T) {
	base := Input{TypeID: "1", Label: " batch ", Products: []string{"10", "2"}, Quantity: 1000, Prefix: " jt ", CodeLength: 16, CodeMode: "RANDOM", SecretLength: 12, SecretFormat: "MIXED", From: time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC), Until: time.Date(2027, 10, 12, 2, 0, 0, 0, time.UTC)}
	if e := validate(&base); e != nil || base.Products[0] != "2" || base.Prefix != "JT" {
		t.Fatal(base, e)
	}
	for _, change := range []func(*Input){func(b *Input) { b.Products = []string{"2", "2"} }, func(b *Input) { b.Quantity = 1001 }, func(b *Input) { b.Until = b.From }, func(b *Input) { b.CodeMode = "SEQUENTIAL"; b.Start = "99999999999999" }, func(b *Input) { b.SecretLength = 7 }, func(b *Input) { b.Start = "1" }} {
		v := base
		change(&v)
		if validate(&v) == nil {
			t.Fatal("invalid batch accepted", v)
		}
	}
}

// TestRandomCollisionBounded 用途：验证批内随机碰撞重采样的上限。
// 参数：t Go 测试上下文。
// 返回值：无。
// 约束：不满足断言时报告失败，不连接业务库。
func TestRandomCollisionBounded(t *testing.T) {
	s := New(nil, t.TempDir(), nil, bytes.Repeat([]byte{1}, 32), 1)
	s.Random = bytes.NewReader(make([]byte, 4096))
	s.Hash = func(string) (string, error) { return "test-hash", nil }
	_, e := s.credentials(context.Background(), Batch{Input: Input{Quantity: 2, CodeMode: "RANDOM", CodeLength: 12, SecretLength: 12, SecretFormat: "MIXED"}})
	if e == nil {
		t.Fatal("collision not bounded")
	}
}
