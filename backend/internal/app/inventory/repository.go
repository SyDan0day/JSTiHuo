package inventory

import (
	"context"
	"database/sql"
	"fmt"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Service) balance(ctx context.Context, q queryer, id string, lock bool) (Balance, error) {
	b := Balance{ProductID: id}
	sqlText := "SELECT version,on_hand,reserved,warning_level FROM inventory_balances WHERE product_id=?"
	if lock {
		sqlText += " FOR UPDATE"
	}
	e := q.QueryRowContext(ctx, sqlText, id).Scan(&b.Version, &b.OnHand, &b.Reserved, &b.Warning)
	if e == sql.ErrNoRows {
		return b, fault(404, "NOT_FOUND")
	}
	b.Available = b.OnHand - b.Reserved
	return b, e
}
func (s *Service) document(ctx context.Context, q queryer, id string, lock bool) (Document, error) {
	d := Document{ID: id}
	var proof sql.NullString
	sqlText := "SELECT version,document_no,state,direction,stock_kind,note,proof_file_id FROM inventory_documents WHERE id=?"
	if lock {
		sqlText += " FOR UPDATE"
	}
	e := q.QueryRowContext(ctx, sqlText, id).Scan(&d.Version, &d.Number, &d.State, &d.Direction, &d.Kind, &d.Note, &proof)
	if e == sql.ErrNoRows {
		return d, fault(404, "NOT_FOUND")
	}
	if e != nil {
		return d, e
	}
	d.Proof = proof.String
	d.Items = []Line{}
	rows, e := q.QueryContext(ctx, "SELECT product_id,quantity FROM inventory_document_items WHERE document_id=? ORDER BY product_id", id)
	if e != nil {
		return d, e
	}
	defer rows.Close()
	for rows.Next() {
		var l Line
		if e = rows.Scan(&l.ProductID, &l.Quantity); e != nil {
			return d, e
		}
		d.Items = append(d.Items, l)
	}
	return d, rows.Err()
}
func (s *Service) Balances(ctx context.Context, f Filter) (Page[Balance], error) {
	p := Page[Balance]{Items: []Balance{}}
	sqlText := "SELECT b.product_id,b.version,b.on_hand,b.reserved,b.warning_level FROM inventory_balances b JOIN products p ON p.id=b.product_id WHERE p.deleted_at IS NULL"
	args := []any{}
	if f.After > 0 {
		sqlText += " AND b.product_id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		sqlText += " AND (LOCATE(?,p.name)>0 OR LOCATE(?,p.code)>0)"
		args = append(args, f.Q, f.Q)
	}
	if f.Status == "LOW" {
		sqlText += " AND b.on_hand-b.reserved<=b.warning_level"
	}
	sqlText += " ORDER BY b.product_id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := s.DB.QueryContext(ctx, sqlText, args...)
	if e != nil {
		return p, e
	}
	defer rows.Close()
	for rows.Next() {
		var b Balance
		if e = rows.Scan(&b.ProductID, &b.Version, &b.OnHand, &b.Reserved, &b.Warning); e != nil {
			return p, e
		}
		b.Available = b.OnHand - b.Reserved
		p.Items = append(p.Items, b)
	}
	if len(p.Items) > f.Limit {
		p.Items = p.Items[:f.Limit]
		p.Next = p.Items[f.Limit-1].ProductID
	}
	return p, rows.Err()
}
func (s *Service) Documents(ctx context.Context, f Filter) (Page[Document], error) {
	p := Page[Document]{Items: []Document{}}
	sqlText := "SELECT id FROM inventory_documents WHERE 1=1"
	args := []any{}
	if f.After > 0 {
		sqlText += " AND id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		sqlText += " AND (LOCATE(?,document_no)>0 OR LOCATE(?,note)>0)"
		args = append(args, f.Q, f.Q)
	}
	if f.Status != "" {
		sqlText += " AND state=?"
		args = append(args, f.Status)
	}
	if f.Direction != "" {
		sqlText += " AND direction=?"
		args = append(args, f.Direction)
	}
	sqlText += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := s.DB.QueryContext(ctx, sqlText, args...)
	if e != nil {
		return p, e
	}
	ids := []string{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return p, e
		}
		ids = append(ids, fmt.Sprint(id))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	if len(ids) > f.Limit {
		ids = ids[:f.Limit]
		p.Next = ids[f.Limit-1]
	}
	for _, id := range ids {
		d, e := s.document(ctx, s.DB, id, false)
		if e != nil {
			return p, e
		}
		p.Items = append(p.Items, d)
	}
	return p, nil
}
func (s *Service) Transactions(ctx context.Context, f Filter) (Page[Ledger], error) {
	p := Page[Ledger]{Items: []Ledger{}}
	sqlText := "SELECT id,product_id,on_hand_delta,reserved_delta,event_key,reason,created_at FROM inventory_transactions WHERE 1=1"
	args := []any{}
	if f.After > 0 {
		sqlText += " AND id<?"
		args = append(args, f.After)
	}
	if f.Product != "" {
		sqlText += " AND product_id=?"
		args = append(args, f.Product)
	}
	if f.Q != "" {
		sqlText += " AND (LOCATE(?,event_key)>0 OR LOCATE(?,reason)>0)"
		args = append(args, f.Q, f.Q)
	}
	sqlText += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := s.DB.QueryContext(ctx, sqlText, args...)
	if e != nil {
		return p, e
	}
	defer rows.Close()
	for rows.Next() {
		var l Ledger
		if e = rows.Scan(&l.ID, &l.ProductID, &l.Delta, &l.ReservedDelta, &l.Event, &l.Reason, &l.Created); e != nil {
			return p, e
		}
		p.Items = append(p.Items, l)
	}
	if len(p.Items) > f.Limit {
		p.Items = p.Items[:f.Limit]
		p.Next = p.Items[f.Limit-1].ID
	}
	return p, rows.Err()
}
