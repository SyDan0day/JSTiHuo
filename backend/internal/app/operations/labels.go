package operations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
	"jstihuo/internal/infra/integrations"
)

type ResolveJob struct {
	Version    int64  `json:"version"`
	Resolution string `json:"resolution"`
	Evidence   string `json:"evidence_reference"`
	Reason     string `json:"reason"`
	CarrierID  string `json:"carrier_id,omitempty"`
	Tracking   string `json:"tracking_no,omitempty"`
}

// labelOrder 用途：锁定待发配送订单，作为外部面单与人工发货的共同栅栏。
// 参数：ctx 上下文；tx 事务；id 原订单；version 非零时检查版本。
// 返回值：当前版本或状态、版本、数据库错误。
func labelOrder(ctx context.Context, tx *sql.Tx, id string, version int64) (int64, error) {
	var actual int64
	var state, mode string
	var deleted bool
	err := tx.QueryRowContext(ctx, "SELECT version,status,fulfillment_mode,deleted_at IS NOT NULL FROM orders WHERE id=? FOR UPDATE", id).Scan(&actual, &state, &mode, &deleted)
	if err == sql.ErrNoRows {
		return 0, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return 0, err
	}
	if version != 0 && actual != version {
		return 0, fault(409, "VERSION_CONFLICT")
	}
	if deleted || state != "PENDING" || mode != "DELIVERY" {
		return 0, fault(409, "ILLEGAL_TRANSITION")
	}
	return actual, nil
}

// AutoLabel 用途：按已配置默认承运商建立与订单版本绑定的模拟面单任务。
// 参数：ctx 上下文；a 管理员；id 订单；in 版本及原因；key 幂等键；rid 请求标识。
// 返回值：任务 JSON 或未配置集成、状态、权限错误。
// 约束：未配置真实适配器的承运商返回 503，不产生假成功任务。
func (s *Service) AutoLabel(ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string) (json.RawMessage, error) {
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !validText(strings.TrimSpace(in.Reason), 1, 200) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	command := mutation{Operation: "autoLabel", Target: id, Permission: "order.ship", Entity: "job", Key: key, RequestID: rid, Status: 202, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		if _, err := labelOrder(ctx, tx, id, in.Version); err != nil {
			return nil, "", err
		}
		var existing string
		err := tx.QueryRowContext(ctx, "SELECT id FROM integration_jobs WHERE order_id=? AND kind='LABEL' AND state IN ('PENDING','RUNNING','READY','UNKNOWN') ORDER BY id DESC LIMIT 1 FOR UPDATE", id).Scan(&existing)
		if err == nil {
			job, err := loadJob(ctx, tx, existing, false)
			if err == nil {
				err = authorizeJob(a, job)
			}
			return job.Job, existing, err
		}
		if err != sql.ErrNoRows {
			return nil, "", err
		}
		var raw []byte
		err = tx.QueryRowContext(ctx, "SELECT value FROM system_configs WHERE config_key='shipping_calendar' FOR SHARE").Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, "", err
		}
		var config struct {
			CarrierID string `json:"default_carrier_id"`
		}
		if len(raw) > 0 && json.Unmarshal(raw, &config) != nil {
			return nil, "", fault(503, "INTEGRATION_DISABLED")
		}
		if !rules.ValidID(config.CarrierID) {
			return nil, "", fault(503, "INTEGRATION_DISABLED")
		}
		var provider string
		var enabled bool
		err = tx.QueryRowContext(ctx, "SELECT provider,enabled FROM carriers WHERE id=? FOR SHARE", config.CarrierID).Scan(&provider, &enabled)
		if err == sql.ErrNoRows || err == nil && (!enabled || s.Carriers[provider] == nil) {
			return nil, "", fault(503, "INTEGRATION_DISABLED")
		}
		if err != nil {
			return nil, "", err
		}
		opts := jobOptions{Phase: "APPLY", Owner: a.AdminID, OwnerVersion: a.AuthVersion, CarrierID: config.CarrierID, ExpectedOrderVersion: in.Version, Provider: provider}
		raw, err = json.Marshal(opts)
		if err != nil {
			return nil, "", err
		}
		jobKey := fmt.Sprintf("label:%s:%d:%s", id, in.Version, integrations.Token(key))
		result, err := tx.ExecContext(ctx, `INSERT INTO integration_jobs(job_key,kind,order_id,options,result_summary,provider_request_id) VALUES(?,'LABEL',?,?,JSON_OBJECT(),?)`, jobKey, id, raw, jobKey)
		if err != nil {
			return nil, "", err
		}
		number, err := result.LastInsertId()
		if err != nil {
			return nil, "", err
		}
		jobID := fmt.Sprint(number)
		if err = s.jobEvent(ctx, tx, jobID, 1); err != nil {
			return nil, "", err
		}
		job, err := loadJob(ctx, tx, jobID, false)
		return job.Job, jobID, err
	})
}

