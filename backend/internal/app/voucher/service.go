package voucher

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io"
	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
	"jstihuo/internal/infra/businessid"
	password "jstihuo/internal/infra/crypto"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB         *sql.DB
	Key        []byte
	Root       string
	ExportKey  []byte
	KeyVersion int
	ExportKeys map[int][]byte
	Now        func() time.Time
	Random     io.Reader
	Hash       func(string) (string, error)
}

// New 用途：装配卡券服务。
// 参数：db 业务数据库；root 私有文件根目录。
// 参数：requestKey 独立请求 HMAC 密钥；exportKey active 导出 AES 密钥。
// 参数：version active 导出密钥版本。
// 返回值：服务指针。
// 约束：默认使用真实时钟、密码学随机源及 Argon2id，不连接外部供应商。
func New(db *sql.DB, root string, requestKey, exportKey []byte, version int) *Service {
	return &Service{DB: db, Key: requestKey, Root: root, ExportKey: exportKey, KeyVersion: version, ExportKeys: map[int][]byte{version: exportKey}, Now: time.Now, Random: rand.Reader, Hash: password.Hash}
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
var prefixPattern = regexp.MustCompile(`^[A-Z0-9]{0,4}$`)
var serialPattern = regexp.MustCompile(`^[0-9]{1,24}$`)

// duplicate 用途：识别 MySQL 唯一约束冲突。
// 参数：e 待识别的底层错误。
// 返回值：1062 时为 true。
func duplicate(e error) bool { var d *mysql.MySQLError; return errors.As(e, &d) && d.Number == 1062 }

// mutate 用途：统一执行卡券管理员幂等写入。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：op 内部操作标识；key 当前动作的幂等键。
// 参数：rid 审计请求标识；body 已规范化业务命令。
// 参数：fn 当前事务业务回调。
// 返回值：安全结果引用或权限、复核、摘要冲突、SQL 错误。
// 事务：权限、审计和成功引用与业务同事务；失败整体回滚。
func (s *Service) mutate(ctx context.Context, a identity.Actor, op, key, rid string, body any, fn func(*sql.Tx) (string, error)) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", fault(400, "INVALID_ARGUMENT")
	}
	raw, e := json.Marshal(body)
	if e != nil {
		return "", e
	}
	mac := hmac.New(sha256.New, s.Key)
	mac.Write(raw)
	hash := mac.Sum(nil)
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	now := s.Now().UTC()
	scope := fmt.Sprintf("admin:%d:%s", a.AdminID, op)
	_, e = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour))
	if e != nil {
		return "", e
	}
	var stored, ref []byte
	var expiry time.Time
	if e = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &ref, &expiry); e != nil {
		return "", e
	}
	var enabled, force bool
	var version int64
	if e = tx.QueryRowContext(ctx, "SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR SHARE", a.AdminID).Scan(&enabled, &force, &version); e != nil {
		return "", e
	}
	if !enabled || version != a.AuthVersion {
		return "", fault(401, "AUTH_REQUIRED")
	}
	if force {
		return "", fault(403, "PASSWORD_CHANGE_REQUIRED")
	}
	permission := "voucher.generate"
	switch op {
	case "sealBatch", "sealVoucher":
		permission = "voucher.seal"
	case "resetSecret":
		permission = "voucher.secret.reset"
	case "bulkVouchers", "editVoucher":
		permission = "voucher.write"
	}
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?", a.AdminID, permission).Scan(&count); e != nil {
		return "", e
	}
	if count == 0 {
		return "", fault(403, "FORBIDDEN")
	}
	if op == "generateBatch" || op == "resetSecret" {
		var reauth sql.NullTime
		if e = tx.QueryRowContext(ctx, "SELECT reauth_at FROM sessions WHERE id=? AND admin_id=? AND expires_at>? FOR SHARE", a.SessionID, a.AdminID, now).Scan(&reauth); e != nil {
			if e == sql.ErrNoRows {
				return "", fault(401, "AUTH_REQUIRED")
			}
			return "", e
		}
		if !reauth.Valid || reauth.Time.After(now) || now.Sub(reauth.Time) >= 5*time.Minute {
			return "", fault(403, "REAUTH_REQUIRED")
		}
	}
	var previous struct {
		ID string `json:"id"`
	}
	if expiry.After(now) {
		if !hmac.Equal(stored, hash) {
			return "", fault(409, "IDEMPOTENCY_CONFLICT")
		}
		if e = json.Unmarshal(ref, &previous); e != nil {
			return "", e
		}
		if previous.ID != "" {
			return previous.ID, tx.Commit()
		}
	}
	id, e := fn(tx)
	if e != nil {
		return "", e
	}
	entity := "voucher_batch"
	if op == "createVoucherType" {
		entity = "voucher_type"
	}
	auditID := any(id)
	if op == "bulkVouchers" || op == "resetSecret" || op == "editVoucher" {
		var encoded struct {
			VoucherID string `json:"voucher_id"`
		}
		if op == "resetSecret" {
			json.Unmarshal([]byte(id), &encoded)
			auditID = encoded.VoucherID
		} else {
			auditID = nil
		}
		entity = "voucher"
	}
	if op == "sealVoucher" {
		entity = "voucher"
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	reason := ""
	json.Unmarshal(fields["reason"], &reason)
	auditDiff := map[string]any{"reason": reason}
	if in, ok := body.(BulkInput); ok {
		auditDiff["ids"] = in.IDs
		auditDiff["versions"] = in.Versions
		auditDiff["action"] = in.Action
		if op == "editVoucher" && len(in.IDs) == 1 {
			auditID = in.IDs[0]
		}
	}
	diff, _ := json.Marshal(auditDiff)
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN',?,?,?,?,?,'SUCCESS')", a.AdminID, op, entity, auditID, rid, diff); e != nil {
		return "", e
	}
	out, _ := json.Marshal(map[string]string{"id": id})
	status := 200
	if op == "createBatch" || op == "createVoucherType" || op == "bulkVouchers" {
		status = 201
	}
	if op == "generateBatch" {
		status = 202
	}
	_, e = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=?,expires_at=? WHERE scope=? AND request_key=?", hash, out, status, now.Add(24*time.Hour), scope, key)
	if e != nil {
		return "", e
	}
	return id, tx.Commit()
}

