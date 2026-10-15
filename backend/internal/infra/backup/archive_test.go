package backup

import (
	"bytes"
	"errors"
	"testing"
)

// TestAuthenticatedArchive 用途：验证备份认证、随机性、路径白名单与输入边界。
// 参数：t 测试上下文。
// 返回值：无，错误密钥或篡改不得返回部分 SQL 与图片。
func TestAuthenticatedArchive(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	input := Archive{SQL: []byte("synthetic SQL"), Images: map[string][]byte{"images/0123456789abcdef0123456789abcdef.png": {137, 80, 78, 71, 13, 10, 26, 10}}}
	first, err := Pack(key, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Pack(key, input)
	if err != nil || bytes.Equal(first, second) {
		t.Fatal("备份 nonce 不随机", err)
	}
	out, err := Unpack(key, first)
	if err != nil || !bytes.Equal(out.SQL, input.SQL) || len(out.Images) != 1 {
		t.Fatal("备份不能恢复", err)
	}
	out.Clear()
	for _, data := range [][]byte{first[:10], append([]byte{}, first...)} {
		if len(data) > 10 {
			data[len(data)-1] ^= 1
		}
		out, err := Unpack(key, data)
		if err == nil || len(out.SQL) > 0 {
			t.Fatal("篡改返回了部分归档")
		}
	}
	_, err = Unpack(bytes.Repeat([]byte{8}, 32), first)
	if !errors.Is(err, ErrAuthentication) {
		t.Fatal("错误密钥未拒绝")
	}
	for _, name := range []string{"../secret", "/etc/passwd", "imports/0123456789abcdef.gcm", "credentials/export.csv", "images/../../outside.png"} {
		_, err := Pack(key, Archive{SQL: input.SQL, Images: map[string][]byte{name: input.Images["images/0123456789abcdef0123456789abcdef.png"]}})
		if !errors.Is(err, ErrInvalid) {
			t.Fatal("非法备份路径未拒绝")
		}
	}
}
