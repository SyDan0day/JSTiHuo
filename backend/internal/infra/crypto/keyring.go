package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
)

type Keyring struct {
	Active int
	Keys   map[int][]byte
}

// ParseKeyring 用途：严格解析 AES-256 版本化密钥环。
// 参数：raw 本地密钥环 JSON。
// 返回值：密钥环或版本、编码、长度、active 引用错误。
// 约束：不打印密钥。
func ParseKeyring(raw []byte) (Keyring, error) {
	var input struct {
		Active string            `json:"active"`
		Keys   map[string]string `json:"keys"`
	}
	if e := json.Unmarshal(raw, &input); e != nil {
		return Keyring{}, errors.New("invalid keyring")
	}
	out := Keyring{Keys: map[int][]byte{}}
	for label, value := range input.Keys {
		v, e := strconv.Atoi(label)
		if e != nil || v < 1 || uint64(v) > 4294967295 || strconv.Itoa(v) != label {
			return out, errors.New("invalid key version")
		}
		key, e := base64.StdEncoding.DecodeString(value)
		if e != nil || len(key) != 32 {
			return out, errors.New("invalid key")
		}
		out.Keys[v] = key
	}
	v, e := strconv.Atoi(input.Active)
	if e != nil || out.Keys[v] == nil {
		return out, errors.New("invalid active key")
	}
	out.Active = v
	return out, nil
}

// Encrypt 用途：使用 active 版本 AES-GCM 加密内存数据。
// 参数：plain 仅驻留内存的明文；aad 密文绑定上下文。
// 返回值：nonce 加密信封、密钥版本或密钥、随机源错误。
// 约束：每次使用随机 nonce，不写明文文件。
func (r Keyring) Encrypt(plain, aad []byte) ([]byte, int, error) {
	block, e := aes.NewCipher(r.Keys[r.Active])
	if e != nil {
		return nil, 0, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, 0, e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, 0, e
	}
	return gcm.Seal(nonce, nonce, plain, aad), r.Active, nil
}

// Decrypt 用途：按存储版本验证并解密 GCM 信封。
// 参数：data nonce 和认证密文信封；aad 密文绑定上下文。
// 参数：version 密文保存的密钥版本。
// 返回值：明文或缺失密钥、信封截断、认证失败错误。
// 约束：调用方负责清理明文缓冲。
func (r Keyring) Decrypt(data, aad []byte, version int) ([]byte, error) {
	block, e := aes.NewCipher(r.Keys[version])
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	if len(data) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("invalid ciphertext")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], aad)
}