// CreateType 用途：创建不可变卡型版本。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 卡型规则或批次生成配置；key 当前动作的幂等键。
// 参数：rid 审计请求标识。
// 返回值：卡型或校验、冲突、SQL 错误。
// 约束：同名创建串行递增 revision，不修改旧策略。
func (s *Service) CreateType(ctx context.Context, a identity.Actor, in TypeInput, key, rid string) (Type, error) {
	in.Name = strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 80 || !rules.ValidSelection(in.Mode, in.Count, 99) {
		return Type{}, fault(422, "VALIDATION_FAILED")
	}
	id, e := s.mutate(ctx, a, "createVoucherType", key, rid, in, func(tx *sql.Tx) (string, error) {
		// Serialize immutable revisions of the same name, including the first revision.
		var guard int64
		if e := tx.QueryRowContext(ctx, "SELECT id FROM system_configs WHERE config_key='identity_guard' FOR UPDATE").Scan(&guard); e != nil {
			return "", e
		}
		var rev int
		if e := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(revision),0)+1 FROM voucher_types WHERE name=?", in.Name).Scan(&rev); e != nil {
			return "", e
		}
		if rev > 100000 {
			return "", fault(409, "VERSION_CONFLICT")
		}
		r, e := tx.ExecContext(ctx, "INSERT INTO voucher_types(name,revision,selection_mode,selection_count) VALUES(?,?,?,?)", in.Name, rev, in.Mode, in.Count)
		if e != nil {
			return "", e
		}
		n, e := r.LastInsertId()
		return fmt.Sprint(n), e
	})
	if e != nil {
		return Type{}, e
	}
	return s.getType(ctx, s.DB, id)
}

// validate 用途：校验并规范化批次配置。
// 参数：in 卡型规则或批次生成配置。
// 返回值：nil 或配置、商品集合、顺序溢出错误。
// 约束：规范前缀、时间和商品排序，不写数据库。
func validate(in *Input) error {
	in.Label = strings.TrimSpace(in.Label)
	in.Prefix = strings.ToUpper(strings.TrimSpace(in.Prefix))
	in.From = in.From.UTC().Truncate(time.Microsecond)
	in.Until = in.Until.UTC().Truncate(time.Microsecond)
	if !rules.ValidID(in.TypeID) || utf8.RuneCountInString(in.Label) < 1 || utf8.RuneCountInString(in.Label) > 100 || in.Quantity < 1 || in.Quantity > 1000 || !prefixPattern.MatchString(in.Prefix) || in.CodeLength < 12 || in.CodeLength > 24 || !rules.ValidSecret(in.SecretLength, in.SecretFormat) || in.From.IsZero() || in.From.Year() < 1000 || in.Until.Year() > 9999 || !in.Until.After(in.From) || len(in.Products) < 1 || len(in.Products) > 99 {
		return fault(422, "VALIDATION_FAILED")
	}
	seen := map[string]bool{}
	for _, p := range in.Products {
		if !rules.ValidID(p) || seen[p] {
			return fault(422, "VALIDATION_FAILED")
		}
		seen[p] = true
	}
	sort.Slice(in.Products, func(i, j int) bool {
		if len(in.Products[i]) != len(in.Products[j]) {
			return len(in.Products[i]) < len(in.Products[j])
		}
		return in.Products[i] < in.Products[j]
	})
	if in.CodeMode == "RANDOM" {
		if in.Start != "" || in.CodeLength-len(in.Prefix) < 8 {
			return fault(422, "VALIDATION_FAILED")
		}
	} else if in.CodeMode == "SEQUENTIAL" {
		if !serialPattern.MatchString(in.Start) {
			return fault(422, "VALIDATION_FAILED")
		}
		if _, ok := rules.Serial(in.Prefix, in.Start, in.CodeLength, in.Quantity-1); !ok {
			return fault(422, "VALIDATION_FAILED")
		}
	} else {
		return fault(422, "VALIDATION_FAILED")
	}
	return nil
}

