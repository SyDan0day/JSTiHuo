package operations

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"jstihuo/internal/app/identity"
	imports "jstihuo/internal/domain/imports"
	rules "jstihuo/internal/domain/voucher"
)

type ImportInput struct {
	FileID    string `json:"file_id"`
	Kind      string `json:"kind"`
	BatchID   string `json:"batch_id,omitempty"`
	CarrierID string `json:"carrier_id,omitempty"`
}

type ExportInput struct {
	Kind    string `json:"kind"`
	BatchID string `json:"batch_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

type Job struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	State        string `json:"state"`
	Processed    int    `json:"processed"`
	Failed       int    `json:"failed"`
	ResultFileID string `json:"result_file_id,omitempty"`
	Phase        string `json:"phase"`
	Version      int64  `json:"version"`
	OrderID      string `json:"order_id,omitempty"`
	CarrierID    string `json:"carrier_id,omitempty"`
	Tracking     string `json:"tracking_no,omitempty"`
}

type jobOptions struct {
	Phase                string       `json:"phase"`
	Owner                int64        `json:"owner_admin_id"`
	OwnerVersion         int64        `json:"owner_auth_version"`
	BatchID              string       `json:"batch_id,omitempty"`
	CarrierID            string       `json:"carrier_id,omitempty"`
	InputHash            string       `json:"input_sha256,omitempty"`
	Export               *ExportInput `json:"export,omitempty"`
	Attempts             int          `json:"attempts,omitempty"`
	ExpectedOrderVersion int64        `json:"expected_order_version,omitempty"`
	Provider             string       `json:"provider,omitempty"`
	CallStarted          bool         `json:"call_started,omitempty"`
	Confirmed            bool         `json:"confirmed,omitempty"`
}

type RowError = imports.RowError

type jobSummary struct {
	Processed int        `json:"processed"`
	Failed    int        `json:"failed"`
	Errors    []RowError `json:"row_errors"`
	CarrierID string     `json:"carrier_id,omitempty"`
	Tracking  string     `json:"tracking_no,omitempty"`
}

type jobRecord struct {
	Job
	Options jobOptions
	Summary jobSummary
	FileID  string
	Lease   sql.NullTime
}

type VersionReason struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}

// loadJob 用途：读取任务内部元数据，不把所有者或原始输入直接暴露到 API。
// 参数：ctx 上下文；q 数据源；id 任务 ID；lock 是否排他锁定。
// 返回值：任务记录或不存在、解码及数据库错误。
func loadJob(ctx context.Context, q queryer, id string, lock bool) (jobRecord, error) {
	var out jobRecord
	var options, summary []byte
	var file, result, orderID sql.NullString
	query := "SELECT id,kind,state,version,file_id,result_file_id,options,result_summary,lease_until,order_id FROM integration_jobs WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Kind, &out.State, &out.Version, &file, &result, &options, &summary, &out.Lease, &orderID)
	if err == sql.ErrNoRows {
		return out, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(options, &out.Options); err != nil {
		return out, err
	}
	if err = json.Unmarshal(summary, &out.Summary); err != nil {
		return out, err
	}
	out.FileID, out.ResultFileID = file.String, result.String
	out.OrderID, out.CarrierID, out.Tracking = orderID.String, out.Summary.CarrierID, out.Summary.Tracking
	out.Phase, out.Processed, out.Failed = out.Options.Phase, out.Summary.Processed, out.Summary.Failed
	return out, nil
}

// jobPermission 用途：确定任务对应的原始业务权限。
// 参数：kind 受控任务种类。
// 返回值：权限键；未知任务返回空值。
func jobPermission(kind string) string {
	switch kind {
	case "IMPORT_VOUCHERS", "IMPORT_SHIPMENTS":
		return "file.import"
	case "EXPORT_REPORT":
		return "file.export"
	case "LABEL":
		return "order.ship"
	case "TRACKING":
		return "order.read"
	default:
		return ""
	}
}

// authorizeJob 用途：验证任务读取或确认的来源权限与归属。
// 参数：a 管理员；job 内部任务记录。
// 返回值：nil 或 403，不接受缺失 owner 的历史或伪造记录。
func authorizeJob(a identity.Actor, job jobRecord) error {
	permission := jobPermission(job.Kind)
	if a.Kind != "ADMIN" || job.Options.Owner <= 0 || permission == "" || !a.Can(permission) || !a.Can("file.read") {
		return fault(403, "FORBIDDEN")
	}
	if job.Options.Owner != a.AdminID && !a.Can("file.export") {
		return fault(403, "FORBIDDEN")
	}
	return nil
}

// GetJob 用途：读取授权范围内的脱敏任务状态。
// 参数：ctx 上下文；a 管理员；id 任务 ID。
// 返回值：任务公开状态；无权限、无归属或不存在返回错误。
func (s *Service) GetJob(ctx context.Context, a identity.Actor, id string) (Job, error) {
	if !rules.ValidID(id) {
		return Job{}, fault(422, "INVALID_ARGUMENT")
	}
	job, err := loadJob(ctx, s.DB, id, false)
	if err != nil {
		return Job{}, err
	}
	if err = authorizeJob(a, job); err != nil {
		return Job{}, err
	}
	return job.Job, nil
}

// ImportJob 用途：建立加密源文件的异步预检任务，按内容、种类和批次去重。
// 参数：ctx 上下文；a 管理员；in 文件及受控导入参数；key 幂等键；rid 请求标识。
// 返回值：PREVIEW 任务 JSON 或输入、归属、批次状态错误。
func (s *Service) ImportJob(ctx context.Context, a identity.Actor, in ImportInput, key, rid string) (json.RawMessage, error) {
	validKind := in.Kind == "IMPORT_VOUCHERS" || in.Kind == "IMPORT_SHIPMENTS"
	if !rules.ValidID(in.FileID) || !validKind ||
		(in.Kind == "IMPORT_VOUCHERS" && (!rules.ValidID(in.BatchID) || in.CarrierID != "")) ||
		(in.Kind == "IMPORT_SHIPMENTS" && (in.BatchID != "" || in.CarrierID != "" && !rules.ValidID(in.CarrierID))) {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	command := mutation{Operation: "importJob", Permission: "file.import", Entity: "job", Key: key, RequestID: rid, Status: 202, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		if in.Kind == "IMPORT_SHIPMENTS" {
			if err := identity.AuthorizeAdmin(ctx, tx, a, "order.ship"); err != nil {
				return nil, "", err
			}
		}
		file, err := loadPrivate(ctx, tx, in.FileID, true)
		if err != nil {
			return nil, "", err
		}
		if file.Kind != "IMPORT" || file.Owner != a.AdminID {
			return nil, "", fault(403, "FORBIDDEN")
		}
		options := jobOptions{Phase: "PREVIEW", Owner: a.AdminID, OwnerVersion: a.AuthVersion, BatchID: in.BatchID, CarrierID: in.CarrierID, InputHash: hex.EncodeToString(file.SHA)}
		raw, err := json.Marshal(options)
		if err != nil {
			return nil, "", err
		}
		jobKey := "import:" + options.InputHash + ":" + in.Kind + ":" + in.BatchID
		_, err = tx.ExecContext(ctx, `INSERT INTO integration_jobs(job_key,kind,file_id,options,result_summary)
			VALUES(?,?,?,?,JSON_OBJECT()) ON DUPLICATE KEY UPDATE job_key=job_key`, jobKey, in.Kind, in.FileID, raw)
		if err != nil {
			return nil, "", err
		}
		var id string
		if err = tx.QueryRowContext(ctx, "SELECT id FROM integration_jobs WHERE job_key=? FOR UPDATE", jobKey).Scan(&id); err != nil {
			return nil, "", err
		}
		job, err := loadJob(ctx, tx, id, false)
		if err != nil {
			return nil, "", err
		}
		if job.Options.Owner != a.AdminID {
			return nil, "", fault(403, "FORBIDDEN")
		}
		if job.Options.CarrierID != in.CarrierID {
			return nil, "", fault(409, "IDEMPOTENCY_CONFLICT")
		}
		if job.State == "PENDING" && job.Version == 1 {
			if !s.Now().Before(file.Expires) || (file.State != "STAGED" && file.State != "READY") {
				return nil, "", fault(410, "EXPORT_EXPIRED")
			}
			if in.Kind == "IMPORT_VOUCHERS" {
				if err = checkImportBatch(ctx, tx, in.BatchID); err != nil {
					return nil, "", err
				}
			}
			if err = s.jobEvent(ctx, tx, id, 1); err != nil {
				return nil, "", err
			}
		}
		return job.Job, id, nil
	})
}

// checkImportBatch 用途：确认导入目标仍为未生成的草稿批次。
// 参数：ctx 上下文；tx 事务；id 批次 ID。
// 返回值：nil 或不存在、非草稿、已有卡券错误。
func checkImportBatch(ctx context.Context, tx *sql.Tx, id string) error {
	var state string
	if err := tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", id).Scan(&state); err != nil {
		if err == sql.ErrNoRows {
			return fault(404, "NOT_FOUND")
		}
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM vouchers WHERE batch_id=?", id).Scan(&count); err != nil {
		return err
	}
	if state != "DRAFT" || count != 0 {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	return nil
}

// CommitImport 用途：只把无错误预检任务原子转为实际导入阶段。
// 参数：ctx 上下文；a 管理员；id 任务；in 版本与原因；key 幂等键；rid 请求标识。
// 返回值：APPLY 任务 JSON；重复确认不重复执行，过期或旧版本返回错误。
func (s *Service) CommitImport(ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string) (json.RawMessage, error) {
	if !rules.ValidID(id) || in.Version < 1 || in.Version >= 9007199254740991 || !validText(in.Reason, 0, 200) {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	command := mutation{Operation: "commitImport", Permission: "file.import", Entity: "job", Key: key, RequestID: rid, Status: 202, Body: struct {
		ID    string
		Input VersionReason
	}{id, in}}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		job, err := loadJob(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if err = authorizeJob(a, job); err != nil {
			return nil, "", err
		}
		if job.Options.Owner != a.AdminID {
			return nil, "", fault(403, "FORBIDDEN")
		}
		if job.Kind != "IMPORT_VOUCHERS" && job.Kind != "IMPORT_SHIPMENTS" {
			return nil, "", fault(409, "ILLEGAL_TRANSITION")
		}
		if job.Phase == "APPLY" {
			return job.Job, id, nil
		}
		if job.Version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		if job.State != "READY" || job.Failed != 0 {
			return nil, "", fault(422, "VALIDATION_FAILED")
		}
		file, err := loadPrivate(ctx, tx, job.FileID, true)
		if err != nil {
			return nil, "", err
		}
		plain, err := s.readPrivate(file, "IMPORT")
		if err != nil {
			return nil, "", err
		}
		clear(plain)
		if hex.EncodeToString(file.SHA) != job.Options.InputHash {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		job.Options.Phase, job.Options.Attempts = "APPLY", 0
		job.Options.OwnerVersion = a.AuthVersion
		raw, err := json.Marshal(job.Options)
		if err != nil {
			return nil, "", err
		}
		_, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET state='PENDING',options=?,result_summary=JSON_OBJECT(),lease_until=NULL,version=version+1 WHERE id=?", raw, id)
		if err != nil {
			return nil, "", err
		}
		job.Phase, job.State, job.Version = "APPLY", "PENDING", job.Version+1
		job.Processed, job.Failed = 0, 0
		if err = s.jobEvent(ctx, tx, id, job.Version); err != nil {
			return nil, "", err
		}
		return job.Job, id, nil
	})
}

// ExportJob 用途：创建限定字段的异步脱敏报表。
// 参数：ctx 上下文；a 管理员；in 报表种类和白名单筛选；key 幂等键；rid 请求标识。
// 返回值：APPLY 任务 JSON；不接受 PII 扩展、任意列或任意查询。
func (s *Service) ExportJob(ctx context.Context, a identity.Actor, in ExportInput, key, rid string) (json.RawMessage, error) {
	if in.Kind != "ORDERS" && in.Kind != "VOUCHER_METADATA" {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	if in.BatchID != "" && (!rules.ValidID(in.BatchID) || in.Kind != "VOUCHER_METADATA") {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	allowed := ",PENDING,SHIPPED,COMPLETED,CANCELLED,"
	if in.Kind == "VOUCHER_METADATA" {
		allowed = ",SEALED,EXPORTABLE,"
	}
	if in.Status != "" && !strings.Contains(allowed, ","+in.Status+",") {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	command := mutation{Operation: "exportJob", Permission: "file.export", Entity: "job", Key: key, RequestID: rid, Status: 202, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		permission := "order.read"
		if in.Kind == "VOUCHER_METADATA" {
			permission = "voucher.read"
		}
		if err := identity.AuthorizeAdmin(ctx, tx, a, permission); err != nil {
			return nil, "", err
		}
		options := jobOptions{Phase: "APPLY", Owner: a.AdminID, OwnerVersion: a.AuthVersion, Export: &in}
		raw, err := json.Marshal(options)
		if err != nil {
			return nil, "", err
		}
		jobKey := fmt.Sprintf("export:%d:%s", a.AdminID, key)
		r, err := tx.ExecContext(ctx, "INSERT INTO integration_jobs(job_key,kind,options,result_summary) VALUES(?,'EXPORT_REPORT',?,JSON_OBJECT())", jobKey, raw)
		if err != nil {
			return nil, "", err
		}
		id, err := r.LastInsertId()
		if err != nil {
			return nil, "", err
		}
		job := Job{ID: fmt.Sprint(id), Kind: "EXPORT_REPORT", State: "PENDING", Phase: "APPLY", Version: 1}
		if err = s.jobEvent(ctx, tx, job.ID, 1); err != nil {
			return nil, "", err
		}
		return job, job.ID, nil
	})
}

// jobEvent 用途：同事务保存任务就绪事件，不包含输入文件正文。
// 参数：ctx 上下文；tx 事务；id 任务 ID；version 任务版本。
// 返回值：nil 或数据库错误。
func (s *Service) jobEvent(ctx context.Context, tx *sql.Tx, id string, version int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at)
		VALUES(?,'job.requested',JSON_OBJECT('job_id',?),?) ON DUPLICATE KEY UPDATE event_key=event_key`, fmt.Sprintf("job:%s:%d", id, version), id, s.Now().UTC())
	return err
}
