package bootstrap

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	dir := t.TempDir()
	write := func(n, s string) { p := filepath.Join(dir, n); os.WriteFile(p, []byte(s), 0600); t.Setenv(n, p) }
	t.Setenv("DB_DSN_FILE", "")
	if _, e := Load(); e == nil {
		t.Fatal("missing secret accepted")
	}
	write("DB_DSN_FILE", "user:password@tcp(localhost:3306)/jst")
	t.Setenv("APP_ORIGIN", "http://localhost:8080")
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	for _, n := range []string{"LOOKUP_HMAC_KEY_FILE", "REQUEST_HMAC_KEY_FILE", "CHALLENGE_HMAC_KEY_FILE"} {
		write(n, key)
	}
	b, _ := json.Marshal(map[string]any{"active": "1", "keys": map[string]string{"1": key}})
	for _, n := range []string{"PII_KEYRING_FILE", "EXPORT_KEYRING_FILE"} {
		write(n, string(b))
	}
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	write("PII_KEYRING_FILE", `{"active":"2","keys":{}}`)
	if _, e := Load(); e == nil {
		t.Fatal("invalid keyring accepted")
	}
}
