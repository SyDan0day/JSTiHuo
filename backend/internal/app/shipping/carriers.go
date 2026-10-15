package shipping

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
)

type Service struct {
	DB         *sql.DB
	RequestKey []byte
	Now        func() time.Time
}
type CarrierInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	Provider  string `json:"provider"`
	KDniao    string `json:"kdniao_code"`
	Kuaidi100 string `json:"kuaidi100_code"`
	WDT       string `json:"wdt_code"`
	Enabled   bool   `json:"enabled"`
	SecretRef string `json:"secret_ref"`
}
type Carrier struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	CarrierInput
}
type CarrierUpdate struct {
	Version int64 `json:"version"`
	CarrierInput
}
type Filter struct {
	After     string
	Limit     int
	Q, Status string
}
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_after_id,omitempty"`
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
var codePattern = regexp.MustCompile(`^[A-Z0-9_-]{1,40}$`)

// New 用途：装配承运商目录服务。
// 参数：db 业务数据库；requestKey 独立请求 HMAC 密钥。
// 返回值：使用真实时钟的服务，不连接任何外部供应商。
func New(db *sql.DB, requestKey []byte) *Service { return &Service{db, requestKey, time.Now} }

// fault 用途：构造承运商业务错误。
// 参数：status HTTP 语义状态；code 固定业务错误码。
// 返回值：不包含敏感输入的 typed error。
func fault(status int, code string) error { return &identity.Fault{Status: status, Code: code} }

