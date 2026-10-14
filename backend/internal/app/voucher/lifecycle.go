package voucher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
	"sort"
	"time"
	"unicode/utf8"
)

type VersionReason struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}
type ResetInput struct {
	VersionReason
	Length int    `json:"secret_length"`
	Format string `json:"secret_format"`
}
type Delivery struct {
	VoucherID string    `json:"voucher_id"`
	FileID    string    `json:"export_file_id"`
	Expires   time.Time `json:"expires_at"`
	State     string    `json:"credential_state"`
}
type BulkInput struct {
	IDs      []string   `json:"ids"`
	Versions []int64    `json:"versions"`
	Action   string     `json:"action"`
	Reason   string     `json:"reason"`
	From     *time.Time `json:"valid_from,omitempty"`
	Until    *time.Time `json:"valid_until,omitempty"`
}
type Action struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	State   string `json:"state"`
}
type BulkResult struct {
	Items []Action `json:"items"`
}

// validVersion 用途：检查版本与原因长度。
// 参数：v 待校验的版本与原因。
// 返回值：满足版本与长度边界时为 true。
func validVersion(v VersionReason) bool {
	return v.Version >= 1 && v.Version <= 9007199254740991 && utf8.RuneCountInString(v.Reason) <= 200
}

// revoke 用途：撤销对应卡券及订单身份会话。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识。
// 返回值：nil 或 SQL 错误。
// 约束：先删除挑战再删会话，保留 RESTRICT 外键约束，不自行提交。
func revoke(ctx context.Context, tx *sql.Tx, id string) error {
	// Delete dependent challenges before sessions (foreign keys are RESTRICT).
	where := "(s.voucher_id=? OR s.order_id IN (SELECT o.id FROM orders o JOIN voucher_redemptions r ON r.id=o.redemption_id WHERE r.voucher_id=?))"
	if _, e := tx.ExecContext(ctx, "DELETE c FROM challenges c JOIN sessions s ON s.id=c.session_id WHERE "+where, id, id); e != nil {
		return e
	}
	_, e := tx.ExecContext(ctx, "DELETE s FROM sessions s WHERE "+where, id, id)
	return e
}

// expireFile 用途：撤销文件下载并持久化删除任务。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识；now 当前时刻。
// 返回值：nil 或 SQL 错误。
// 约束：锁文件、标记 EXPIRED、幂等入 Outbox，物理删除另行重试。
func expireFile(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if id == "" {
		return nil
	}
	var key, state string
	if e := tx.QueryRowContext(ctx, "SELECT storage_key,state FROM files WHERE id=? FOR UPDATE", id).Scan(&key, &state); e != nil {
		return e
	}
	if state == "DELETED" {
		return nil
	}
	if _, e := tx.ExecContext(ctx, "UPDATE files SET state='EXPIRED',version=version+1 WHERE id=? AND state<>'EXPIRED'", id); e != nil {
		return e
	}
	// Only object IDs enter durable task payloads. Deletion is authorized by EXPIRED.
	_, e := tx.ExecContext(ctx, "INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,'file.destroy',JSON_OBJECT('file_id',?),?) ON DUPLICATE KEY UPDATE event_key=event_key", "file.destroy:"+id, id, now.UTC())
	return e
}

// lockVoucher 用途：按批次再卡券顺序锁定发行对象。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识。
// 返回值：卡券、文件标识、凭证版本或不存在、SQL 错误。
// 约束：软删除对象不可用于新发行操作。
func (s *Service) lockVoucher(ctx context.Context, tx *sql.Tx, id string) (Voucher, string, int, error) {
	var batchID string
	if e := tx.QueryRowContext(ctx, "SELECT batch_id FROM vouchers WHERE id=?", id).Scan(&batchID); e != nil {
		if e == sql.ErrNoRows {
			e = fault(404, "NOT_FOUND")
		}
		return Voucher{}, "", 0, e
	}
	var batchState string
	if e := tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", batchID).Scan(&batchState); e != nil {
		return Voucher{}, "", 0, e
	}
	var v Voucher
	var file sql.NullString
	var cred int
	var deleted sql.NullTime
	e := tx.QueryRowContext(ctx, "SELECT id,version,batch_id,code,active,credential_state,valid_from,valid_until,redeemed_at IS NOT NULL,credential_file_id,credential_version,deleted_at FROM vouchers WHERE id=? FOR UPDATE", id).Scan(&v.ID, &v.Version, &v.BatchID, &v.Code, &v.Active, &v.State, &v.From, &v.Until, &v.Redeemed, &file, &cred, &deleted)
	if e == nil && deleted.Valid {
		e = fault(404, "NOT_FOUND")
	}
	return v, file.String, cred, e
}

