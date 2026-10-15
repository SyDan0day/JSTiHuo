package bootstrap

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	DSN, Address, Origin string
	ProxyNetworks        []netip.Prefix
}

func ReadSecret(name string) ([]byte, error) {
	p := os.Getenv(name)
	if p == "" {
		return nil, errors.New(name + " is required")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return nil, errors.New(name + " cannot be read")
	}
	if len(b) == 0 {
		return nil, errors.New(name + " is empty")
	}
	return b, nil
}

// Load 用途：验证数据库、独立密钥、站点 origin 与可信代理网络配置。
// 参数：无，从明确的部署环境变量与秘密文件读取。
// 返回值：有效配置或不含秘密内容的启动错误。
func Load() (Config, error) {
	var c Config
	if raw := os.Getenv("TRUSTED_PROXY_CIDRS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
			if err != nil {
				return c, errors.New("TRUSTED_PROXY_CIDRS requires explicit CIDR networks")
			}
			c.ProxyNetworks = append(c.ProxyNetworks, prefix.Masked())
		}
	}
	b, e := ReadSecret("DB_DSN_FILE")
	if e != nil {
		return c, e
	}
	c.DSN = strings.TrimSpace(string(b))
	c.Address = os.Getenv("HTTP_ADDR")
	if c.Address == "" {
		c.Address = ":8080"
	}
	c.Origin = os.Getenv("APP_ORIGIN")
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return c, errors.New("APP_ORIGIN must be an absolute HTTP origin")
	}
	if os.Getenv("APP_ENV") != "development" && u.Scheme != "https" {
		return c, errors.New("only development permits HTTP origins")
	}
	for _, n := range []string{"LOOKUP_HMAC_KEY_FILE", "REQUEST_HMAC_KEY_FILE", "CHALLENGE_HMAC_KEY_FILE"} {
		b, e = ReadSecret(n)
		if e != nil {
			return c, e
		}
		k, e := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if e != nil || len(k) != 32 {
			return c, errors.New(n + " requires a base64 32-byte key")
		}
	}
	for _, n := range []string{"PII_KEYRING_FILE", "EXPORT_KEYRING_FILE"} {
		b, e = ReadSecret(n)
		if e != nil {
			return c, e
		}
		var ring struct {
			Active string            `json:"active"`
			Keys   map[string]string `json:"keys"`
		}
		if json.Unmarshal(b, &ring) != nil || ring.Keys[ring.Active] == "" {
			return c, errors.New(n + " invalid keyring")
		}
		for _, v := range ring.Keys {
			k, e := base64.StdEncoding.DecodeString(v)
			if e != nil || len(k) != 32 {
				return c, errors.New(n + " invalid AES key")
			}
		}
	}
	return c, nil
}
