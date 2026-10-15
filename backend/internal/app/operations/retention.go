package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"jstihuo/internal/app/redemption"
)

// RetireCompletedPII 用途：匿名化超过保留期且无活动售后的已完成订单。
// 参数：ctx 工作上下文；days 保留天数，至少 180 天；limit 单轮最多处理 1000 单。
// 返回值：已提交条数或错误；失败订单整体回滚，已提交订单不重复处理。
// 事务：逐单持锁，清除手机索引和收件资料、撤销本单会话并保留审计及账本。
func (s *Service) RetireCompletedPII(ctx context.Context, days, limit int) (int, error) {
	if days < 180 || days > 36500 || limit < 1 || limit > 1000 {
		return 0, errors.New("RETENTION_CONFIGURATION_INVALID")
	}
	cutoff := s.Now().UTC().AddDate(0, 0, -days)
	for count := 0; count < limit; count++ {
		changed, err := s.retireOrderPII(ctx, cutoff)
		if err != nil || !changed {
			return count, err
		}
	}
	return limit, nil
}

// retireOrderPII 用途：在订单锁内确认保留期并原子匿名化一单。
// 参数：ctx 上下文；cutoff 最晚允许清理的完成时间，等于边界时保留。
// 返回值：是否完成清理及事务错误；零手机索引标记不可再按手机号查找。
func (s *Service) retireOrderPII(ctx context.Context, cutoff time.Time) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id, number string
	err = tx.QueryRowContext(ctx, `SELECT o.id,o.order_no FROM orders o
		WHERE o.status='COMPLETED' AND o.completed_at<?
		AND o.phone_lookup<>UNHEX(REPEAT('00',32))
		AND NOT EXISTS (SELECT 1 FROM after_sales a WHERE a.order_id=o.id AND a.state IN ('OPEN','APPROVED'))
		ORDER BY o.id LIMIT 1 FOR UPDATE SKIP LOCKED`, cutoff).Scan(&id, &number)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// 外层取得订单锁后重新读取，避免子查询快照先于并发案件提交。
	var active int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM after_sales WHERE order_id=? AND state IN ('OPEN','APPROVED')", id).Scan(&active)
	if err != nil || active > 0 {
		return false, err
	}
	plain, err := json.Marshal(redemption.RecipientData{})
	if err != nil {
		return false, err
	}
	defer clear(plain)
	cipher, version, err := s.PII.Encrypt(plain, redemption.OrderAAD(number))
	if err != nil {
		return false, err
	}
	statements := []struct {
		query string
		args  []any
	}{
		{"DELETE c FROM challenges c JOIN sessions s ON s.id=c.session_id WHERE s.order_id=?", []any{id}},
		{"DELETE FROM sessions WHERE order_id=?", []any{id}},
		{"UPDATE orders SET recipient_cipher=?,pii_key_version=?,phone_lookup=UNHEX(REPEAT('00',32)),customer_note='',admin_note='',version=version+1 WHERE id=?", []any{cipher, version, id}},
		{"INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','order.pii_retired','order',?,'retention-worker',JSON_OBJECT(),'SUCCESS')", []any{id}},
	}
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}
