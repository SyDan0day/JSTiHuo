package catalog

import (
	"strings"
	"testing"
)

// TestValidateContent 用途：验证内容字段边界与 HTML 清洗契约。
// 参数：t 测试上下文。
// 返回值：无，非法输入被拒绝且合法正文保留安全语义。
func TestValidateContent(t *testing.T) {
	valid := ContentInput{Slug: "notice", Title: "公告", Kind: "PAGE", Body: "<p>你好</p>"}
	cases := []struct {
		name string
		edit func(*ContentInput)
		ok   bool
	}{
		{"正常", func(in *ContentInput) {}, true},
		{"空正文", func(in *ContentInput) { in.Body = "" }, true},
		{"标题边界", func(in *ContentInput) { in.Title = strings.Repeat("字", 120) }, true},
		{"超长标题", func(in *ContentInput) { in.Title = strings.Repeat("字", 121) }, false},
		{"空白标题", func(in *ContentInput) { in.Title = "  " }, false},
		{"非法路径", func(in *ContentInput) { in.Slug = "../notice" }, false},
		{"大写路径", func(in *ContentInput) { in.Slug = "Notice" }, false},
		{"路径上限", func(in *ContentInput) { in.Slug = strings.Repeat("a", 64) }, true},
		{"路径超长", func(in *ContentInput) { in.Slug = strings.Repeat("a", 65) }, false},
		{"正文超长", func(in *ContentInput) { in.Body = strings.Repeat("字", 50001) }, false},
		{"清洗膨胀", func(in *ContentInput) { in.Body = strings.Repeat("&", 10001) }, false},
		{"非法类型", func(in *ContentInput) { in.Kind = "SCRIPT" }, false},
		{"公告", func(in *ContentInput) { in.Kind = "ANNOUNCEMENT" }, true},
		{"横幅", func(in *ContentInput) { in.Kind = "BANNER" }, true},
		{"负排序", func(in *ContentInput) { in.Sort = -1 }, false},
		{"排序上限", func(in *ContentInput) { in.Sort = 1000000 }, true},
		{"排序超长", func(in *ContentInput) { in.Sort = 1000001 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.edit(&in)
			_, err := validateContent(in)
			if (err == nil) != tc.ok {
				t.Fatalf("通过=%v，错误=%v", tc.ok, err)
			}
		})
	}
	in := valid
	in.Body = `<p onclick="bad()">安全<strong>文字</strong></p><script>alert(1)</script>`
	out, err := validateContent(in)
	if err != nil || out.Body != "<p>安全<strong>文字</strong></p>" {
		t.Fatalf("清洗结果不符: %q, %v", out.Body, err)
	}
}
