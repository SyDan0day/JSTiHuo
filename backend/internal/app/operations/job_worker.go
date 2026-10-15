package operations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jstihuo/internal/app/identity"
	imports "jstihuo/internal/domain/imports"
	"jstihuo/internal/infra/integrations"
)

type JobClaim struct {
	ID      string
	Version int64
	job     jobRecord
}

var ErrJobLeaseLost = errors.New("JOB_LEASE_LOST")

// ClaimJob 用途：以 SKIP LOCKED 领取待处理或租约过期的文件任务。
// 参数：ctx 上下文。
// 返回值：带版本栅栏的任务、是否领取成功或数据库错误。
func (s *Service) ClaimJob(ctx context.Context) (JobClaim, bool, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return JobClaim{}, false, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM integration_jobs
		WHERE kind IN ('IMPORT_VOUCHERS','IMPORT_SHIPMENTS','EXPORT_REPORT','LABEL')
		AND (state='PENDING' OR state='RUNNING' AND lease_until<=?)
		ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`, s.Now().UTC()).Scan(&id)
	if err == sql.ErrNoRows {
		return JobClaim{}, false, nil
	}
	if err != nil {
		return JobClaim{}, false, err
	}
	job, err := loadJob(ctx, tx, id, false)
	if err != nil {
		return JobClaim{}, false, err
	}
	job.Options.Attempts++
	raw, err := json.Marshal(job.Options)
	if err != nil {
		return JobClaim{}, false, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET state='RUNNING',version=version+1,options=?,lease_until=? WHERE id=?", raw, s.Now().UTC().Add(time.Minute), id)
	if err != nil {
		return JobClaim{}, false, err
	}
	job.Version++
	job.State = "RUNNING"
	return JobClaim{ID: id, Version: job.Version, job: job}, true, tx.Commit()
}

// RenewJob 用途：续租仍属于当前版本且尚未到期的任务。
// 参数：ctx 上下文；claim 领取凭证。
// 返回值：nil、租约已失效或数据库错误，旧 worker 不能复活租约。
func (s *Service) RenewJob(ctx context.Context, claim JobClaim) error {
	now := s.Now().UTC()
	r, err := s.DB.ExecContext(ctx, `UPDATE integration_jobs SET lease_until=GREATEST(lease_until+INTERVAL 1 MICROSECOND,?)
		WHERE id=? AND state='RUNNING' AND version=? AND lease_until>?`, now.Add(time.Minute), claim.ID, claim.Version, now)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrJobLeaseLost
	}
	return nil
}

// lockClaim 用途：锁定并校验任务提交凭证。
// 参数：ctx 上下文；tx 当前事务；claim 领取凭证。
// 返回值：当前任务或租约、版本、数据库错误。
func (s *Service) lockClaim(ctx context.Context, tx *sql.Tx, claim JobClaim) (jobRecord, error) {
	job, err := loadJob(ctx, tx, claim.ID, true)
	if err != nil {
		return job, err
	}
	if job.State != "RUNNING" || job.Version != claim.Version || !job.Lease.Valid || !job.Lease.Time.After(s.Now()) {
		return job, ErrJobLeaseLost
	}
	return job, nil
}

// workerActor 用途：恢复任务原始管理员并复查当前来源权限。
// 参数：ctx 上下文；job 内部任务元数据。
// 返回值：只含已验证来源权限的管理员；停用、改权或配置错误时失败。
func (s *Service) workerActor(ctx context.Context, job jobRecord) (identity.Actor, error) {
	a := identity.Actor{Kind: "ADMIN", AdminID: job.Options.Owner, AuthVersion: job.Options.OwnerVersion, Permissions: []string{jobPermission(job.Kind)}}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if a.AdminID <= 0 || a.AuthVersion < 1 {
		return a, fault(403, "FORBIDDEN")
	}
	if err = identity.AuthorizeAdmin(ctx, tx, a, jobPermission(job.Kind)); err != nil {
		return a, err
	}
	if job.Kind == "IMPORT_SHIPMENTS" {
		if err = identity.AuthorizeAdmin(ctx, tx, a, "order.ship"); err != nil {
			return a, err
		}
		a.Permissions = append(a.Permissions, "order.ship")
	}
	if job.Kind == "EXPORT_REPORT" {
		permission := "order.read"
		if job.Options.Export != nil && job.Options.Export.Kind == "VOUCHER_METADATA" {
			permission = "voucher.read"
		}
		if err = identity.AuthorizeAdmin(ctx, tx, a, permission); err != nil {
			return a, err
		}
	}
	return a, tx.Commit()
}

// RunOnce 用途：处理一次文件任务并维护过期密文。
// 参数：ctx 可取消的 worker 上下文。
// 返回值：是否领取过任务及错误；暂时基础设施失败保留租约以恢复，确定失败写入脱敏结果。
func (s *Service) RunOnce(ctx context.Context) (bool, error) {
	if err := s.MaintainFiles(ctx); err != nil {
		return false, err
	}
	claim, ok, err := s.ClaimJob(ctx)
	if err != nil || !ok {
		return ok, err
	}
	if claim.job.Options.Attempts > 5 && claim.job.Kind != "LABEL" {
		return true, s.finishJob(ctx, claim, "FAILED", jobSummary{Failed: 1, Errors: []RowError{{Row: 1, Code: "RETRY_EXHAUSTED", Field: "job"}}}, nil)
	}
	err = s.ProcessJob(ctx, claim)
	if err == nil || errors.Is(err, ErrJobLeaseLost) || ctx.Err() != nil {
		return true, err
	}
	var f *identity.Fault
	if errors.As(err, &f) && f.Status < 500 {
		summary := jobSummary{Failed: 1, Errors: []RowError{{Row: 1, Code: f.Code, Field: "job"}}}
		report, reportErr := s.errorReport(summary)
		if reportErr != nil {
			return true, reportErr
		}
		defer s.cleanupUnreferenced(ctx, report.Storage)
		return true, s.finishJob(ctx, claim, "FAILED", summary, &report)
	}
	return true, err
}

// ProcessJob 用途：在有界时间内处理领取的导入或导出任务。
// 参数：ctx 上下文；claim 含来源与阶段的领取凭证。
// 返回值：成功、租约失效、业务或基础设施错误，不打印输入正文。
func (s *Service) ProcessJob(ctx context.Context, claim JobClaim) error {
	work, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := s.RenewJob(work, claim); err != nil {
		return err
	}
	actor, err := s.workerActor(work, claim.job)
	if err != nil {
		var f *identity.Fault
		if claim.job.Kind == "LABEL" && errors.As(err, &f) && f.Status < 500 {
			state := "FAILED"
			if claim.job.Options.CallStarted {
				state = "UNKNOWN"
			}
			return s.finishLabel(work, claim, state, integrations.Label{}, "ORIGIN_PERMISSION_REVOKED")
		}
		return err
	}
	if claim.job.Kind == "LABEL" {
		return s.processLabel(work, claim)
	}
	if claim.job.Kind == "EXPORT_REPORT" {
		return s.processExport(work, claim, actor)
	}
	file, err := loadPrivate(work, s.DB, claim.job.FileID, false)
	if err != nil {
		return err
	}
	if hex.EncodeToString(file.SHA) != claim.job.Options.InputHash {
		return fault(409, "VERSION_CONFLICT")
	}
	plain, err := s.readPrivate(file, "IMPORT")
	if err != nil {
		return err
	}
	defer clear(plain)
	rows, errs := imports.Parse(claim.job.Kind, plain)
	defer func() {
		for _, row := range rows {
			clear(row.Values)
		}
	}()
	if len(errs) > 0 {
		return s.finishImport(work, claim, jobSummary{Failed: len(errs), Errors: errs})
	}
	if claim.job.Kind == "IMPORT_VOUCHERS" {
		return s.processVoucherImport(work, claim, actor, rows)
	}
	return s.processShipmentImport(work, claim, actor, rows)
}

// errorReport 用途：把脱敏逐行错误编码成短期私有报表。
// 参数：summary 只包含行号、字段和固定错误码的摘要。
// 返回值：已落盘的报表元数据或写入错误。
func (s *Service) errorReport(summary jobSummary) (storedFile, error) {
	var data bytes.Buffer
	writer := csv.NewWriter(&data)
	if err := writer.Write([]string{"row", "field", "code"}); err != nil {
		return storedFile{}, err
	}
	for _, item := range summary.Errors {
		if err := writer.Write([]string{fmt.Sprint(item.Row), item.Field, item.Code}); err != nil {
			return storedFile{}, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return storedFile{}, err
	}
	return s.writePrivate("REPORT", data.Bytes())
}

// finishImport 用途：保存预检或逐行导入摘要及错误文件。
// 参数：ctx 上下文；claim 领取凭证；summary 脱敏结果。
// 返回值：租约、结果文件或数据库错误，不把部分失败显示为全部成功。
func (s *Service) finishImport(ctx context.Context, claim JobClaim, summary jobSummary) error {
	var report *storedFile
	if len(summary.Errors) > 0 {
		file, err := s.errorReport(summary)
		if err != nil {
			return err
		}
		report = &file
		defer s.cleanupUnreferenced(ctx, file.Storage)
	}
	return s.finishJob(ctx, claim, "READY", summary, report)
}

// finishJob 用途：在租约栅栏内提交任务终态和私有结果文件引用。
// 参数：ctx 上下文；claim 领取凭证；state READY 或 FAILED；summary 脱敏摘要；report 可选结果文件。
// 返回值：租约、权限、文件元数据或数据库错误。
// 事务：终态、结果文件、输入过期与审计一起提交；原始导入文件不再可读取。
func (s *Service) finishJob(ctx context.Context, claim JobClaim, state string, summary jobSummary, report *storedFile) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	job, err := s.lockClaim(ctx, tx, claim)
	if err != nil {
		return err
	}
	if state == "READY" {
		actor := identity.Actor{Kind: "ADMIN", AdminID: job.Options.Owner, AuthVersion: job.Options.OwnerVersion}
		if err = identity.AuthorizeAdmin(ctx, tx, actor, jobPermission(job.Kind)); err != nil {
			return err
		}
		if job.Kind == "EXPORT_REPORT" && job.Options.Export != nil {
			permission := "order.read"
			if job.Options.Export.Kind == "VOUCHER_METADATA" {
				permission = "voucher.read"
			}
			if err = identity.AuthorizeAdmin(ctx, tx, actor, permission); err != nil {
				return err
			}
		}
	}
	var resultFile any
	if report != nil {
		file, err := insertPrivate(ctx, tx, *report, job.Options.Owner)
		if err != nil {
			return err
		}
		resultFile = file.ID
	}
	if err = s.completeJobTx(ctx, tx, job, state, summary, resultFile); err != nil {
		return err
	}
	return tx.Commit()
}

// completeJobTx 用途：更新已锁定任务的状态并记录系统审计。
// 参数：ctx 上下文；tx 当前事务；job 已锁定任务；state 终态；summary 脱敏摘要；resultFile 可选报表 ID。
// 返回值：数据库错误；调用方负责提交和卡券原子性。
func (s *Service) completeJobTx(ctx context.Context, tx *sql.Tx, job jobRecord, state string, summary jobSummary, resultFile any) error {
	if err := s.inject(ctx, tx, "job:finish"); err != nil {
		return err
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET state=?,result_summary=?,result_file_id=?,lease_until=NULL,version=version+1 WHERE id=?", state, raw, resultFile, job.ID)
	if err != nil {
		return err
	}
	if job.Phase == "APPLY" && job.FileID != "" {
		_, err = tx.ExecContext(ctx, "UPDATE files SET state='EXPIRED',version=version+1 WHERE id=?", job.FileID)
		if err != nil {
			return err
		}
	} else if job.Phase == "PREVIEW" && state == "READY" && summary.Failed == 0 {
		_, err = tx.ExecContext(ctx, "UPDATE files SET state='READY',version=version+1 WHERE id=?", job.FileID)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result)
		VALUES('SYSTEM','job.finished','job',?,?,JSON_OBJECT('phase',?,'processed',?,'failed',?),?)`, job.ID, "job-"+job.ID, job.Phase, summary.Processed, summary.Failed, state)
	return err
}

