package voucher

import (
	"crypto/rand"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const Letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
const Digits = "0123456789"
const Mixed = Letters + "23456789"
const CodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ValidID 用途：检查规范正十进制 uint64 标识。
// 参数：id 目标对象标识。
// 返回值：规范且未溢出时为 true。
func ValidID(id string) bool {
	n, e := strconv.ParseUint(id, 10, 64)
	return e == nil && n > 0 && strconv.FormatUint(n, 10) == id
}

// ValidSelection 用途：验证 ONE_OF/N_OF 与集合大小。
// 参数：mode 选品模式；count 必须选择的商品种数。
// 参数：size 权益集合大小。
// 返回值：规则匹配时为 true。
func ValidSelection(mode string, count, size int) bool {
	return ((mode == "ONE_OF" && count == 1) || (mode == "N_OF" && count >= 2 && count <= 99)) && size >= count && size <= 99
}

// ValidSecret 用途：验证密码长度白名单与格式。
// 参数：length 目标字符长度；format 凭证字符格式。
// 返回值：支持该组合时为 true。
func ValidSecret(length int, format string) bool {
	return strings.Contains(",6,8,9,10,11,12,16,24,32,", ","+strconv.Itoa(length)+",") && (format == "MIXED" || format == "DIGITS" || format == "LETTERS")
}

// Alphabet 用途：取得凭证生成字母表。
// 参数：format 已校验凭证格式。
// 返回值：对应无易混混合表、字母表或数字表。
// 约束：未知格式按混合表处理，调用前须校验。
func Alphabet(format string) string {
	if format == "DIGITS" {
		return Digits
	}
	if format == "LETTERS" {
		return Letters
	}
	return Mixed
}

// Random 用途：用拒绝采样生成无偏字符序列。
// 参数：reader 密码学随机源或测试替身；alphabet 可选字符表。
// 参数：n 生成字符数量。
// 返回值：字符串或随机源读取错误。
// 约束：生产不使用伪随机源。
func Random(reader io.Reader, alphabet string, n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		v, e := rand.Int(reader, big.NewInt(int64(len(alphabet))))
		if e != nil {
			return "", e
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b), nil
}

// Serial 用途：生成前缀加零补齐的顺序卡号。
// 参数：prefix 已规范化卡号前缀；start 十进制起始序号。
// 参数：length 目标字符长度；offset 非负顺序偏移量。
// 返回值：卡号与是否未溢出。
// 约束：以大整数计算，避免 24 位序号精度损失。
func Serial(prefix, start string, length, offset int) (string, bool) {
	n, ok := new(big.Int).SetString(start, 10)
	if !ok || n.Sign() < 0 {
		return "", false
	}
	n.Add(n, big.NewInt(int64(offset)))
	value := n.String()
	width := length - len(prefix)
	if width < 1 || len(value) > width {
		return "", false
	}
	return prefix + strings.Repeat("0", width-len(value)) + value, true
}

// ValidAt 用途：检查半开有效期。
// 参数：now 当前时刻；from 有效期起点。
// 参数：until 有效期终点。
// 返回值：位于 [from,until) 时为 true。
func ValidAt(now, from, until time.Time) bool { return !now.Before(from) && now.Before(until) }
