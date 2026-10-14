package voucher

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Maintain 用途：到期封存批次及单卡并处理密文删除任务。
// 参数：ctx 请求上下文。
// 返回值：nil 或锁冲突、SQL、文件校验错误。
// 约束：与下载使用相同锁序，不重启下载权限。
func (s *Service) Maintain(ctx context.Context) error {
	rows, e := s.DB.QueryContext(ctx, "SELECT id,version FROM voucher_batches WHERE state='GENERATED' AND export_file_id IN (SELECT id FROM files WHERE expires_at<=?) ORDER BY id LIMIT 100", s.Now().UTC())
	if e != nil {
		return e
	}
	type target struct {
		ID      string
		Version int64
	}
	targets := []target{}
	for rows.Next() {
		var t target
		if e = rows.Scan(&t.ID, &t.Version); e != nil {
			rows.Close()
			return e
		}
		targets = append(targets, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, t := range targets {
		tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		e = s.sealBatchTx(ctx, tx, t.ID, t.Version)
		if e == nil {
			_, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','batch.auto_sealed','voucher_batch',?,'ttl-maintenance',JSON_OBJECT(),'SUCCESS')", t.ID)
		}
		if e == nil {
			e = tx.Commit()
		}
		tx.Rollback()
		if e != nil {
			return e
		}
	}
	rows, e = s.DB.QueryContext(ctx, "SELECT v.id FROM vouchers v JOIN files f ON f.id=v.credential_file_id JOIN voucher_batches b ON b.id=v.batch_id WHERE b.state='SEALED' AND v.deleted_at IS NULL AND v.credential_state='EXPORTABLE' AND f.expires_at<=? ORDER BY v.id LIMIT 100", s.Now().UTC())
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		v, file, _, e := s.lockVoucher(ctx, tx, id)
		if e == nil && v.State == "EXPORTABLE" {
			var expiryExpired bool
			e = tx.QueryRowContext(ctx, "SELECT expires_at<=? FROM files WHERE id=? FOR UPDATE", s.Now().UTC(), file).Scan(&expiryExpired)
			if e == nil && expiryExpired {
				e = expireFile(ctx, tx, file, s.Now())
				if e == nil {
					_, e = tx.ExecContext(ctx, "UPDATE vouchers SET credential_state='SEALED',version=version+1 WHERE id=?", id)
				}
				if e == nil {
					_, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','voucher.auto_sealed','voucher',?,'ttl-maintenance',JSON_OBJECT(),'SUCCESS')", id)
				}
			}
		}
		if e == nil {
			e = tx.Commit()
		}
		tx.Rollback()
		if e != nil {
			return e
		}
	}
	return s.DestroyExpired(ctx)
}

// DestroyExpired 用途：消费持久化 file.destroy 任务。
// 参数：ctx 请求上下文。
// 返回值：nil 或 SQL、存储路径错误。
// 约束：只删除 EXPIRED、DELETED 文件，删除失败记录并延迟重试。
func (s *Service) DestroyExpired(ctx context.Context) error {
	rows, e := s.DB.QueryContext(ctx, "SELECT id,payload FROM outbox_events WHERE event_type='file.destroy' AND state='PENDING' AND next_attempt_at<=? ORDER BY id LIMIT 100", s.Now().UTC())
	if e != nil {
		return e
	}
	type task struct {
		ID  string
		Raw []byte
	}
	tasks := []task{}
	for rows.Next() {
		var t task
		if e = rows.Scan(&t.ID, &t.Raw); e != nil {
			rows.Close()
			return e
		}
		tasks = append(tasks, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, task := range tasks {
		var payload struct {
			ID string `json:"file_id"`
		}
		if e = json.Unmarshal(task.Raw, &payload); e != nil {
			return e
		}
		var storage, state string
		if e = s.DB.QueryRowContext(ctx, "SELECT storage_key,state FROM files WHERE id=?", payload.ID).Scan(&storage, &state); e != nil {
			return e
		}
		if state != "EXPIRED" && state != "DELETED" {
			continue
		}
		if !strings.HasPrefix(storage, "credentials/") || filepath.Clean(storage) != storage || strings.Contains(storage, "..") {
			return fault(500, "FILE_UNAVAILABLE")
		}
		e = os.Remove(filepath.Join(s.Root, storage))
		if e != nil && !os.IsNotExist(e) {
			if _, updateErr := s.DB.ExecContext(ctx, "UPDATE outbox_events SET attempts=attempts+1,last_error_code='FILE_DELETE_FAILED',next_attempt_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 MINUTE) WHERE id=? AND state='PENDING'", task.ID); updateErr != nil {
				return updateErr
			}
			continue
		}
		if _, e = s.DB.ExecContext(ctx, "UPDATE outbox_events SET state='DONE',attempts=attempts+1,last_error_code=NULL WHERE id=? AND state='PENDING'", task.ID); e != nil {
			return e
		}
	}
	return nil
}
