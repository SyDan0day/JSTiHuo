package operations

import (
	"context"
	"database/sql"
	"time"
)

type Reconcile struct {
	Balanced bool      `json:"balanced"`
	Products []string  `json:"product_ids_with_drift"`
	Checked  time.Time `json:"checked_at"`
}

// Reconcile 用途：以一致快照核对实存、预占、流水总量与活动预占。
// 参数：ctx 上下文。
// 返回值：是否平衡、最多一百个漂移商品及检查时间或数据库错误。
// 约束：只报告差异，不自动补账或覆盖余额；平衡结果覆盖全部商品。
func (s *Service) Reconcile(ctx context.Context) (Reconcile, error) {
	out := Reconcile{Balanced: true, Products: []string{}, Checked: s.Now().UTC()}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	ids, err := stringRows(ctx, tx, `SELECT p.id FROM products p
		LEFT JOIN inventory_balances b ON b.product_id=p.id
		LEFT JOIN (SELECT product_id,SUM(on_hand_delta) on_hand,SUM(reserved_delta) reserved FROM inventory_transactions GROUP BY product_id) l ON l.product_id=p.id
		LEFT JOIN (SELECT product_id,SUM(quantity) reserved FROM inventory_reservations WHERE state='ACTIVE' GROUP BY product_id) r ON r.product_id=p.id
		WHERE b.product_id IS NULL OR b.on_hand<>COALESCE(l.on_hand,0) OR b.reserved<>COALESCE(l.reserved,0)
		OR b.reserved<>COALESCE(r.reserved,0) OR b.on_hand<b.reserved OR b.reserved<0
		ORDER BY p.id LIMIT 101`)
	if err != nil {
		return out, err
	}
	out.Balanced = len(ids) == 0
	if len(ids) > 100 {
		ids = ids[:100]
	}
	out.Products = ids
	return out, tx.Commit()
}
