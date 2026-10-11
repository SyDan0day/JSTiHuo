package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"strconv"
	"time"
	"unicode/utf8"
)

type Fault struct {
	Status int
	Code   string
}

func (f *Fault) Error() string           { return f.Code }
func fail(status int, code string) error { return &Fault{status, code} }

type Session struct {
	CSRF        string    `json:"csrf_token"`
	Expires     time.Time `json:"expires_at"`
	Permissions []string  `json:"permissions"`
	ForceChange bool      `json:"force_password_change"`
}
type Challenge struct {
	ID      string    `json:"id"`
	Image   string    `json:"image_data_url"`
	Expires time.Time `json:"expires_at"`
}
type Credentials struct {
	Login       string `json:"login_name"`
	Password    string `json:"password"`
	ChallengeID string `json:"challenge_id"`
	Answer      string `json:"challenge_answer"`
}
type LoginResult struct {
	Session Session
	Token   string
}
type Actor struct {
	SessionID, AdminID, AuthVersion int64
	Kind, Hash                      string
	Expires, Seen                   time.Time
	CSRFHash                        []byte
	Force                           bool
	Reauth                          sql.NullTime
	Permissions                     []string
}

func (a Actor) Can(permission string) bool {
	if a.Force {
		return false
	}
	for _, p := range a.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}
func (a Actor) Recent(now time.Time) bool {
	return a.Reauth.Valid && now.Sub(a.Reauth.Time) < 5*time.Minute
}
func token() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), e
}
func digest(s string) []byte { b := sha256.Sum256([]byte(s)); return b[:] }
func mac(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}
func validPassword(s string) bool { n := utf8.RuneCountInString(s); return n >= 12 && n <= 128 }
func id(s string) (int64, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 {
		return 0, fail(422, "INVALID_ARGUMENT")
	}
	return n, nil
}
