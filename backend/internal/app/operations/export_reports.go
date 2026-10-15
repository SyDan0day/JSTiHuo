package operations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	imports "jstihuo/internal/domain/imports"
)

type boundedCSV struct{ bytes.Buffer }

// Write 用途：限制明文 CSV 的内存大小。
// 参数：p 本次待写入字节。
// 返回值：写入字节数或超出十 MiB 的明确错误。
func (b *boundedCSV) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 10*1024*1024 {
		return 0, fault(422, "EXPORT_LIMIT_EXCEEDED")
	}
	return b.Buffer.Write(p)
}

// writeCSVRow 用途：按 RFC4180 写入已白名单选取的脱敏字段。
// 参数：writer CSV 编码器；values 脱敏值。
// 返回值：编码或大小上限错误。
func writeCSVRow(writer *csv.Writer, values []string) error {
	for i := range values {
		values[i] = imports.SafeCell(values[i])
	}
	if err := writer.Write(values); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

// processExport 用途：在一致快照中分页导出至多十万行脱敏元数据。
// 参数：ctx 上下文；claim 领取凭证；actor 已授权来源管理员。
// 返回值：短期加密报表的任务结果或权限、大小、租约与数据库错误。
// 约束：只在内存持有明文，禁止选择详细地址、秘密、电话原文或任意列。
func (s *Service) processExport(ctx context.Context, claim JobClaim, actor identity.Actor) error {
	if claim.job.Options.Export == nil {
		return fault(422, "INVALID_ARGUMENT")
	}
	in := *claim.job.Options.Export
	var data boundedCSV
	defer func() { clear(data.Bytes()) }()
	writer := csv.NewWriter(&data)
	headers := []string{"masked_code", "batch_no", "credential_state", "active", "redeemed", "valid_from", "valid_until"}
	if in.Kind == "ORDERS" {
		headers = []string{"order_no", "status", "ship_date", "recipient_name", "recipient_phone", "region"}
	}
	if err := writeCSVRow(writer, headers); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	after, count := "0", 0
	for {
		if err = s.RenewJob(ctx, claim); err != nil {
			return err
		}
		var records [][]string
		if in.Kind == "ORDERS" {
			records, after, err = s.exportOrderPage(ctx, tx, in, after)
		} else {
			records, after, err = s.exportVoucherPage(ctx, tx, in, after)
		}
		if err != nil {
			return err
		}
		for _, record := range records {
			count++
			if count > 100000 {
				return fault(422, "EXPORT_LIMIT_EXCEEDED")
			}
			if err = writeCSVRow(writer, record); err != nil {
				return err
			}
		}
		if len(records) < 1000 {
			break
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	report, err := s.writePrivate("REPORT", data.Bytes())
	if err != nil {
		return err
	}
	defer s.cleanupUnreferenced(ctx, report.Storage)
	return s.finishJob(ctx, claim, "READY", jobSummary{Processed: count, Errors: []RowError{}}, &report)
}

// exportVoucherPage 用途：查询一页不含密码哈希的卡券元数据并掩码卡号。
// 参数：ctx 上下文；tx 只读一致快照；in 筛选；after 排他升序游标。
// 返回值：白名单 CSV 行、最后 ID 或数据库错误。
func (s *Service) exportVoucherPage(ctx context.Context, tx *sql.Tx, in ExportInput, after string) ([][]string, string, error) {
	query := `SELECT v.id,v.code,b.batch_no,v.credential_state,v.active,v.redeemed_at IS NOT NULL,v.valid_from,v.valid_until
		FROM vouchers v JOIN voucher_batches b ON b.id=v.batch_id WHERE v.id>? AND v.deleted_at IS NULL`
	args := []any{after}
	if in.BatchID != "" {
		query += " AND v.batch_id=?"
		args = append(args, in.BatchID)
	}
	if in.Status != "" {
		query += " AND v.credential_state=?"
		args = append(args, in.Status)
	}
	rows, err := tx.QueryContext(ctx, query+" ORDER BY v.id LIMIT 1000", args...)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	result := [][]string{}
	for rows.Next() {
		var id, code, batch, state string
		var active, redeemed bool
		var from, until time.Time
		if err = rows.Scan(&id, &code, &batch, &state, &active, &redeemed, &from, &until); err != nil {
			return nil, after, err
		}
		masked := "****"
		if len(code) >= 8 {
			masked = code[:4] + "****" + code[len(code)-4:]
		}
		result = append(result, []string{masked, batch, state, strconv.FormatBool(active), strconv.FormatBool(redeemed), from.UTC().Format(time.RFC3339Nano), until.UTC().Format(time.RFC3339Nano)})
		after = id
	}
	return result, after, rows.Err()
}

// exportOrderPage 用途：查询一页订单并仅输出脱敏收件摘要。
// 参数：ctx 上下文；tx 只读一致快照；in 状态筛选；after 排他升序游标。
// 返回值：白名单 CSV 行、最后 ID 或数据库、密钥错误。
func (s *Service) exportOrderPage(ctx context.Context, tx *sql.Tx, in ExportInput, after string) ([][]string, string, error) {
	query := `SELECT o.id,o.order_no,o.status,c.ship_date,o.recipient_cipher,o.pii_key_version
		FROM orders o LEFT JOIN shipping_calendars c ON c.id=o.calendar_id WHERE o.id>? AND o.deleted_at IS NULL`
	args := []any{after}
	if in.Status != "" {
		query += " AND o.status=?"
		args = append(args, in.Status)
	}
	rows, err := tx.QueryContext(ctx, query+" ORDER BY o.id LIMIT 1000", args...)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	result := [][]string{}
	for rows.Next() {
		var id, number, state string
		var date sql.NullTime
		var cipher []byte
		var version int
		if err = rows.Scan(&id, &number, &state, &date, &cipher, &version); err != nil {
			return nil, after, err
		}
		values, err := s.maskedRecipient(number, cipher, version)
		if err != nil {
			return nil, after, err
		}
		day := ""
		if date.Valid {
			day = date.Time.Format("2006-01-02")
		}
		result = append(result, append([]string{number, state, day}, values...))
		after = id
	}
	return result, after, rows.Err()
}

// maskedRecipient 用途：解密必要收件字段后立即生成不可逆展示摘要。
// 参数：number 订单号 AAD；cipher 联系人密文；version 密钥版本。
// 返回值：掩码姓名、掩码电话与地区，或认证、格式错误。
func (s *Service) maskedRecipient(number string, cipher []byte, version int) ([]string, error) {
	plain, err := s.PII.Decrypt(cipher, redemption.OrderAAD(number), version)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	var recipient redemption.RecipientData
	if err = json.Unmarshal(plain, &recipient); err != nil {
		return nil, err
	}
	name, phone := "", ""
	if runes := []rune(recipient.Name); len(runes) > 0 {
		name = string(runes[0]) + "**"
	}
	if len(recipient.Phone) == 11 {
		phone = recipient.Phone[:3] + "****" + recipient.Phone[7:]
	}
	if name == "" || phone == "" {
		return nil, fmt.Errorf("invalid recipient envelope")
	}
	return []string{name, phone, recipient.Region}, nil
}
