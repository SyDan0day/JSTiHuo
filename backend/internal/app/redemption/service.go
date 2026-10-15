package redemption

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/domain/calendar"
	"jstihuo/internal/domain/regions"
	rules "jstihuo/internal/domain/voucher"
	"jstihuo/internal/infra/businessid"
	crypt "jstihuo/internal/infra/crypto"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB                    *sql.DB
	Auth                  *identity.Service
	Catalog               *catalog.Service
	Regions               regions.Dataset
	PII                   crypt.Keyring
	RequestKey, LookupKey []byte
	Now                   func() time.Time
	Test                  bool
	Dummy                 string
	Fault                 func(string) error
	FaultTx               func(context.Context, *sql.Tx, string) error
	// Number 为可测试业务号随机依赖；生产始终使用密码学随机 ULID。
	Number func(time.Time) (string, error)
}

// New 用途：装配兑换服务并读取锁定版本的地区数据。
// 参数：db 业务数据库；auth 身份服务依赖。
// 参数：cat 商品服务依赖；pii PII 版本密钥环。
// 参数：requestKey 独立请求 HMAC 密钥；lookupKey 独立手机查找 HMAC 密钥。
// 返回值：服务或地区加载、随机哈希初始化错误。
// 约束：不写业务数据。
func New(db *sql.DB, auth *identity.Service, cat *catalog.Service, pii crypt.Keyring, requestKey, lookupKey []byte) (*Service, error) {
	d, e := regions.Load()
	if e != nil {
		return nil, e
	}
	dummy, e := crypt.Hash("dummy-voucher-credential")
	if e != nil {
		return nil, e
	}
	return &Service{DB: db, Auth: auth, Catalog: cat, Regions: d, PII: pii, RequestKey: requestKey, LookupKey: lookupKey, Now: time.Now, Dummy: dummy, Number: func(now time.Time) (string, error) { return businessid.New("O_", now) }}, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// card 用途：读取卡券及冻结策略。
// 参数：ctx 请求上下文；q 数据库或当前事务查询接口。
// 参数：id code 为空时使用的卡券标识；code 规范化卡号，非空时优先于 id。
// 参数：lock 是否在调用方事务中锁读。
// 返回值：卡券快照或查询、JSON 错误。
// 约束：lock 只能在已按批次共享锁、卡券排他锁顺序的事务内使用。
func (s *Service) card(ctx context.Context, q queryer, id, code string, lock bool) (Card, error) {
	var c Card
	var raw []byte
	where := "v.id=?"
	value := id
	if code != "" {
		where = "v.code=?"
		value = code
	}
	query := "SELECT v.id,v.batch_id,v.secret_hash,v.version,v.credential_version,v.active,v.redeemed_at IS NOT NULL,v.deleted_at IS NOT NULL,v.credential_state,v.valid_from,v.valid_until,b.state,b.policy_snapshot FROM vouchers v JOIN voucher_batches b ON b.id=v.batch_id WHERE " + where
	if lock {
		query = "SELECT v.id,v.batch_id,v.secret_hash,v.version,v.credential_version,v.active,v.redeemed_at IS NOT NULL,v.deleted_at IS NOT NULL,v.credential_state,v.valid_from,v.valid_until,(SELECT state FROM voucher_batches WHERE id=v.batch_id),(SELECT policy_snapshot FROM voucher_batches WHERE id=v.batch_id) FROM vouchers v WHERE " + where + " FOR UPDATE"
	}
	e := q.QueryRowContext(ctx, query, value).Scan(&c.ID, &c.BatchID, &c.Hash, &c.Version, &c.CredentialVersion, &c.Active, &c.Redeemed, &c.Deleted, &c.State, &c.From, &c.Until, &c.BatchState, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &c.Policy)
	}
	return c, e
}

// eligible 用途：判断卡券当前是否可兑换。
// 参数：c 权威卡券快照。
// 返回值：满足未消费、封存、激活和有效期条件时为 true。
// 约束：不修改状态。
func (s *Service) eligible(c Card) bool {
	return !c.Deleted && c.Active && !c.Redeemed && c.State == "SEALED" && c.BatchState == "SEALED" && rules.ValidAt(s.Now(), c.From, c.Until)
}

// products 用途：投影卡券权益商品的公开信息。
// 参数：ctx 请求上下文；c 冻结卡券权益。
// 参数：availableOnly 是否只保留当前可兑换商品。
// 返回值：商品列表或读取错误。
// 约束：无库存变动。
func (s *Service) products(ctx context.Context, c Card, availableOnly bool) ([]catalog.Public, error) {
	out := []catalog.Public{}
	for _, id := range c.Policy.Products {
		p, e := s.Catalog.Get(ctx, id, false)
		if e != nil {
			return nil, e
		}
		if availableOnly && (p.Status != "ENABLED" || (p.From != nil && s.Now().Before(*p.From)) || (p.Until != nil && !s.Now().Before(*p.Until))) {
			continue
		}
		images := []string{}
		for _, id := range p.MediaIDs {
			images = append(images, "/media/"+id)
		}
		out = append(out, catalog.Public{ID: p.ID, Name: p.Name, Attributes: p.Attributes, Description: p.Description, Images: images})
	}
	return out, nil
}