// MaintainFiles 用途：过期并清理 IMPORT/REPORT 密文，不触碰图片与凭证专用锁。
// 参数：ctx 上下文。
// 返回值：数据库或删除错误；未登记孤儿在二十四小时后清理。
func (s *Service) MaintainFiles(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, "UPDATE files SET state='EXPIRED',version=version+1 WHERE kind IN ('IMPORT','REPORT') AND state IN ('STAGED','READY') AND expires_at<=?", s.Now().UTC())
	if err != nil {
		return err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT storage_key FROM files WHERE kind IN ('IMPORT','REPORT') AND state='EXPIRED' LIMIT 100")
	if err != nil {
		return err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if !privateStorage.MatchString(key) {
			return fault(503, "FILE_UNAVAILABLE")
		}
		if err = os.Remove(filepath.Join(s.FileRoot, key)); err != nil && !os.IsNotExist(err) {
			return err
		}
		if _, err = s.DB.ExecContext(ctx, "UPDATE files SET state='DELETED',version=version+1 WHERE storage_key=? AND state='EXPIRED'", key); err != nil {
			return err
		}
	}
	return s.cleanupPrivateOrphans(ctx)
}

// cleanupPrivateOrphans 用途：删除无数据库引用且超过保留期的私有孤儿密文。
// 参数：ctx 上下文。
// 返回值：扫描、核对或删除错误，不删除未知目录内容。
func (s *Service) cleanupPrivateOrphans(ctx context.Context) error {
	for _, directory := range []string{"imports", "reports"} {
		entries, err := os.ReadDir(filepath.Join(s.FileRoot, directory))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if s.Now().Sub(info.ModTime()) < 24*time.Hour {
				continue
			}
			key := directory + "/" + entry.Name()
			if !privateStorage.MatchString(key) && !strings.HasPrefix(entry.Name(), ".staged-") {
				continue
			}
			var count int
			if err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE storage_key=?", key).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				if err = os.Remove(filepath.Join(s.FileRoot, key)); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	return nil
}
