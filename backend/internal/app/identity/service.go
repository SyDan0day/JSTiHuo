package identity

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	crypt "jstihuo/internal/infra/crypto"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB                                  *sql.DB
	Now                                 func() time.Time
	LookupKey, ChallengeKey, RequestKey []byte
	dummy                               string
}

func New(db *sql.DB, lookup, challenge, request []byte) (*Service, error) {
	hash, e := crypt.Hash("dummy-credential-not-an-account")
	return &Service{db, func() time.Time { return time.Now().UTC() }, lookup, challenge, request, hash}, e
}
func (s *Service) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) Bootstrap(ctx context.Context) (LoginResult, error) {
	t, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	c, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	exp := s.Now().Add(10 * time.Minute)
	e = s.transaction(ctx, func(tx *sql.Tx) error { return insertSession(ctx, tx, t, c, "PREAUTH", 0, 0, exp, s.Now(), false) })
	return LoginResult{Session{c, exp, []string{}, false}, t}, e
}
func (s *Service) Authenticate(ctx context.Context, t, kind string) (Actor, error) {
	a, e := loadActor(ctx, s.DB, digest(t), kind)
	if e != nil {
		return Actor{}, fail(401, "AUTH_REQUIRED")
	}
	now := s.Now()
	if !now.Before(a.Expires) || (kind == "ADMIN" && now.Sub(a.Seen) >= 30*time.Minute) {
		return Actor{}, fail(401, "AUTH_REQUIRED")
	}
	if kind == "ADMIN" {
		if e = checkAdmin(ctx, s.DB, &a); e != nil {
			return Actor{}, fail(401, "AUTH_REQUIRED")
		}
	}
	return a, nil
}
func (s *Service) CheckCSRF(a Actor, csrf string) bool {
	return len(csrf) >= 16 && subtle.ConstantTimeCompare(a.CSRFHash, digest(csrf)) == 1
}
func (s *Service) limit(ctx context.Context, label string, max int, window time.Duration) error {
	now := s.Now()
	end := now.Truncate(window).Add(window)
	key := mac(s.LookupKey, label+end.Format(time.RFC3339))
	n, e := countLimit(ctx, s.DB, key, end)
	if e != nil {
		return e
	}
	if n > max {
		return fail(429, "RATE_LIMITED")
	}
	return nil
}
func (s *Service) Challenge(ctx context.Context, a Actor, purpose, ip string) (Challenge, error) {
	if purpose != "LOGIN" && purpose != "VOUCHER" && purpose != "ORDER_QUERY" && purpose != "FEEDBACK" {
		return Challenge{}, fail(422, "INVALID_ARGUMENT")
	}
	if e := s.limit(ctx, "challenge:"+ip, 20, 10*time.Minute); e != nil {
		return Challenge{}, e
	}
	answer, png, e := captcha()
	if e != nil {
		return Challenge{}, e
	}
	exp := s.Now().Add(5 * time.Minute)
	n, e := insertChallenge(ctx, s.DB, a.SessionID, purpose, mac(s.ChallengeKey, answer), exp)
	return Challenge{fmt.Sprint(n), png, exp}, e
}
func (s *Service) consumeChallenge(ctx context.Context, a Actor, challengeID, answer string) error {
	n, e := id(challengeID)
	if e != nil {
		return e
	}
	valid := false
	e = s.transaction(ctx, func(tx *sql.Tx) error {
		row, e := lockChallenge(ctx, tx, n, a.SessionID)
		if e != nil {
			return fail(422, "CHALLENGE_INVALID")
		}
		if row.Purpose != "LOGIN" || row.Consumed.Valid || row.Attempts >= 5 || !s.Now().Before(row.Expires) {
			return fail(422, "CHALLENGE_INVALID")
		}
		valid = subtle.ConstantTimeCompare(row.Answer, mac(s.ChallengeKey, strings.ToUpper(answer))) == 1
		return updateChallenge(ctx, tx, n, valid, s.Now())
	})
	if e != nil {
		return e
	}
	if !valid {
		return fail(422, "CHALLENGE_INVALID")
	}
	return nil
}
func (s *Service) Login(ctx context.Context, a Actor, c Credentials, ip, requestID string) (LoginResult, error) {
	if utf8.RuneCountInString(c.Login) < 3 || utf8.RuneCountInString(c.Login) > 64 || !validPassword(c.Password) || len(c.Answer) < 4 || len(c.Answer) > 8 {
		return LoginResult{}, fail(422, "INVALID_ARGUMENT")
	}
	if e := s.limit(ctx, "login-ip:"+ip, 30, 15*time.Minute); e != nil {
		return LoginResult{}, e
	}
	if e := s.consumeChallenge(ctx, a, c.ChallengeID, c.Answer); e != nil {
		return LoginResult{}, e
	}
	u, e := findAdmin(ctx, s.DB, c.Login)
	hash := s.dummy
	if e == nil {
		hash = u.Password
	}
	ok := crypt.Verify(c.Password, hash)
	if !ok || e != nil || !u.Enabled {
		if e = s.limit(ctx, "login-account:"+c.Login, 5, 15*time.Minute); e != nil {
			return LoginResult{}, e
		}
		if e = auditFailure(ctx, s.DB, requestID, "admin.login"); e != nil {
			return LoginResult{}, e
		}
		return LoginResult{}, fail(401, "INVALID_CREDENTIALS")
	}
	if e = checkLimit(ctx, s.DB, mac(s.LookupKey, "login-account:"+c.Login+s.Now().Truncate(15*time.Minute).Add(15*time.Minute).Format(time.RFC3339)), 5); e != nil {
		return LoginResult{}, e
	}
	t, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	csrf, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	exp := s.Now().Add(8 * time.Hour)
	permissions := []string{}
	e = s.transaction(ctx, func(tx *sql.Tx) error {
		current, e := lockAdmin(ctx, tx, u.ID)
		if e != nil || !current.Enabled || current.AuthVersion != u.AuthVersion || current.Password != hash {
			return fail(401, "INVALID_CREDENTIALS")
		}
		if e = deleteSession(ctx, tx, a.SessionID); e != nil {
			return e
		}
		if e = insertSession(ctx, tx, t, csrf, "ADMIN", u.ID, u.AuthVersion, exp, s.Now(), false); e != nil {
			return e
		}
		permissions, e = loadPermissions(ctx, tx, u.ID)
		if e != nil {
			return e
		}
		return audit(ctx, tx, u.ID, requestID, "admin.login")
	})
	if u.Force {
		permissions = []string{}
	}
	return LoginResult{Session{csrf, exp, permissions, u.Force}, t}, e
}
func (s *Service) ReadSession(ctx context.Context, a Actor) (Session, error) {
	csrf, e := token()
	if e != nil {
		return Session{}, e
	}
	e = refreshCSRF(ctx, s.DB, a.SessionID, digest(csrf), s.Now())
	p := a.Permissions
	if a.Force {
		p = []string{}
	}
	return Session{csrf, a.Expires, p, a.Force}, e
}
func (s *Service) Logout(ctx context.Context, a Actor, requestID string) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		if e := deleteSession(ctx, tx, a.SessionID); e != nil {
			return e
		}
		return audit(ctx, tx, a.AdminID, requestID, "admin.logout")
	})
}
func (s *Service) Reauth(ctx context.Context, a Actor, password, ip, requestID string) (LoginResult, error) {
	if a.Force {
		return LoginResult{}, fail(403, "PASSWORD_CHANGE_REQUIRED")
	}
	if !validPassword(password) {
		return LoginResult{}, fail(422, "INVALID_ARGUMENT")
	}
	if e := s.limit(ctx, "reauth:"+fmt.Sprint(a.AdminID)+":"+ip, 5, 15*time.Minute); e != nil {
		return LoginResult{}, e
	}
	u, e := findAdminID(ctx, s.DB, a.AdminID)
	if e != nil || !crypt.Verify(password, u.Password) {
		return LoginResult{}, fail(401, "INVALID_CREDENTIALS")
	}
	t, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	c, e := token()
	if e != nil {
		return LoginResult{}, e
	}
	e = s.transaction(ctx, func(tx *sql.Tx) error {
		v, e := lockAdmin(ctx, tx, a.AdminID)
		if e != nil || v.AuthVersion != a.AuthVersion || !v.Enabled {
			return fail(401, "AUTH_REQUIRED")
		}
		if e = deleteSession(ctx, tx, a.SessionID); e != nil {
			return e
		}
		if e = insertSession(ctx, tx, t, c, "ADMIN", a.AdminID, a.AuthVersion, a.Expires, s.Now(), true); e != nil {
			return e
		}
		return audit(ctx, tx, a.AdminID, requestID, "admin.reauth")
	})
	return LoginResult{Session{c, a.Expires, a.Permissions, false}, t}, e
}
func (s *Service) ChangePassword(ctx context.Context, a Actor, current, next, key, requestID string) (map[string]any, error) {
	if !validPassword(current) || !validPassword(next) || current == next {
		return nil, fail(422, "INVALID_ARGUMENT")
	}
	if e := s.limit(ctx, "password:"+fmt.Sprint(a.AdminID), 5, 15*time.Minute); e != nil {
		return nil, e
	}
	u, e := findAdminID(ctx, s.DB, a.AdminID)
	if e != nil || !crypt.Verify(current, u.Password) {
		return nil, fail(401, "INVALID_CREDENTIALS")
	}
	hash, e := crypt.Hash(next)
	if e != nil {
		return nil, e
	}
	var version int64
	e = s.transaction(ctx, func(tx *sql.Tx) error {
		v, e := lockAdmin(ctx, tx, a.AdminID)
		if e != nil || v.AuthVersion != a.AuthVersion || !v.Enabled {
			return fail(401, "AUTH_REQUIRED")
		}
		version = v.Version + 1
		if e = recordPasswordKey(ctx, tx, a, key, mac(s.RequestKey, string(a.CSRFHash)+current+"\x00"+next), version, s.Now()); e != nil {
			return e
		}
		if e = changePassword(ctx, tx, a.AdminID, hash); e != nil {
			return e
		}
		if e = revokeAdmin(ctx, tx, a.AdminID); e != nil {
			return e
		}
		return audit(ctx, tx, a.AdminID, requestID, "admin.password_changed")
	})
	return map[string]any{"id": fmt.Sprint(a.AdminID), "version": version, "state": "ENABLED"}, e
}

func (s *Service) AuditDenied(ctx context.Context, requestID string) error {
	return auditFailure(ctx, s.DB, requestID, "identity.request_denied")
}
