package operations

import (
	"strings"
	"testing"
)

// TestValidateFeedback 用途：验证生产手机号、邮箱、文本码点与挑战字段边界。
// 参数：t 测试上下文。
// 返回值：无，非法输入必须被拒绝，测试号码仅在显式测试模式允许。
func TestValidateFeedback(t *testing.T) {
	base := FeedbackInput{Name: "测试联系人", Phone: "13900000000", Message: "测试反馈", ChallengeID: "1", Answer: "1234"}
	cases := []struct {
		name     string
		change   func(*FeedbackInput)
		test, ok bool
	}{
		{"正常", func(in *FeedbackInput) {}, false, true},
		{"空白姓名", func(in *FeedbackInput) { in.Name = "  " }, false, false},
		{"空白内容", func(in *FeedbackInput) { in.Message = "  " }, false, false},
		{"码点上限", func(in *FeedbackInput) { in.Message = strings.Repeat("字", 200) }, false, true},
		{"码点超长", func(in *FeedbackInput) { in.Message = strings.Repeat("字", 201) }, false, false},
		{"生产拒绝测试号码", func(in *FeedbackInput) { in.Phone = "10000000000" }, false, false},
		{"测试号码", func(in *FeedbackInput) { in.Phone = "10000000000" }, true, true},
		{"邮箱", func(in *FeedbackInput) { in.Email = "test@example.invalid" }, false, true},
		{"拒绝显示名邮箱", func(in *FeedbackInput) { in.Email = "测试 <test@example.invalid>" }, false, false},
		{"错误邮箱", func(in *FeedbackInput) { in.Email = "invalid" }, false, false},
		{"非法挑战", func(in *FeedbackInput) { in.ChallengeID = "01" }, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.change(&in)
			_, err := validateFeedback(in, tc.test)
			if (err == nil) != tc.ok {
				t.Fatalf("允许=%v, 错误=%v", tc.ok, err)
			}
		})
	}
}
