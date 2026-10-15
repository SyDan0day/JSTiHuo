package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"errors"
	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/voucher"
	imports "jstihuo/internal/domain/imports"
	rules "jstihuo/internal/domain/voucher"
)

type importBatch struct {
	State                  string
	Quantity, SecretLength int
	SecretFormat           string
	Activate               bool
	Policy                 voucher.Policy
}

type importedVoucher struct {
	Code, Secret, Hash string
	From, Until        time.Time
}

// readImportBatch 用途：读取导入所需的冻结权益及密码格式。
// 参数：ctx 上下文；q 数据源；id 批次 ID；lock 是否排他锁定。
// 返回值：草稿配置或不存在、非草稿、快照及数据库错误。
func readImportBatch(ctx context.Context, q queryer, id string, lock bool) (importBatch, error) {
	var b importBatch
	var raw []byte
	query := "SELECT state,quantity,secret_length,secret_format,activate_on_seal,policy_snapshot FROM voucher_batches WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&b.State, &b.Quantity, &b.SecretLength, &b.SecretFormat, &b.Activate, &raw)
	if err == sql.ErrNoRows {
		return b, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return b, err
	}
	if b.State != "DRAFT" {
		return b, fault(409, "ILLEGAL_TRANSITION")
	}
	if err = json.Unmarshal(raw, &b.Policy); err != nil {
		return b, err
	}
	if !rules.ValidSelection(b.Policy.Mode, b.Policy.Count, len(b.Policy.Products)) {
		return b, fault(422, "VALIDATION_FAILED")
	}
	return b, nil
}

// parseVoucherRow 用途：验证一行卡券的规范卡号、原样密码和半开有效期。
// 参数：row CSV 行；batch 批次密码约束。
// 返回值：规范化卡券与可选脱敏错误，不在错误中回显字段值。
func parseVoucherRow(row imports.Row, batch importBatch) (importedVoucher, *RowError) {
	values := row.Values
	out := importedVoucher{Code: strings.ToUpper(strings.TrimSpace(values[0])), Secret: values[1]}
	bad := func(field, code string) (importedVoucher, *RowError) {
		return out, &RowError{Row: row.Number, Code: code, Field: field}
	}
	if !regexp.MustCompile(`^[A-Z0-9]{12,24}$`).MatchString(out.Code) {
		return bad("code", "INVALID_CODE")
	}
	if !rules.ValidSecret(len(out.Secret), batch.SecretFormat) || len(out.Secret) != batch.SecretLength {
		return bad("secret", "INVALID_SECRET_FORMAT")
	}
	alphabet := rules.Alphabet(batch.SecretFormat)
	for _, value := range out.Secret {
		if !strings.ContainsRune(alphabet, value) {
			return bad("secret", "INVALID_SECRET_FORMAT")
		}
	}
	from, e1 := time.Parse(time.RFC3339Nano, values[2])
	until, e2 := time.Parse(time.RFC3339Nano, values[3])
	if e1 != nil || from.Year() < 1000 || from.Year() > 9999 {
		return bad("valid_from", "INVALID_DATE")
	}
	if e2 != nil || until.Year() < 1000 || until.Year() > 9999 {
		return bad("valid_until", "INVALID_DATE")
	}
	out.From, out.Until = from.UTC().Truncate(time.Microsecond), until.UTC().Truncate(time.Microsecond)
	if !out.From.Before(out.Until) {
		return bad("valid_until", "INVALID_DATE_RANGE")
	}
	return out, nil
}

// validateVoucherRows 用途：预检卡券行及全局卡号唯一性，不创建卡。
// 参数：ctx 上下文；rows CSV 记录；batch 草稿快照。
// 返回值：规范化卡券、逐行脱敏错误或数据库错误。
func (s *Service) validateVoucherRows(ctx context.Context, rows []imports.Row, batch importBatch) ([]importedVoucher, []RowError, error) {
	items, errs := []importedVoucher{}, []RowError{}
	seen := map[string]bool{}
	if len(rows) != batch.Quantity {
		return nil, []RowError{{Row: 1, Code: "BATCH_QUANTITY_MISMATCH", Field: "row"}}, nil
	}
	for _, row := range rows {
		item, invalid := parseVoucherRow(row, batch)
		if invalid != nil {
			errs = append(errs, *invalid)
			continue
		}
		if seen[item.Code] {
			errs = append(errs, RowError{Row: row.Number, Code: "DUPLICATE_CODE", Field: "code"})
			continue
		}
		seen[item.Code] = true
		var count int
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM vouchers WHERE code=?", item.Code).Scan(&count); err != nil {
			return nil, nil, err
		}
		if count != 0 {
			errs = append(errs, RowError{Row: row.Number, Code: "DUPLICATE_CODE", Field: "code"})
			continue
		}
		items = append(items, item)
	}
	return items, errs, nil
}

