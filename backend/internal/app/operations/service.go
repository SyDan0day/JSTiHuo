package operations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/voucher"
	crypt "jstihuo/internal/infra/crypto"
	"jstihuo/internal/infra/integrations"
)

type Service struct {
	DB            *sql.DB
	Auth          *identity.Service
	PII           crypt.Keyring
	RequestKey    []byte
	Now           func() time.Time
	Test          bool
	FileRoot      string
	FileKeys      crypt.Keyring
	Orders        *order.Service
	Vouchers      *voucher.Service
	Hash          func(string) (string, error)
	FaultTx       func(context.Context, *sql.Tx, string) error
	Carriers      map[string]integrations.CarrierProvider
	Notifier      integrations.Notifier
	WebhookKeys   map[string][]byte
	trackingMu    sync.Mutex
	trackingAfter string
}

// inject 用途：执行测试装配的事务故障，不通过环境或 API 开放。
// 参数：ctx 上下文；tx 当前事务；step 固定故障边界。
// 返回值：测试故障或 nil；生产不设置钩子。
func (s *Service) inject(ctx context.Context, tx *sql.Tx, step string) error {
	if s.FaultTx != nil {
		return s.FaultTx(ctx, tx, step)
	}
	return nil
}

type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_after_id,omitempty"`
}

type Filter struct {
	After     string
	Limit     int
	Q, Status string
}

type mutation struct {
	Operation, Permission, Entity, Key, RequestID string
	Recent                                        bool
	Status                                        int
	Body                                          any
	Diff                                          map[string]any
	CustomerOrderID                               string
	Target                                        string
	Identity                                      bool
}

var requestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// New 用途：装配运营服务，复用身份、PII 密钥环与请求摘要密钥。
// 参数：db 数据库；auth 身份服务；pii 联系人密钥环；key 请求 HMAC 密钥。
// 返回值：运营服务，不执行数据库写入。
func New(db *sql.DB, auth *identity.Service, pii crypt.Keyring, key []byte) *Service {
	return &Service{DB: db, Auth: auth, PII: pii, RequestKey: key, Now: time.Now, Hash: crypt.Hash,
		Carriers: map[string]integrations.CarrierProvider{"MOCK": integrations.MockCarrier{}}, Notifier: integrations.MockNotifier{}}
}

// fault 用途：构造统一业务错误。
// 参数：status HTTP 状态码；code 稳定错误码。
// 返回值：不含客户信息的错误。
func fault(status int, code string) error { return &identity.Fault{Status: status, Code: code} }

// validText 用途：按 Unicode 码点验证文本长度。
// 参数：value 待校验文本；min 最小码点数；max 最大码点数。
// 返回值：长度是否在范围内。
func validText(value string, min, max int) bool {
	n := utf8.RuneCountInString(value)
	return n >= min && n <= max
}