// Verify 用途：鉴真并原子轮换 PREAUTH 为受限卡券会话。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 待校验业务命令；ip 受控请求来源地址。
// 参数：rid 审计请求标识。
// 返回值：状态、CSRF 和内部会话结果或校验、挑战、限流、数据库错误。
// 事务：锁内复查凭证版本并轮换 PREAUTH；不核销或预占库存。
func (s *Service) Verify(ctx context.Context, a identity.Actor, in Credentials, ip, rid string) (VerifyResult, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if !regexp.MustCompile(`^[A-Z0-9]{6,32}$`).MatchString(in.Code) || len(in.Secret) < 6 || len(in.Secret) > 32 || !rules.ValidID(in.ChallengeID) || len(in.Answer) < 4 || len(in.Answer) > 8 {
		return VerifyResult{}, fault(422, "VALIDATION_FAILED")
	}
	if a.Kind != "PREAUTH" {
		return VerifyResult{}, fault(403, "SCOPE_REQUIRED")
	}
	if e := s.Auth.Limit(ctx, "voucher-ip:"+ip, 60, 15*time.Minute); e != nil {
		return VerifyResult{}, e
	}
	if e := s.Auth.Limit(ctx, "voucher-code:"+in.Code, 10, 15*time.Minute); e != nil {
		return VerifyResult{}, e
	}
	if e := s.Auth.ConsumeChallenge(ctx, a, in.ChallengeID, in.Answer, "VOUCHER"); e != nil {
		return VerifyResult{}, e
	}
	c, e := s.card(ctx, s.DB, "", in.Code, false)
	hash := s.Dummy
	if e == nil {
		hash = c.Hash
	}
	valid := crypt.Verify(in.Secret, hash)
	if e != nil && e != sql.ErrNoRows {
		return VerifyResult{}, e
	}
	if !valid || e != nil || c.Deleted {
		return VerifyResult{}, fault(401, "INVALID_CREDENTIALS")
	}
	products, e := s.products(ctx, c, false)
	if e != nil {
		return VerifyResult{}, e
	}
	session, e := s.Auth.PrepareCustomer("VOUCHER")
	if e != nil {
		return VerifyResult{}, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return VerifyResult{}, e
	}
	defer tx.Rollback()
	var state string
	if e = tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR SHARE", c.BatchID).Scan(&state); e != nil {
		return VerifyResult{}, e
	}
	current, e := s.card(ctx, tx, c.ID, "", true)
	if e != nil {
		return VerifyResult{}, e
	}
	if current.Hash != hash || current.CredentialVersion != c.CredentialVersion || current.Deleted {
		return VerifyResult{}, fault(401, "INVALID_CREDENTIALS")
	}
	scope := "READ"
	if s.eligible(current) {
		scope = "WRITE"
	}
	var previousExpiry time.Time
	if e = tx.QueryRowContext(ctx, "SELECT expires_at FROM sessions WHERE id=? AND kind='PREAUTH' FOR UPDATE", a.SessionID).Scan(&previousExpiry); e != nil || !s.Now().Before(previousExpiry) {
		return VerifyResult{}, fault(401, "AUTH_REQUIRED")
	}
	id, e := parseID(c.ID)
	if e != nil {
		return VerifyResult{}, e
	}
	if e = s.Auth.InsertCustomer(ctx, tx, session, &a, "VOUCHER", scope, id, 0, c.CredentialVersion); e != nil {
		return VerifyResult{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('CUSTOMER','voucher.verified','voucher',?,?,JSON_OBJECT('scope',?),'SUCCESS')", c.ID, rid, scope); e != nil {
		return VerifyResult{}, e
	}
	if e = tx.Commit(); e != nil {
		return VerifyResult{}, e
	}
	return VerifyResult{Verification{true, current.Active, current.Redeemed, current.From, current.Until, current.Policy.Mode, current.Policy.Count, products, session.Session.CSRF, session.Session.Expires}, session}, nil
}

// parseID 用途：把十进制对象标识转换为 int64。
// 参数：id 目标对象标识。
// 返回值：正整数标识或格式、溢出错误。
func parseID(id string) (int64, error) {
	var n int64
	if !rules.ValidID(id) {
		return 0, fault(422, "INVALID_ARGUMENT")
	}
	_, e := fmt.Sscan(id, &n)
	return n, e
}

