package voucher

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// getType 用途：读取不可变卡型版本。
// 参数：ctx 请求上下文；q 数据库或当前事务查询接口。
// 参数：id 目标对象标识。
// 返回值：卡型或不存在、SQL 错误。
// 约束：不修改卡型。
func (s *Service) getType(ctx context.Context, q queryer, id string) (Type, error) {
	var t Type
	e := q.QueryRowContext(ctx, "SELECT id,version,revision,name,selection_mode,selection_count,enabled FROM voucher_types WHERE id=?", id).Scan(&t.ID, &t.Version, &t.Revision, &t.Name, &t.Mode, &t.Count, &t.Enabled)
	if e == sql.ErrNoRows {
		e = fault(404, "NOT_FOUND")
	}
	return t, e
}

// batch 用途：读取批次配置、冻结商品与已生成数量。
// 参数：ctx 请求上下文；q 数据库或当前事务查询接口。
// 参数：id 目标对象标识。
// 返回值：批次或不存在、SQL、JSON 错误。
// 约束：不改变生成任务。
func (s *Service) batch(ctx context.Context, q queryer, id string) (Batch, error) {
	var b Batch
	var raw []byte
	var start, file sql.NullString
	e := q.QueryRowContext(ctx, "SELECT id,version,batch_no,state,type_id,label,policy_snapshot,quantity,code_prefix,code_length,code_mode,start_serial,secret_length,secret_format,valid_from,valid_until,activate_on_seal,export_file_id,(SELECT COUNT(*) FROM vouchers WHERE batch_id=vb.id) FROM voucher_batches vb WHERE id=?", id).Scan(&b.ID, &b.Version, &b.Number, &b.State, &b.TypeID, &b.Label, &raw, &b.Quantity, &b.Prefix, &b.CodeLength, &b.CodeMode, &start, &b.SecretLength, &b.SecretFormat, &b.From, &b.Until, &b.Activate, &file, &b.Generated)
	if e == sql.ErrNoRows {
		return b, fault(404, "NOT_FOUND")
	}
	if e != nil {
		return b, e
	}
	var p Policy
	if e = json.Unmarshal(raw, &p); e != nil {
		return b, e
	}
	b.Products = p.Products
	b.Start = start.String
	b.FileID = file.String
	return b, nil
}

// GetBatch 用途：查询批次权威结果。
// 参数：ctx 请求上下文；id 目标对象标识。
// 返回值：批次或读取错误。
func (s *Service) GetBatch(ctx context.Context, id string) (Batch, error) {
	return s.batch(ctx, s.DB, id)
}

// GetVoucher 用途：查询未删除卡券及冻结商品集合。
// 参数：ctx 请求上下文；id 目标对象标识。
// 返回值：卡券或不存在、SQL 错误。
// 约束：查询结束关闭 rows，不返回秘密。
func (s *Service) GetVoucher(ctx context.Context, id string) (Voucher, error) {
	var v Voucher
	e := s.DB.QueryRowContext(ctx, "SELECT id,version,batch_id,code,active,credential_state,valid_from,valid_until,redeemed_at IS NOT NULL FROM vouchers WHERE id=? AND deleted_at IS NULL", id).Scan(&v.ID, &v.Version, &v.BatchID, &v.Code, &v.Active, &v.State, &v.From, &v.Until, &v.Redeemed)
	if e == sql.ErrNoRows {
		return v, fault(404, "NOT_FOUND")
	}
	if e != nil {
		return v, e
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT product_id FROM voucher_products WHERE voucher_id=? ORDER BY product_id", id)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	v.Products = []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return v, e
		}
		v.Products = append(v.Products, id)
	}
	return v, rows.Err()
}

// ids 用途：按固定资源白名单查询分页标识。
// 参数：ctx 请求上下文；kind 会话或资源类型。
// 参数：f 已校验列表筛选条件。
// 返回值：标识列表或 SQL 错误。
// 约束：用户值均绑定参数，固定 id DESC。
func (s *Service) ids(ctx context.Context, kind string, f Filter) ([]string, error) {
	table, where, args := "voucher_batches", "1=1", []any{}
	switch kind {
	case "types":
		table = "voucher_types"
		if f.Q != "" {
			where += " AND name LIKE ?"
			args = append(args, "%"+f.Q+"%")
		}
		if f.Status != "" {
			where += " AND enabled=?"
			args = append(args, f.Status == "ENABLED")
		}
	case "batches":
		if f.Q != "" {
			where += " AND (batch_no LIKE ? OR label LIKE ?)"
			args = append(args, "%"+f.Q+"%", "%"+f.Q+"%")
		}
		if f.Status != "" {
			where += " AND state=?"
			args = append(args, f.Status)
		}
	case "vouchers":
		table = "vouchers"
		where = "deleted_at IS NULL"
		if f.Q != "" {
			where += " AND code LIKE ?"
			args = append(args, "%"+strings.ToUpper(f.Q)+"%")
		}
		if f.Batch != "" {
			where += " AND batch_id=?"
			args = append(args, f.Batch)
		}
		if f.Active != nil {
			where += " AND active=?"
			args = append(args, *f.Active)
		}
		if f.Redeemed != nil {
			where += " AND (redeemed_at IS NOT NULL)=?"
			args = append(args, *f.Redeemed)
		}
	}
	if f.After != "" {
		where += " AND id<?"
		args = append(args, f.After)
	}
	args = append(args, f.Limit+1)
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM "+table+" WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// pageIDs 用途：截取一页并确定排他游标。
// 参数：ids 已排序对象标识；limit 本页长度。
// 返回值：本页标识和可空下一页游标。
func pageIDs(ids []string, limit int) ([]string, string) {
	if len(ids) > limit {
		return ids[:limit], ids[limit-1]
	}
	return ids, ""
}

// Types 用途：分页列出卡型版本。
// 参数：ctx 请求上下文；f 已校验列表筛选条件。
// 返回值：卡型页或读取错误。
// 约束：固定游标排序，关闭查询资源。
func (s *Service) Types(ctx context.Context, f Filter) (Page[Type], error) {
	out := Page[Type]{Items: []Type{}}
	ids, e := s.ids(ctx, "types", f)
	if e != nil {
		return out, e
	}
	ids, out.Next = pageIDs(ids, f.Limit)
	for _, id := range ids {
		v, e := s.getType(ctx, s.DB, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// Batches 用途：分页列出批次权威状态。
// 参数：ctx 请求上下文；f 已校验列表筛选条件。
// 返回值：批次页或读取错误。
func (s *Service) Batches(ctx context.Context, f Filter) (Page[Batch], error) {
	out := Page[Batch]{Items: []Batch{}}
	ids, e := s.ids(ctx, "batches", f)
	if e != nil {
		return out, e
	}
	ids, out.Next = pageIDs(ids, f.Limit)
	for _, id := range ids {
		v, e := s.GetBatch(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// Vouchers 用途：分页列出卡券。
// 参数：ctx 请求上下文；f 已校验列表筛选条件。
// 返回值：卡券页或读取错误。
func (s *Service) Vouchers(ctx context.Context, f Filter) (Page[Voucher], error) {
	out := Page[Voucher]{Items: []Voucher{}}
	ids, e := s.ids(ctx, "vouchers", f)
	if e != nil {
		return out, e
	}
	ids, out.Next = pageIDs(ids, f.Limit)
	for _, id := range ids {
		v, e := s.GetVoucher(ctx, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}