// sealBatchTx 用途：在已有事务内封存整批。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识；version 期望版本。
// 返回值：nil 或版本、状态、SQL 错误。
// 约束：关闭导出并按配置激活可用未消费卡，不自行提交。
func (s *Service) sealBatchTx(ctx context.Context, tx *sql.Tx, id string, version int64) error {
	var state string
	var actual int64
	var file sql.NullString
	var activate bool
	if e := tx.QueryRowContext(ctx, "SELECT state,version,export_file_id,activate_on_seal FROM voucher_batches WHERE id=? FOR UPDATE", id).Scan(&state, &actual, &file, &activate); e != nil {
		if e == sql.ErrNoRows {
			return fault(404, "NOT_FOUND")
		}
		return e
	}
	if state == "SEALED" {
		return nil
	}
	if actual != version {
		return fault(409, "VERSION_CONFLICT")
	}
	if state != "GENERATED" {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	// Lock vouchers before their file so reset and download use the same order.
	rows, e := tx.QueryContext(ctx, "SELECT id FROM vouchers WHERE batch_id=? ORDER BY id FOR UPDATE", id)
	if e != nil {
		return e
	}
	for rows.Next() {
		var voucherID string
		if e = rows.Scan(&voucherID); e != nil {
			rows.Close()
			return e
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if e = expireFile(ctx, tx, file.String, s.Now()); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE vouchers SET credential_state='SEALED',active=(? AND valid_until>? AND deleted_at IS NULL AND redeemed_at IS NULL),version=version+1 WHERE batch_id=?", activate, s.Now().UTC(), id); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "UPDATE voucher_batches SET state='SEALED',version=version+1 WHERE id=?", id)
	return e
}

// SealBatch 用途：幂等封存批次。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 对应操作的版本与发行指令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：权威批次或权限、状态、SQL 错误。
// 约束：封存及文件撤权、删除任务、审计一次提交。
func (s *Service) SealBatch(ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string) (Batch, error) {
	if !rules.ValidID(id) || !validVersion(in) {
		return Batch{}, fault(422, "VALIDATION_FAILED")
	}
	ref, e := s.mutate(ctx, a, "sealBatch", key, rid, struct {
		ID string `json:"id"`
		VersionReason
	}{id, in}, func(tx *sql.Tx) (string, error) { return id, s.sealBatchTx(ctx, tx, id, in.Version) })
	if e != nil {
		return Batch{}, e
	}
	return s.GetBatch(ctx, ref)
}

// SealVoucher 用途：封存已封存批次内的重置凭证。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 对应操作的版本与发行指令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：卡券或权限、状态、SQL 错误。
// 约束：只允许未消费停用的 EXPORTABLE 卡，永久撤销当前导出。
func (s *Service) SealVoucher(ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string) (Voucher, error) {
	if !rules.ValidID(id) || !validVersion(in) {
		return Voucher{}, fault(422, "VALIDATION_FAILED")
	}
	ref, e := s.mutate(ctx, a, "sealVoucher", key, rid, struct {
		ID string `json:"id"`
		VersionReason
	}{id, in}, func(tx *sql.Tx) (string, error) {
		v, file, _, e := s.lockVoucher(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if v.Version != in.Version {
			return "", fault(409, "VERSION_CONFLICT")
		}
		var state string
		if e = tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=?", v.BatchID).Scan(&state); e != nil {
			return "", e
		}
		if state != "SEALED" || v.State != "EXPORTABLE" || v.Active || v.Redeemed {
			return "", fault(409, "ILLEGAL_TRANSITION")
		}
		if e = expireFile(ctx, tx, file, s.Now()); e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, "UPDATE vouchers SET credential_state='SEALED',version=version+1 WHERE id=?", id)
		return id, e
	})
	if e != nil {
		return Voucher{}, e
	}
	return s.GetVoucher(ctx, ref)
}

// Reset 用途：重置未消费停用卡的凭证。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 对应操作的版本与发行指令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：短期交付引用或权限、复核、状态、文件、SQL 错误。
// 事务：新凭证、版本、撤会话和新文件引用原子提交；不缓存秘密。
func (s *Service) Reset(ctx context.Context, a identity.Actor, id string, in ResetInput, key, rid string) (Delivery, error) {
	if !rules.ValidID(id) || !validVersion(in.VersionReason) || !rules.ValidSecret(in.Length, in.Format) {
		return Delivery{}, fault(422, "VALIDATION_FAILED")
	}
	// Hash outside entity locks. No secret is ever stored in a command or replay record.
	secret, e := rules.Random(s.Random, rules.Alphabet(in.Format), in.Length)
	if e != nil {
		return Delivery{}, e
	}
	hash, e := s.Hash(secret)
	if e != nil {
		return Delivery{}, e
	}
	written := ""
	ref, e := s.mutate(ctx, a, "resetSecret", key, rid, struct {
		ID string `json:"id"`
		ResetInput
	}{id, in}, func(tx *sql.Tx) (string, error) {
		v, old, cred, e := s.lockVoucher(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if v.Version != in.Version {
			return "", fault(409, "VERSION_CONFLICT")
		}
		var state, number string
		if e = tx.QueryRowContext(ctx, "SELECT state,batch_no FROM voucher_batches WHERE id=?", v.BatchID).Scan(&state, &number); e != nil {
			return "", e
		}
		if state != "SEALED" || v.Active || v.Redeemed || cred >= 100000 {
			return "", fault(409, "ILLEGAL_TRANSITION")
		}
		// Old batch export is already expired; reset files are individual and can be revoked.
		if e = expireFile(ctx, tx, old, s.Now()); e != nil {
			return "", e
		}
		if e = revoke(ctx, tx, id); e != nil {
			return "", e
		}
		b := Batch{ID: "voucher:" + id, Number: number}
		storage, digest, size, e := s.writeExport(b, []credential{{v.Code, secret, hash}})
		if e != nil {
			return "", e
		}
		written = storage
		expires := s.Now().UTC().Add(24 * time.Hour)
		r, e := tx.ExecContext(ctx, "INSERT INTO files(storage_key,kind,mime_type,size_bytes,sha256,state,owner_admin_id,key_version,expires_at) VALUES(?,'CREDENTIAL_EXPORT','text/csv',?,?,'READY',?,?,?)", storage, size, digest, a.AdminID, s.KeyVersion, expires)
		if e != nil {
			return "", e
		}
		fid, e := r.LastInsertId()
		if e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, "UPDATE vouchers SET secret_hash=?,credential_file_id=?,credential_version=credential_version+1,version=version+1,credential_state='EXPORTABLE',active=0 WHERE id=?", hash, fid, id)
		if e != nil {
			return "", e
		}
		data, _ := json.Marshal(Delivery{id, fmt.Sprint(fid), expires, "EXPORTABLE"})
		return string(data), nil
	})
	if e != nil && written != "" {
		s.removeUnreferenced(written)
	}
	if e != nil {
		return Delivery{}, e
	}
	var delivery Delivery
	e = json.Unmarshal([]byte(ref), &delivery)
	return delivery, e
}

// Bulk 用途：执行显式 ID/version 的原子批量动作。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 对应操作的版本与发行指令；key 当前动作的幂等键。
// 参数：rid 审计请求标识。
// 返回值：各卡动作结果或校验、权限、状态、版本、SQL 错误。
// 约束：最多五百张，全成全败；冻结商品权益不变。
func (s *Service) Bulk(ctx context.Context, a identity.Actor, in BulkInput, key, rid string) (BulkResult, error) {
	return s.bulk(ctx, a, in, key, rid, "bulkVouchers")
}

// bulk 用途：执行批量或单卡有效期操作的公共事务规则。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 对应操作的版本与发行指令；key 当前动作的幂等键。
// 参数：rid 审计请求标识；op 内部操作标识。
// 返回值：动作结果或校验、权限、版本、SQL 错误。
// 约束：先按数值顺序锁全部批次再锁卡，有效期变更撤销对应会话。
func (s *Service) bulk(ctx context.Context, a identity.Actor, in BulkInput, key, rid, op string) (BulkResult, error) {
	if len(in.IDs) < 1 || len(in.IDs) > 500 || len(in.Versions) != len(in.IDs) || utf8.RuneCountInString(in.Reason) > 200 {
		return BulkResult{}, fault(422, "VALIDATION_FAILED")
	}
	switch in.Action {
	case "ACTIVATE", "DEACTIVATE", "SOFT_DELETE":
		if in.From != nil || in.Until != nil {
			return BulkResult{}, fault(422, "VALIDATION_FAILED")
		}
	case "UPDATE_VALIDITY":
		if in.From == nil || in.Until == nil || in.From.Year() < 1000 || in.Until.Year() > 9999 || !in.Until.After(*in.From) {
			return BulkResult{}, fault(422, "VALIDATION_FAILED")
		}
	default:
		return BulkResult{}, fault(422, "VALIDATION_FAILED")
	}
	versions := map[string]int64{}
	for i, id := range in.IDs {
		if !rules.ValidID(id) || versions[id] != 0 || in.Versions[i] < 1 || in.Versions[i] > 9007199254740991 {
			return BulkResult{}, fault(422, "VALIDATION_FAILED")
		}
		versions[id] = in.Versions[i]
	}
	in.IDs = append([]string(nil), in.IDs...)
	sort.Slice(in.IDs, func(i, j int) bool { return numericLess(in.IDs[i], in.IDs[j]) })
	in.Versions = make([]int64, len(in.IDs))
	for i, id := range in.IDs {
		in.Versions[i] = versions[id]
	}
	ref, e := s.mutate(ctx, a, op, key, rid, in, func(tx *sql.Tx) (string, error) {
		batchSet := map[string]bool{}
		for _, id := range in.IDs {
			var b string
			if e := tx.QueryRowContext(ctx, "SELECT batch_id FROM vouchers WHERE id=?", id).Scan(&b); e != nil {
				if e == sql.ErrNoRows {
					return "", fault(404, "NOT_FOUND")
				}
				return "", e
			}
			batchSet[b] = true
		}
		batchIDs := []string{}
		for b := range batchSet {
			batchIDs = append(batchIDs, b)
		}
		sort.Slice(batchIDs, func(i, j int) bool { return numericLess(batchIDs[i], batchIDs[j]) })
		states := map[string]string{}
		for _, b := range batchIDs {
			var state string
			if e := tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", b).Scan(&state); e != nil {
				return "", e
			}
			states[b] = state
		}
		result := BulkResult{Items: []Action{}}
		for i, id := range in.IDs {
			v, file, _, e := s.lockVoucher(ctx, tx, id)
			if e != nil {
				return "", e
			}
			if v.Version != in.Versions[i] {
				return "", fault(409, "VERSION_CONFLICT")
			}
			state := "DISABLED"
			if v.Redeemed && in.Action != "DEACTIVATE" {
				return "", fault(409, "ILLEGAL_TRANSITION")
			}
			switch in.Action {
			case "ACTIVATE":
				if v.State != "SEALED" || states[v.BatchID] != "SEALED" || !s.Now().Before(v.Until) {
					return "", fault(409, "ILLEGAL_TRANSITION")
				}
				_, e = tx.ExecContext(ctx, "UPDATE vouchers SET active=1,version=version+1 WHERE id=?", id)
				state = "ENABLED"
			case "DEACTIVATE":
				_, e = tx.ExecContext(ctx, "UPDATE vouchers SET active=0,version=version+1 WHERE id=?", id)
			case "SOFT_DELETE":
				if v.State == "EXPORTABLE" && states[v.BatchID] == "SEALED" {
					if e = expireFile(ctx, tx, file, s.Now()); e != nil {
						return "", e
					}
				}
				if v.Active {
					return "", fault(409, "ILLEGAL_TRANSITION")
				}
				_, e = tx.ExecContext(ctx, "UPDATE vouchers SET deleted_at=?,credential_state=IF(credential_state='EXPORTABLE' AND ?='SEALED','SEALED',credential_state),version=version+1 WHERE id=?", s.Now().UTC(), states[v.BatchID], id)
				state = "DELETED"
			case "UPDATE_VALIDITY":
				_, e = tx.ExecContext(ctx, "UPDATE vouchers SET valid_from=?,valid_until=?,version=version+1 WHERE id=?", in.From.UTC().Truncate(time.Microsecond), in.Until.UTC().Truncate(time.Microsecond), id)
				if v.Active {
					state = "ENABLED"
				}
			}
			if e != nil {
				return "", e
			}
			if in.Action == "UPDATE_VALIDITY" || in.Action == "SOFT_DELETE" {
				if e = revoke(ctx, tx, id); e != nil {
					return "", e
				}
			}
			result.Items = append(result.Items, Action{id, v.Version + 1, state})
		}
		raw, _ := json.Marshal(result)
		return string(raw), nil
	})
	if e != nil {
		return BulkResult{}, e
	}
	var result BulkResult
	e = json.Unmarshal([]byte(ref), &result)
	return result, e
}

// numericLess 用途：比较正十进制标识数值顺序。
// 参数：a 第一个规范十进制标识；b 第二个规范十进制标识。
// 返回值：a 小于 b 时为 true。
// 约束：不转换为有限精度数值。
func numericLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// Edit 用途：按完整快照更新未消费卡有效期。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 对应操作的版本与发行指令。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：卡券或校验、版本、状态、SQL 错误。
// 事务：按日期顺序锁日历，原子迁移名额和更新地址密文。
// 约束：保留冻结商品与库存预占；截止后或非待发状态禁止编辑。
func (s *Service) Edit(ctx context.Context, a identity.Actor, id string, in EditInput, key, rid string) (Voucher, error) {
	// The bulk validity engine enforces the same transaction and version checks.
	if !rules.ValidID(id) || !validVersion(in.VersionReason) {
		return Voucher{}, fault(422, "VALIDATION_FAILED")
	}
	_, e := s.bulk(ctx, a, BulkInput{IDs: []string{id}, Versions: []int64{in.Version}, Action: "UPDATE_VALIDITY", Reason: in.Reason, From: &in.From, Until: &in.Until}, key, rid, "editVoucher")
	if e != nil {
		return Voucher{}, e
	}
	return s.GetVoucher(ctx, id)
}

type EditInput struct {
	VersionReason
	From  time.Time `json:"valid_from"`
	Until time.Time `json:"valid_until"`
}