// Options 用途：读取当前可兑换商品与冻结选品规则。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 返回值：选品信息或作用域、卡券状态、集合不足、查询错误。
// 约束：不核销卡券。
func (s *Service) Options(ctx context.Context, a identity.Actor) (Options, error) {
	if a.Kind != "VOUCHER" {
		return Options{}, fault(403, "SCOPE_REQUIRED")
	}
	c, e := s.card(ctx, s.DB, fmt.Sprint(a.VoucherID), "", false)
	if e != nil {
		return Options{}, e
	}
	if !s.eligible(c) || c.CredentialVersion != a.AuthVersion || a.Scope != "WRITE" {
		return Options{}, fault(422, "VOUCHER_UNAVAILABLE")
	}
	products, e := s.products(ctx, c, true)
	if e != nil {
		return Options{}, e
	}
	if len(products) < c.Policy.Count {
		return Options{}, fault(422, "SELECTION_INVALID")
	}
	return Options{"selection-v1", c.Policy.Mode, c.Policy.Count, products}, nil
}

// Configuration 用途：读取预约日历配置，缺失时采用既定默认。
// 参数：ctx 请求上下文。
// 返回值：有效配置或数据库、配置不可用错误。
// 约束：不覆盖已有配置。
func (s *Service) Configuration(ctx context.Context) (calendar.Config, error) {
	return ReadConfiguration(ctx, s.DB)
}

// ReadConfiguration 用途：从数据库或事务读取当前日历规则，不取得配置行锁。
// 参数：ctx 上下文；q 支持单行查询的数据源。
// 返回值：有效配置或读取、配置不可用错误；缺失使用默认值。
// 并发：业务锁定日历后使用 READ COMMITTED 重读，避免反向取得配置锁。
func ReadConfiguration(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (calendar.Config, error) {
	c := calendar.Default()
	var raw []byte
	e := q.QueryRowContext(ctx, "SELECT value FROM system_configs WHERE config_key='shipping_calendar'").Scan(&raw)
	if e == sql.ErrNoRows {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil || !c.Valid() {
		return c, fault(503, "CONFIGURATION_UNAVAILABLE")
	}
	return c, nil
}

// Calendar 用途：列出上海业务日期窗口内的预约建议。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 返回值：日期、余量和可用标记或授权、查询错误。
// 约束：只补建缺失日历行，不占名额或库存。
func (s *Service) Calendar(ctx context.Context, a identity.Actor) (Calendar, error) {
	if _, e := s.Options(ctx, a); e != nil {
		return Calendar{}, e
	}
	return s.CalendarDays(ctx, "")
}

// CalendarDays 用途：读取并补建配置窗口内的预约建议。
// 参数：ctx 请求上下文；ownedDate 既有订单占用日期。
// 返回值：建议日历或配置、SQL 错误。
// 约束：ownedDate 在额度已满时仍保留该订单自己的名额提示，函数不占库存或额度，调用方负责授权。
func (s *Service) CalendarDays(ctx context.Context, ownedDate string) (Calendar, error) {
	config, e := s.Configuration(ctx)
	if e != nil {
		return Calendar{}, e
	}
	now := s.Now()
	today := calendar.Day(now)
	out := Calendar{Items: []Day{}}
	// Preparing future rows changes no used slots, cards, orders, or stock.
	for day := 0; day <= config.Horizon; day++ {
		date := today.AddDate(0, 0, day).Format("2006-01-02")
		if _, e = s.DB.ExecContext(ctx, "INSERT INTO shipping_calendars(ship_date,capacity) VALUES(?,?) ON DUPLICATE KEY UPDATE ship_date=ship_date", date, config.Capacity); e != nil {
			return out, e
		}
		var capacity, used int
		var closed bool
		if e = s.DB.QueryRowContext(ctx, "SELECT capacity,used_slots,closed FROM shipping_calendars WHERE ship_date=?", date).Scan(&capacity, &used, &closed); e != nil {
			return out, e
		}
		remaining := capacity - used
		out.Items = append(out.Items, Day{date, remaining, date == ownedDate || (config.Allowed(now, date) && !closed && remaining > 0)})
	}
	return out, nil
}

// ValidateAddress 用途：规范化并验证收货信息和地区父子关系。
// 参数：a 已认证操作主体。
// 返回值：nil 或字段校验错误。
// 约束：只去除姓名、手机、详细地址首尾空白。
func (s *Service) ValidateAddress(a *Address) error {
	a.Name = strings.TrimSpace(a.Name)
	a.Phone = strings.TrimSpace(a.Phone)
	a.Detail = strings.TrimSpace(a.Detail)
	if utf8.RuneCountInString(a.Name) < 1 || utf8.RuneCountInString(a.Name) > 40 || utf8.RuneCountInString(a.Detail) < 5 || utf8.RuneCountInString(a.Detail) > 200 || (!regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(a.Phone) && !(s.Test && a.Phone == "10000000000")) || !s.Regions.Validate(a.Province, a.City, a.District) {
		return fault(422, "VALIDATION_FAILED")
	}
	return nil
}
