package imports

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"unicode/utf8"
)

type Row struct {
	Number int
	Values []string
}

type RowError struct {
	Row   int    `json:"row"`
	Code  string `json:"code"`
	Field string `json:"field"`
}

// Parse 用途：按固定 UTF-8 CSV 契约读取最多一千行，不在错误中携带输入值。
// 参数：kind IMPORT_VOUCHERS 或 IMPORT_SHIPMENTS；raw 有界文件正文。
// 返回值：逻辑行与脱敏错误；语法或表头错误时不返回部分可导入记录。
func Parse(kind string, raw []byte) ([]Row, []RowError) {
	header := "code,secret,valid_from,valid_until"
	if kind == "IMPORT_SHIPMENTS" {
		header = "order_no,carrier_code,tracking_no,expected_version"
	}
	bad := func(row int, code, field string) ([]Row, []RowError) { return nil, []RowError{{row, code, field}} }
	if kind != "IMPORT_VOUCHERS" && kind != "IMPORT_SHIPMENTS" {
		return bad(1, "INVALID_KIND", "header")
	}
	if len(raw) == 0 || len(raw) > 5*1024*1024 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return bad(1, "INVALID_UTF8_OR_SIZE", "file")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = 4
	fields, err := reader.Read()
	if err != nil || strings.Join(fields, ",") != header {
		return bad(1, "INVALID_HEADER", "header")
	}
	rows := []Row{}
	for {
		values, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return bad(len(rows)+2, "INVALID_CSV", "row")
		}
		if len(rows) >= 1000 {
			return bad(1002, "ROW_LIMIT", "row")
		}
		rows = append(rows, Row{Number: len(rows) + 2, Values: values})
	}
	if len(rows) == 0 {
		return bad(2, "EMPTY_IMPORT", "row")
	}
	return rows, nil
}

// SafeCell 用途：防止 CSV 单元格被电子表格解释为公式。
// 参数：value 已允许导出的脱敏文本。
// 返回值：必要时前置单引号的文本，保留原值与 RFC4180 语义。
func SafeCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") ||
		(trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + value
	}
	return value
}
