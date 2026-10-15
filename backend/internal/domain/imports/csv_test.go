package imports

import (
	"strings"
	"testing"
)

// TestParse 用途：校验 CSV 语法、UTF-8、表头、大小和行数上限。
// 参数：t 测试上下文。
// 返回值：无，错误结果只能含行号、字段和固定代码。
func TestParse(t *testing.T) {
	header := "code,secret,valid_from,valid_until\n"
	cases := []struct {
		name, raw string
		count     int
		code      string
	}{
		{"正常", header + "CARD00000001,ABCDEF,2026-01-01T00:00:00Z,2027-01-01T00:00:00Z\n", 1, ""},
		{"BOM", "\ufeff" + header + "a,b,c,d\r\n", 1, ""},
		{"带引号", header + "\"a,b\",b,c,d\n", 1, ""},
		{"错误表头", "secret,code,valid_from,valid_until\na,b,c,d\n", 0, "INVALID_HEADER"},
		{"空文件", "", 0, "INVALID_UTF8_OR_SIZE"},
		{"仅表头", header, 0, "EMPTY_IMPORT"},
		{"无效UTF8", header + string([]byte{255}), 0, "INVALID_UTF8_OR_SIZE"},
		{"无效引号", header + "a,\"b,c,d\n", 0, "INVALID_CSV"},
		{"字段过多", header + "a,b,c,d,e\n", 0, "INVALID_CSV"},
		{"一千行", header + strings.Repeat("a,b,c,d\n", 1000), 1000, ""},
		{"超行数", header + strings.Repeat("a,b,c,d\n", 1001), 0, "ROW_LIMIT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, errs := Parse("IMPORT_VOUCHERS", []byte(tc.raw))
			if len(rows) != tc.count {
				t.Fatal("行数不符")
			}
			if tc.code == "" && len(errs) != 0 || tc.code != "" && (len(errs) != 1 || errs[0].Code != tc.code) {
				t.Fatalf("脱敏错误不符: %v", errs)
			}
		})
	}
}

// TestSafeCell 用途：验证公式前缀、空白绕过与普通文本导出。
// 参数：t 测试上下文。
// 返回值：无，危险单元格必须前置单引号。
func TestSafeCell(t *testing.T) {
	for _, value := range []string{"=1+1", "+SUM(A1)", "-1", "@evil", "\t=1", "\r=1", "  =1"} {
		if SafeCell(value) != "'"+value {
			t.Fatal("未转义公式前缀")
		}
	}
	for _, value := range []string{"订单O_123", "普通文本", "", "123"} {
		if SafeCell(value) != value {
			t.Fatal("普通值发生变化")
		}
	}
}

// FuzzParse 用途：验证任意小输入不会导致解析 panic 或回显敏感值。
// 参数：f 模糊测试上下文。
// 返回值：无。
func FuzzParse(f *testing.F) {
	f.Add([]byte("code,secret,valid_from,valid_until\na,b,c,d\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1024*1024 {
			return
		}
		_, errs := Parse("IMPORT_VOUCHERS", raw)
		for _, err := range errs {
			if err.Row < 1 || len(err.Code) > 40 || len(err.Field) > 20 {
				t.Fatal("错误结果不符")
			}
		}
	})
}