// processVoucherImport 用途：预检或整批原子导入卡券，在事务外计算 Argon2id。
// 参数：ctx 上下文；claim 领取凭证；actor 经复查的任务管理员；rows CSV 记录。
// 返回值：任务完成、租约失效或数据库错误；任一卡号冲突不能留下部分卡券。
func (s *Service) processVoucherImport(ctx context.Context, claim JobClaim, actor identity.Actor, rows []imports.Row) error {
	batch, err := readImportBatch(ctx, s.DB, claim.job.Options.BatchID, false)
	if err != nil {
		return err
	}
	items, errs, err := s.validateVoucherRows(ctx, rows, batch)
	if err != nil {
		return err
	}
	defer func() {
		for i := range items {
			items[i].Secret = ""
			items[i].Hash = ""
		}
	}()
	summary := jobSummary{Processed: len(rows), Failed: len(errs), Errors: errs}
	if claim.job.Phase == "PREVIEW" {
		return s.finishImport(ctx, claim, summary)
	}
	if len(errs) > 0 {
		return s.finishInvalidVoucherImport(ctx, claim, summary)
	}
	for i := range items {
		if err = s.RenewJob(ctx, claim); err != nil {
			return err
		}
		items[i].Hash, err = s.Hash(items[i].Secret)
		items[i].Secret = ""
		if err != nil {
			return err
		}
	}
	err = s.applyVoucherImport(ctx, claim, actor, items)
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return fault(409, "DUPLICATE_CODE")
	}
	return err
}

// finishInvalidVoucherImport 用途：保存整批拒绝结果并销毁已确认导入的源文件引用。
// 参数：ctx 上下文；claim 领取凭证；summary 全批验证错误。
// 返回值：结果文件或数据库错误，不创建任何卡。
func (s *Service) finishInvalidVoucherImport(ctx context.Context, claim JobClaim, summary jobSummary) error {
	report, err := s.errorReport(summary)
	if err != nil {
		return err
	}
	defer s.cleanupUnreferenced(ctx, report.Storage)
	return s.finishJob(ctx, claim, "FAILED", summary, &report)
}

// applyVoucherImport 用途：以任务栅栏和批次锁原子保存全部哈希与冻结权益。
// 参数：ctx 上下文；claim 有效领取凭证；actor 来源管理员；items 已验证并哈希的卡券。
// 返回值：提交、权限、状态、唯一约束或租约错误。
// 事务：全部卡券、批次 SEALED、源文件过期、任务终态与审计一起提交。
func (s *Service) applyVoucherImport(ctx context.Context, claim JobClaim, actor identity.Actor, items []importedVoucher) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	job, err := s.lockClaim(ctx, tx, claim)
	if err != nil {
		return err
	}
	if err = identity.AuthorizeAdmin(ctx, tx, actor, "file.import"); err != nil {
		return err
	}
	batch, err := readImportBatch(ctx, tx, job.Options.BatchID, true)
	if err != nil {
		return err
	}
	if len(items) != batch.Quantity {
		return fault(409, "VERSION_CONFLICT")
	}
	for index, item := range items {
		if err = ctx.Err(); err != nil {
			return err
		}
		if !job.Lease.Time.After(s.Now()) {
			return ErrJobLeaseLost
		}
		if err = insertImportedVoucher(ctx, tx, job.Options.BatchID, batch, item); err != nil {
			return err
		}
		if err = s.inject(ctx, tx, fmt.Sprintf("voucher:%d", index+1)); err != nil {
			return err
		}
		job.Lease.Time = s.Now().UTC().Add(time.Minute)
		if _, err = tx.ExecContext(ctx, "UPDATE integration_jobs SET lease_until=? WHERE id=?", job.Lease.Time, job.ID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE voucher_batches SET state='SEALED',version=version+1,lease_until=NULL,last_error_code=NULL WHERE id=?", job.Options.BatchID)
	if err != nil {
		return err
	}
	if err = s.completeJobTx(ctx, tx, job, "READY", jobSummary{Processed: len(items), Errors: []RowError{}}, nil); err != nil {
		return err
	}
	return tx.Commit()
}

// insertImportedVoucher 用途：插入一张封存卡及不可变商品集合。
// 参数：ctx 上下文；tx 当前事务；batchID 批次；batch 权益及激活配置；item 哈希卡券。
// 返回值：SQL 错误；不保存原密码或提供重新导出。
func insertImportedVoucher(ctx context.Context, tx *sql.Tx, batchID string, batch importBatch, item importedVoucher) error {
	r, err := tx.ExecContext(ctx, `INSERT INTO vouchers(batch_id,code,secret_hash,active,credential_state,valid_from,valid_until)
		VALUES(?,?,?,?,'SEALED',?,?)`, batchID, item.Code, item.Hash, batch.Activate, item.From, item.Until)
	if err != nil {
		return err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return err
	}
	for _, product := range batch.Policy.Products {
		if _, err = tx.ExecContext(ctx, "INSERT INTO voucher_products(voucher_id,product_id) VALUES(?,?)", id, product); err != nil {
			return err
		}
	}
	return nil
}
