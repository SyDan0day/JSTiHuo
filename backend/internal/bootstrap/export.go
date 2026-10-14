package bootstrap

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
)

// ExportKey 用途：读取 active 导出密钥。
// 参数：无。
// 返回值：32 字节密钥、版本或读取、配置错误。
// 约束：不输出密钥。
func ExportKey() ([]byte, int, error) {
	b, e := ReadSecret("EXPORT_KEYRING_FILE")
	if e != nil {
		return nil, 0, e
	}
	var ring struct {
		Active string            `json:"active"`
		Keys   map[string]string `json:"keys"`
	}
	if json.Unmarshal(b, &ring) != nil {
		return nil, 0, errors.New("invalid export keyring")
	}
	version, e := strconv.Atoi(ring.Active)
	if e != nil || version < 1 || uint64(version) > 4294967295 {
		return nil, 0, errors.New("export key version must be a positive uint32")
	}
	key, e := base64.StdEncoding.DecodeString(ring.Keys[ring.Active])
	if e != nil || len(key) != 32 {
		return nil, 0, errors.New("invalid export key")
	}
	return key, version, nil
}

// ExportKeys 用途：读取全部保留导出解密版本。
// 参数：无。
// 返回值：版本密钥映射或读取、格式、编码错误。
// 约束：保留旧版本用于授权解密，不输出密钥。
func ExportKeys() (map[int][]byte, error) {
	raw, e := ReadSecret("EXPORT_KEYRING_FILE")
	if e != nil {
		return nil, e
	}
	var ring struct {
		Keys map[string]string `json:"keys"`
	}
	if e = json.Unmarshal(raw, &ring); e != nil {
		return nil, e
	}
	keys := map[int][]byte{}
	for label, value := range ring.Keys {
		v, e := strconv.Atoi(label)
		if e != nil || v < 1 || uint64(v) > 4294967295 {
			return nil, errors.New("invalid export key version")
		}
		key, e := base64.StdEncoding.DecodeString(value)
		if e != nil || len(key) != 32 {
			return nil, errors.New("invalid export key")
		}
		keys[v] = key
	}
	return keys, nil
}