// write 用途：执行运营写事务并记录可重读的资源引用。
// 参数：ctx 上下文；a 主体；command 固定操作、权限和规范化输入。
// 参数：fn 事务回调，返回脱敏响应和审计对象 ID，不得调用外部服务。
// 返回值：资源 JSON；权限、冲突、业务或数据库错误。
// 事务：业务、幂等结果和无正文审计原子提交；死锁最多重试三次。
func (s *Service) write(ctx context.Context, a identity.Actor, command mutation,
	fn func(*sql.Tx) (any, string, error)) (json.RawMessage, error) {
	if !requestKeyPattern.MatchString(command.Key) {
		return nil, fault(400, "INVALID_ARGUMENT")
	}
	raw, err := json.Marshal(command.Body)
	if err != nil {
		return nil, err
	}
	if command.Target != "" {
		raw, err = json.Marshal(struct {
			ID    string `json:"id"`
			Input any    `json:"input"`
		}{command.Target, command.Body})
		if err != nil {
			return nil, err
		}
	}
	digest := hmac.New(sha256.New, s.RequestKey)
	digest.Write(raw)
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.writeTx(ctx, a, command, digest.Sum(nil), fn)
		if err == nil {
			return result, nil
		}
		var dbError *mysql.MySQLError
		if !errors.As(err, &dbError) || (dbError.Number != 1205 && dbError.Number != 1213) {
			if errors.As(err, &dbError) && dbError.Number == 1062 {
				return nil, fault(409, "VERSION_CONFLICT")
			}
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return nil, fault(503, "TEMPORARY_UNAVAILABLE")
}

// writeTx 用途：在单次事务中复查管理员权限并执行或重放命令。
// 参数：ctx 上下文；a 主体；command 固定命令；hash 输入 HMAC；fn 业务回调。
// 返回值：当前脱敏资源 JSON 或错误。
// 约束：幂等记录不包含请求正文、联系人、凭证或会话令牌。
func (s *Service) writeTx(ctx context.Context, a identity.Actor, command mutation, hash []byte,
	fn func(*sql.Tx) (any, string, error)) (json.RawMessage, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := s.Now().UTC()
	scope := fmt.Sprintf("admin:%d:%s", a.AdminID, command.Operation)
	if a.Kind == "ORDER" {
		scope = fmt.Sprintf("order:%d:%s", a.OrderID, command.Operation)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at)
		VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope`, scope, command.Key, hash, now.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	var stored, response []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE",
		scope, command.Key).Scan(&stored, &response, &expires)
	if err != nil {
		return nil, err
	}
	if command.Identity {
		var guard string
		if err = tx.QueryRowContext(ctx, "SELECT id FROM system_configs WHERE config_key='identity_guard' FOR UPDATE").Scan(&guard); err != nil {
			return nil, err
		}
	}
	if err = s.authorizeMutation(ctx, tx, a, command); err != nil {
		return nil, err
	}
	if command.Recent {
		var reauth sql.NullTime
		err = tx.QueryRowContext(ctx, "SELECT reauth_at FROM sessions WHERE id=? AND expires_at>?", a.SessionID, now).Scan(&reauth)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fault(401, "AUTH_REQUIRED")
			}
			return nil, err
		}
		if !reauth.Valid || reauth.Time.After(now) || now.Sub(reauth.Time) >= 5*time.Minute {
			return nil, fault(403, "REAUTH_REQUIRED")
		}
	}
	if expires.After(now) {
		if !hmac.Equal(stored, hash) {
			return nil, fault(409, "IDEMPOTENCY_CONFLICT")
		}
		var prior struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(response, &prior); err != nil {
			return nil, err
		}
		if prior.ID != "" {
			value, err := s.replayResult(ctx, tx, command.Entity, prior.ID)
			if err != nil {
				return nil, err
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			return encoded, tx.Commit()
		}
	}
	result, id, err := fn(tx)
	if err != nil {
		return nil, err
	}
	response, err = json.Marshal(result)
	if err != nil {
		return nil, err
	}
	diff := command.Diff
	if diff == nil {
		diff = map[string]any{}
	}
	auditData, err := json.Marshal(diff)
	if err != nil {
		return nil, err
	}
	var adminID any
	actorKind := "CUSTOMER"
	if a.Kind == "ADMIN" {
		adminID = a.AdminID
		actorKind = "ADMIN"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result)
		VALUES(?,?,?,?,?,?,?,'SUCCESS')`, adminID, actorKind, command.Operation, command.Entity, id, command.RequestID, auditData)
	if err != nil {
		return nil, err
	}
	status := command.Status
	if status == 0 {
		status = 200
	}
	reference, err := json.Marshal(map[string]string{"id": id})
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=?,expires_at=? WHERE scope=? AND request_key=?",
		hash, reference, status, now.Add(24*time.Hour), scope, command.Key)
	if err != nil {
		return nil, err
	}
	return response, tx.Commit()
}

// authorizeMutation 用途：按固定命令验证管理员权限或本单 WRITE 会话。
// 参数：ctx 上下文；tx 事务；a 主体；command 服务器定义的授权边界。
// 返回值：nil 或身份、作用域、凭证版本错误。
func (s *Service) authorizeMutation(ctx context.Context, tx *sql.Tx, a identity.Actor, command mutation) error {
	if a.Kind == "ADMIN" {
		return identity.AuthorizeAdmin(ctx, tx, a, command.Permission)
	}
	if a.Kind != "ORDER" || command.CustomerOrderID == "" || command.CustomerOrderID != fmt.Sprint(a.OrderID) || s.Orders == nil {
		return fault(403, "SCOPE_REQUIRED")
	}
	return s.Orders.AuthorizeCustomerWrite(ctx, tx, a, command.CustomerOrderID)
}

// replayResult 用途：按已授权幂等引用重读固定资源，避免在幂等表缓存正文。
// 参数：ctx 上下文；tx 当前事务；entity 固定资源种类；id 先前成功对象 ID。
// 返回值：脱敏对象或读取错误，不接受客户端提供任意表名。
func (s *Service) replayResult(ctx context.Context, tx *sql.Tx, entity, id string) (any, error) {
	switch entity {
	case "file":
		file, err := loadPrivate(ctx, tx, id, false)
		return file.File, err
	case "job":
		job, err := loadJob(ctx, tx, id, false)
		return job.Job, err
	case "feedback":
		var out Feedback
		err := tx.QueryRowContext(ctx, "SELECT id,version,message,state,resolution FROM feedback WHERE id=?", id).
			Scan(&out.ID, &out.Version, &out.Message, &out.State, &out.Resolution)
		return out, err
	case "after_sale":
		return loadCase(ctx, tx, id, false)
	case "notification":
		return loadNotification(ctx, tx, id, false)
	case "admin":
		return loadManagedAdmin(ctx, tx, id, false)
	case "role":
		return loadRole(ctx, tx, id, false)
	case "config":
		return loadConfig(ctx, tx)
	case "shipment":
		var out order.Shipment
		err := tx.QueryRowContext(ctx, "SELECT id,order_id,carrier_id,tracking_no,kind,status,shipped_at FROM shipments WHERE id=?", id).Scan(&out.ID, &out.OrderID, &out.CarrierID, &out.Tracking, &out.Kind, &out.Status, &out.Shipped)
		return out, err
	default:
		return nil, fault(500, "INVALID_RESULT_REFERENCE")
	}
}