// validate 用途：规范并校验承运商完整目录快照。
// 参数：input 可规范化的编码、名称、平台映射及密钥引用。
// 返回值：nil、字段错误或未配置集成错误，不接受密钥明文或任意引用。
func validate(input *CarrierInput) error {
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	if !codePattern.MatchString(input.Code) || input.Name == "" || utf8.RuneCountInString(input.Name) > 80 || utf8.RuneCountInString(input.Alias) > 80 {
		return fault(422, "VALIDATION_FAILED")
	}
	for _, value := range []string{input.KDniao, input.Kuaidi100, input.WDT} {
		if utf8.RuneCountInString(value) > 40 {
			return fault(422, "VALIDATION_FAILED")
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{0,120}$`).MatchString(input.SecretRef) {
		return fault(422, "VALIDATION_FAILED")
	}
	switch input.Provider {
	case "MANUAL", "MOCK":
		if input.SecretRef != "" {
			return fault(422, "VALIDATION_FAILED")
		}
	case "KDNIAO", "KUAIDI100":
		if input.SecretRef != "" {
			return fault(503, "INTEGRATION_DISABLED")
		}
	default:
		return fault(422, "VALIDATION_FAILED")
	}
	return nil
}

// Get 用途：读取承运商目录的权威快照。
// 参数：ctx 请求上下文；id 承运商标识。
// 返回值：完整目录字段或不存在、数据库错误；secret_ref 仅为引用名。
func (s *Service) Get(ctx context.Context, id string) (Carrier, error) {
	var carrier Carrier
	err := s.DB.QueryRowContext(ctx, "SELECT id,version,code,name,alias,provider,COALESCE(kdniao_code,''),COALESCE(kuaidi100_code,''),COALESCE(wdt_code,''),enabled,COALESCE(secret_ref,'') FROM carriers WHERE id=?", id).Scan(&carrier.ID, &carrier.Version, &carrier.Code, &carrier.Name, &carrier.Alias, &carrier.Provider, &carrier.KDniao, &carrier.Kuaidi100, &carrier.WDT, &carrier.Enabled, &carrier.SecretRef)
	if err == sql.ErrNoRows {
		err = fault(404, "NOT_FOUND")
	}
	return carrier, err
}

// List 用途：分页列出固定白名单条件下的承运商。
// 参数：ctx 请求上下文；filter 已校验的游标、名称编码搜索与启用状态。
// 返回值：承运商页或数据库错误，固定 id DESC，不执行任意排序。
func (s *Service) List(ctx context.Context, filter Filter) (Page[Carrier], error) {
	query := "SELECT id FROM carriers WHERE 1=1"
	args := []any{}
	if filter.After != "" {
		query += " AND id<?"
		args = append(args, filter.After)
	}
	if filter.Q != "" {
		query += " AND (LOCATE(?,code)>0 OR LOCATE(?,name)>0)"
		args = append(args, filter.Q, filter.Q)
	}
	if filter.Status != "" {
		query += " AND enabled=?"
		args = append(args, filter.Status == "ENABLED")
	}
	args = append(args, filter.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return Page[Carrier]{}, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return Page[Carrier]{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Page[Carrier]{}, err
	}
	result := Page[Carrier]{Items: []Carrier{}}
	if len(ids) > filter.Limit {
		ids = ids[:filter.Limit]
		result.Next = ids[len(ids)-1]
	}
	for _, id := range ids {
		carrier, err := s.Get(ctx, id)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, carrier)
	}
	return result, nil
}

// Save 用途：幂等创建或版本化更新承运商目录。
// 参数：ctx 请求上下文；actor 管理员；id 更新对象，空值表示创建。
// 参数：input 完整快照；version 期望版本；key 幂等键；requestID 审计请求标识。
// 返回值：目录快照或权限、字段、版本、唯一冲突及数据库错误。
// 事务：目录、审计及安全重放引用一次提交，不调用供应商。
func (s *Service) Save(
	ctx context.Context, actor identity.Actor, id string, input CarrierInput, version int64, key, requestID string,
) (Carrier, error) {
	if err := validate(&input); err != nil {
		return Carrier{}, err
	}
	if !keyPattern.MatchString(key) || (id != "" && (version < 1 || version > 9007199254740991)) {
		return Carrier{}, fault(422, "VALIDATION_FAILED")
	}
	operation := "createCarrier"
	if id != "" {
		operation = "updateCarrier"
	}
	body, err := json.Marshal(struct {
		ID      string
		Input   CarrierInput
		Version int64
	}{id, input, version})
	if err != nil {
		return Carrier{}, err
	}
	mac := hmac.New(sha256.New, s.RequestKey)
	mac.Write(body)
	target, err := s.saveTx(ctx, actor, id, input, version, operation, key, requestID, mac.Sum(nil))
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		err = fault(409, "CONFLICT")
	}
	if err != nil {
		return Carrier{}, err
	}
	return s.Get(ctx, target)
}

// saveTx 用途：在单个事务中保存承运商及幂等引用。
// 参数：ctx 当前上下文；actor 管理员；id 目录标识；input 完整快照；version 期望版本。
// 参数：operation 内部固定操作；key 幂等键；requestID 请求标识；hash 请求 HMAC。
// 返回值：对象标识或授权、摘要冲突、版本、数据库错误。
func (s *Service) saveTx(
	ctx context.Context, actor identity.Actor, id string, input CarrierInput, version int64,
	operation, key, requestID string, hash []byte,
) (string, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	scope := fmt.Sprintf("admin:%d:%s", actor.AdminID, operation)
	now := s.Now().UTC()
	_, err = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour))
	if err != nil {
		return "", err
	}
	var stored, reference []byte
	var expiry time.Time
	if err = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &reference, &expiry); err != nil {
		return "", err
	}
	if err = identity.AuthorizeAdmin(ctx, tx, actor, "carrier.write"); err != nil {
		return "", err
	}
	if expiry.After(now) {
		if !hmac.Equal(stored, hash) {
			return "", fault(409, "IDEMPOTENCY_CONFLICT")
		}
		var ref struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(reference, &ref); err != nil {
			return "", err
		}
		if ref.ID != "" {
			return ref.ID, tx.Commit()
		}
	}
	target, err := writeCarrier(ctx, tx, id, input, version)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN',?,'carrier',?,?,JSON_OBJECT('version',?,'provider',?,'enabled',?),'SUCCESS')", actor.AdminID, operation, target, requestID, version, input.Provider, input.Enabled)
	if err != nil {
		return "", err
	}
	encoded, _ := json.Marshal(map[string]string{"id": target})
	status := 201
	if id != "" {
		status = 200
	}
	_, err = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=?,expires_at=? WHERE scope=? AND request_key=?", hash, encoded, status, now.Add(24*time.Hour), scope, key)
	if err != nil {
		return "", err
	}
	return target, tx.Commit()
}

// writeCarrier 用途：执行创建或版本比较后的目录写入。
// 参数：ctx 请求上下文；tx 当前事务；id 可空更新标识；input 完整字段；version 期望版本。
// 返回值：对象标识或版本、不存在、SQL 错误，调用方负责提交。
func writeCarrier(ctx context.Context, tx *sql.Tx, id string, input CarrierInput, version int64) (string, error) {
	args := []any{input.Code, input.Name, input.Alias, input.Provider, input.KDniao, input.Kuaidi100, input.WDT, input.Enabled, input.SecretRef}
	if id == "" {
		result, err := tx.ExecContext(ctx, "INSERT INTO carriers(code,name,alias,provider,kdniao_code,kuaidi100_code,wdt_code,enabled,secret_ref) VALUES(?,?,?,?,?,?,?,?,?)", args...)
		if err != nil {
			return "", err
		}
		created, err := result.LastInsertId()
		return fmt.Sprint(created), err
	}
	var actual int64
	if err := tx.QueryRowContext(ctx, "SELECT version FROM carriers WHERE id=? FOR UPDATE", id).Scan(&actual); err != nil {
		if err == sql.ErrNoRows {
			return "", fault(404, "NOT_FOUND")
		}
		return "", err
	}
	if actual != version {
		return "", fault(409, "VERSION_CONFLICT")
	}
	args = append(args, id)
	_, err := tx.ExecContext(ctx, "UPDATE carriers SET code=?,name=?,alias=?,provider=?,kdniao_code=?,kuaidi100_code=?,wdt_code=?,enabled=?,secret_ref=?,version=version+1 WHERE id=?", args...)
	return id, err
}
