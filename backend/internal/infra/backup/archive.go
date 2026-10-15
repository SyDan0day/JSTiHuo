package backup

import (
	"archive/tar"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"regexp"
	"sort"
)

const MaxBytes = 512 * 1024 * 1024
const magic = "JSTBACKUP\x01"

var ErrInvalid = errors.New("BACKUP_INVALID")
var ErrAuthentication = errors.New("BACKUP_AUTH_FAILED")
var imageName = regexp.MustCompile(`^images/[a-f0-9]{32}\.png$`)

type Archive struct {
	SQL    []byte
	Images map[string][]byte
}

// Clear 用途：清理恢复归档中暂存的 SQL 与文件内存。
// 参数：无。
// 返回值：无，不删除调用方文件。
func (a *Archive) Clear() {
	clear(a.SQL)
	for key, data := range a.Images {
		clear(data)
		delete(a.Images, key)
	}
	a.SQL = nil
}

// Pack 用途：将 SQL 与公开图片白名单封装成独立密钥认证加密备份。
// 参数：key 为独立 32 字节备份密钥；archive 为最多 512 MiB 的归档输入。
// 返回值：包含版本、随机 nonce 与 GCM 标签的密文或格式、长度、随机源错误。
// 约束：不接收凭证导出、导入源、报表、密钥文件或任意路径。
func Pack(key []byte, archive Archive) ([]byte, error) {
	if len(key) != 32 || len(archive.SQL) == 0 || len(archive.SQL) > MaxBytes {
		return nil, ErrInvalid
	}
	buffer := new(bytes.Buffer)
	writer := tar.NewWriter(buffer)
	if err := writeEntry(writer, "db.sql", archive.SQL); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(archive.Images))
	total := len(archive.SQL)
	for name, data := range archive.Images {
		if !imageName.MatchString(name) || !pngHeader(data) {
			return nil, ErrInvalid
		}
		total += len(data)
		if total > MaxBytes {
			return nil, ErrInvalid
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		if err := writeEntry(writer, name, archive.Images[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	plain := buffer.Bytes()
	defer clear(plain)
	if len(plain) > MaxBytes {
		return nil, ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	header := append([]byte(magic), nonce...)
	return aead.Seal(header, nonce, plain, []byte(magic)), nil
}

// Unpack 用途：先认证全部归档密文，再解析严格路径白名单。
// 参数：key 为对应备份独立密钥；sealed 为有界备份字节。
// 返回值：已认证 SQL 与图片内存或认证、版本、路径、长度错误。
// 约束：认证失败前不执行 SQL、不写恢复目录；调用方使用后必须 Clear。
func Unpack(key, sealed []byte) (Archive, error) {
	out := Archive{Images: map[string][]byte{}}
	if len(key) != 32 || len(sealed) > MaxBytes+128 || len(sealed) < len(magic)+12+16 || string(sealed[:len(magic)]) != magic {
		return out, ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return out, ErrInvalid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return out, err
	}
	start := len(magic)
	nonce := sealed[start : start+aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, sealed[start+aead.NonceSize():], []byte(magic))
	if err != nil {
		return out, ErrAuthentication
	}
	defer clear(plain)
	reader := tar.NewReader(bytes.NewReader(plain))
	seen := map[string]bool{}
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			out.Clear()
			return out, ErrInvalid
		}
		valid := header.Name == "db.sql" || imageName.MatchString(header.Name)
		if !valid || seen[header.Name] || header.Typeflag != tar.TypeReg || header.Size < 1 || header.Size > MaxBytes-total {
			out.Clear()
			return out, ErrInvalid
		}
		seen[header.Name] = true
		total += header.Size
		data, err := io.ReadAll(io.LimitReader(reader, header.Size))
		if err != nil || int64(len(data)) != header.Size {
			out.Clear()
			return out, ErrInvalid
		}
		if header.Name == "db.sql" {
			out.SQL = data
		} else if pngHeader(data) {
			out.Images[header.Name] = data
		} else {
			clear(data)
			out.Clear()
			return out, ErrInvalid
		}
	}
	if len(out.SQL) == 0 {
		out.Clear()
		return out, ErrInvalid
	}
	return out, nil
}

// writeEntry 用途：以固定权限写入单个内存归档项。
// 参数：writer tar 写入器；name 已验证路径；data 文件字节。
// 返回值：nil 或归档、写入错误。
func writeEntry(writer *tar.Writer, name string, data []byte) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

// pngHeader 用途：限制备份图片为既有 PNG 数据，拒绝任意文本伪装。
// 参数：data 原始图片字节。
// 返回值：PNG 签名是否正确且不超过再编码图片上限。
func pngHeader(data []byte) bool {
	return len(data) >= 8 && len(data) <= 10000000 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
}