// beginLabelCall 用途：在外部调用前持久化调用标记并复查订单版本。
// 参数：ctx 上下文；claim 领取凭证。
// 返回值：稳定供应商请求键或状态、租约错误；事务内不调用供应商。
func (s *Service) beginLabelCall(ctx context.Context, claim JobClaim) (string, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = labelOrder(ctx, tx, claim.job.OrderID, claim.job.Options.ExpectedOrderVersion); err != nil {
		return "", err
	}
	job, err := s.lockClaim(ctx, tx, claim)
	if err != nil {
		return "", err
	}
	job.Options.CallStarted = true
	raw, err := json.Marshal(job.Options)
	if err != nil {
		return "", err
	}
	var requestKey string
	if err = tx.QueryRowContext(ctx, "SELECT provider_request_id FROM integration_jobs WHERE id=?", claim.ID).Scan(&requestKey); err != nil {
		return "", err
	}
	if requestKey == "" {
		return "", fault(409, "CONSISTENCY_CONFLICT")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET options=? WHERE id=?", raw, claim.ID); err != nil {
		return "", err
	}
	return requestKey, tx.Commit()
}

// processLabel 用途：在数据库事务之外调用面单适配器，恢复时先查询或使用同一幂等键。
// 参数：ctx 上下文；claim 当前领取凭证。
// 返回值：结果提交或租约、基础设施错误；未知结果保持 UNKNOWN，禁止盲重试。
func (s *Service) processLabel(ctx context.Context, claim JobClaim) error {
	provider := s.Carriers[claim.job.Options.Provider]
	if provider == nil {
		return s.finishLabel(ctx, claim, "FAILED", integrations.Label{}, "INTEGRATION_DISABLED")
	}
	key, err := s.beginLabelCall(ctx, claim)
	if err != nil {
		return err
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var out integrations.Label
	var found bool
	if claim.job.Options.CallStarted {
		out, found, err = provider.GetLabel(work, key)
		if err != nil || !found && !provider.Idempotent() {
			return s.finishLabel(ctx, claim, "UNKNOWN", out, "RESULT_UNKNOWN")
		}
	}
	if !found {
		out, err = provider.CreateLabel(work, key, integrations.LabelRequest{OrderID: claim.job.OrderID, CarrierID: claim.job.Options.CarrierID, Version: claim.job.Options.ExpectedOrderVersion})
	}
	state, code := "READY", ""
	if err != nil {
		state, code = "UNKNOWN", "RESULT_UNKNOWN"
		if errors.Is(err, integrations.ErrRejected) {
			state, code = "FAILED", "PROVIDER_REJECTED"
		} else if errors.Is(err, integrations.ErrTemporary) {
			state, code = "FAILED", "PROVIDER_UNAVAILABLE"
		}
	}
	if err == nil && (out.CarrierID != claim.job.Options.CarrierID || !regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(out.Tracking)) {
		state, code = "UNKNOWN", "INVALID_PROVIDER_RESULT"
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.finishLabel(ctx, claim, state, out, code)
}

// finishLabel 用途：按订单锁与任务版本栅栏持久化外部结果。
// 参数：ctx 上下文；claim 凭证；state 固定结果状态；out 供应商单号；code 固定错误码。
// 返回值：nil 或版本、租约、数据库错误；不自动确认发货。
func (s *Service) finishLabel(ctx context.Context, claim JobClaim, state string, out integrations.Label, code string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = labelOrder(ctx, tx, claim.job.OrderID, claim.job.Options.ExpectedOrderVersion); err != nil {
		return err
	}
	if _, err = s.lockClaim(ctx, tx, claim); err != nil {
		return err
	}
	summary := jobSummary{Errors: []RowError{}}
	if state == "READY" {
		summary.Processed = 1
		summary.CarrierID = out.CarrierID
		summary.Tracking = out.Tracking
	} else {
		summary.Failed = 1
		summary.Errors = []RowError{{Row: 1, Code: code, Field: "provider"}}
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	if err = s.inject(ctx, tx, "label:finish"); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET state=?,result_summary=?,lease_until=NULL,version=version+1 WHERE id=?", state, raw, claim.ID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','label.result','job',?,'label-worker',JSON_OBJECT('state',?,'code',?),'SUCCESS')`, claim.ID, state, code)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ResolveLabel 用途：经近期复核与证据引用核对未知面单或确认撤销未发面单。
// 参数：ctx 上下文；a 管理员；id 任务；in 版本与核对结论；key 幂等键；rid 请求标识。
// 返回值：任务 JSON 或状态、权限、复核、版本错误；不撤销已发订单。
func (s *Service) ResolveLabel(ctx context.Context, a identity.Actor, id string, in ResolveJob, key, rid string) (json.RawMessage, error) {
	in.Evidence = strings.TrimSpace(in.Evidence)
	in.Tracking = strings.TrimSpace(in.Tracking)
	valid := in.Resolution == "CONFIRMED_VOID" || in.Resolution == "CONFIRMED_FAILED" || in.Resolution == "CONFIRMED_CREATED"
	if !rules.ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || !valid || !validText(in.Evidence, 1, 200) || !validText(in.Reason, 0, 200) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	created := in.Resolution == "CONFIRMED_CREATED"
	if created && (!rules.ValidID(in.CarrierID) || !regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(in.Tracking)) || !created && (in.CarrierID != "" || in.Tracking != "") {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	prior, err := loadJob(ctx, s.DB, id, false)
	if err != nil {
		return nil, err
	}
	if prior.Kind != "LABEL" {
		return nil, fault(409, "ILLEGAL_TRANSITION")
	}
	digest := sha256.Sum256([]byte(in.Evidence))
	command := mutation{Operation: "resolveJob", Target: id, Permission: "integration.manage", Entity: "job", Key: key, RequestID: rid, Body: in, Recent: true, Diff: map[string]any{"resolution": in.Resolution, "evidence_sha256": hex.EncodeToString(digest[:])}}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		if _, err := labelOrder(ctx, tx, prior.OrderID, 0); err != nil {
			return nil, "", err
		}
		job, err := loadJob(ctx, tx, id, true)
		if err != nil {
			return nil, "", err
		}
		if job.Version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		if job.Options.Confirmed || job.State != "UNKNOWN" && (job.State != "READY" || in.Resolution != "CONFIRMED_VOID") {
			return nil, "", fault(409, "ILLEGAL_TRANSITION")
		}
		state := "FAILED"
		if created {
			if in.CarrierID != job.Options.CarrierID {
				return nil, "", fault(422, "VALIDATION_FAILED")
			}
			var enabled bool
			if err = tx.QueryRowContext(ctx, "SELECT enabled FROM carriers WHERE id=? FOR SHARE", in.CarrierID).Scan(&enabled); err != nil {
				return nil, "", err
			}
			if !enabled {
				return nil, "", fault(422, "VALIDATION_FAILED")
			}
			state = "READY"
			job.Summary = jobSummary{Processed: 1, Errors: []RowError{}, CarrierID: in.CarrierID, Tracking: in.Tracking}
		} else {
			job.Summary = jobSummary{Failed: 1, Errors: []RowError{{Row: 1, Code: in.Resolution, Field: "provider"}}}
		}
		raw, err := json.Marshal(job.Summary)
		if err != nil {
			return nil, "", err
		}
		_, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET state=?,result_summary=?,lease_until=NULL,version=version+1 WHERE id=?", state, raw, id)
		if err != nil {
			return nil, "", err
		}
		job, err = loadJob(ctx, tx, id, false)
		return job.Job, id, err
	})
}