// CreateBatch 用途：创建冻结权益草稿。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 卡型规则或批次生成配置；key 当前动作的幂等键。
// 参数：rid 审计请求标识。
// 返回值：批次或规则、对象、SQL 错误。
// 约束：商品存在性检查与策略快照同事务，不生成凭证。
func (s *Service) CreateBatch(ctx context.Context, a identity.Actor, in Input, key, rid string) (Batch, error) {
	in.Products = append([]string(nil), in.Products...)
	if e := validate(&in); e != nil {
		return Batch{}, e
	}
	id, e := s.mutate(ctx, a, "createBatch", key, rid, in, func(tx *sql.Tx) (string, error) {
		t, e := s.getType(ctx, tx, in.TypeID)
		if e != nil {
			return "", e
		}
		if !t.Enabled || !rules.ValidSelection(t.Mode, t.Count, len(in.Products)) {
			return "", fault(422, "SELECTION_INVALID")
		}
		for _, p := range in.Products {
			var n string
			if e = tx.QueryRowContext(ctx, "SELECT id FROM products WHERE id=? AND deleted_at IS NULL FOR SHARE", p).Scan(&n); e != nil {
				if e == sql.ErrNoRows {
					return "", fault(422, "VALIDATION_FAILED")
				}
				return "", e
			}
		}
		policy, _ := json.Marshal(Policy{in.TypeID, t.Revision, t.Mode, t.Count, in.Products})
		var start any
		if in.Start != "" {
			start = in.Start
		}
		for attempt := 0; attempt < 3; attempt++ {
			number, e := businessid.New("B_", s.Now())
			if e != nil {
				return "", e
			}
			r, e := tx.ExecContext(ctx, "INSERT INTO voucher_batches(batch_no,type_id,label,policy_snapshot,quantity,code_prefix,code_length,code_mode,start_serial,secret_length,secret_format,valid_from,valid_until,activate_on_seal,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", number, in.TypeID, in.Label, policy, in.Quantity, in.Prefix, in.CodeLength, in.CodeMode, start, in.SecretLength, in.SecretFormat, in.From, in.Until, in.Activate, a.AdminID)
			if e != nil {
				if duplicate(e) {
					continue
				}
				return "", e
			}
			n, e := r.LastInsertId()
			return fmt.Sprint(n), e
		}
		return "", fault(409, "CONFLICT")
	})
	if e != nil {
		return Batch{}, e
	}
	return s.GetBatch(ctx, id)
}

// Generate 用途：幂等提交或重试生成任务。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；version 期望版本。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：权威批次或权限、复核、版本、SQL 错误。
// 约束：仅 DRAFT、FAILED 可进入 GENERATING，已有结果不重生成。
func (s *Service) Generate(ctx context.Context, a identity.Actor, id string, version int64, key, rid string) (Batch, error) {
	if !rules.ValidID(id) || version < 1 || version > 9007199254740991 {
		return Batch{}, fault(422, "VALIDATION_FAILED")
	}
	result, e := s.mutate(ctx, a, "generateBatch", key, rid, struct {
		ID      string
		Version int64
	}{id, version}, func(tx *sql.Tx) (string, error) {
		var state string
		var v int64
		if e := tx.QueryRowContext(ctx, "SELECT state,version FROM voucher_batches WHERE id=? FOR UPDATE", id).Scan(&state, &v); e != nil {
			if e == sql.ErrNoRows {
				return "", fault(404, "NOT_FOUND")
			}
			return "", e
		}
		if state == "GENERATING" || state == "GENERATED" || state == "SEALED" {
			return id, nil
		}
		if v != version {
			return "", fault(409, "VERSION_CONFLICT")
		}
		if state != "DRAFT" && state != "FAILED" {
			return "", fault(409, "ILLEGAL_TRANSITION")
		}
		_, e := tx.ExecContext(ctx, "UPDATE voucher_batches SET state='GENERATING',lease_until=NULL,last_error_code=NULL,version=version+1 WHERE id=?", id)
		return id, e
	})
	if e != nil {
		return Batch{}, e
	}
	return s.GetBatch(ctx, result)
}
