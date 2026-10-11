package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"time"
)

// Exact replay is authorized by the old opaque cookie AND CSRF AND request digest.
// No session or credential plaintext is retained in the idempotency record.
func (s *Service) PasswordReplay(ctx context.Context, t, csrf, current, next, key string) (map[string]any, bool, error) {
	var hash, ref []byte
	var expiry time.Time
	e := s.DB.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=?", "password:"+base64.RawStdEncoding.EncodeToString(digest(t)), key).Scan(&hash, &ref, &expiry)
	if e == sql.ErrNoRows {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if !s.Now().Before(expiry) {
		return nil, false, nil
	}
	want := mac(s.RequestKey, string(digest(csrf))+current+"\x00"+next)
	if subtle.ConstantTimeCompare(want, hash) != 1 {
		return nil, true, fail(409, "IDEMPOTENCY_CONFLICT")
	}
	var result struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if e = json.Unmarshal(ref, &result); e != nil {
		return nil, true, e
	}
	return map[string]any{"id": result.ID, "version": result.Version, "state": "ENABLED"}, true, nil
}
