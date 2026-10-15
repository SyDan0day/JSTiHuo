package operations

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"jstihuo/internal/app/identity"
)

type Dashboard struct {
	Products  int64 `json:"products"`
	Vouchers  int64 `json:"vouchers"`
	Pending   int64 `json:"pending_orders"`
	Shipped   int64 `json:"shipped_orders"`
	Completed int64 `json:"completed_orders"`
}
type Audit struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Entity    string    `json:"entity_type"`
	EntityID  string    `json:"entity_id,omitempty"`
	Result    string    `json:"result"`
	RequestID string    `json:"request_id"`
	Created   time.Time `json:"created_at"`
}
type AuditFilter struct {
	Filter
	Entity, EntityID string
}

// Dashboard 用途：读取同一数据库快照中的目录、卡券与订单总数。
// 参数：ctx 上下文。
// 返回值：未软删业务对象统计或数据库错误；不含收件资料。
func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	var out Dashboard
	err := s.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM products WHERE deleted_at IS NULL),
		(SELECT COUNT(*) FROM vouchers WHERE deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders WHERE status='PENDING' AND deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders WHERE status='SHIPPED' AND deleted_at IS NULL),
		(SELECT COUNT(*) FROM orders WHERE status='COMPLETED' AND deleted_at IS NULL)`).Scan(&out.Products, &out.Vouchers, &out.Pending, &out.Shipped, &out.Completed)
	return out, err
}

// Audits 用途：按固定游标、动作搜索和对象引用查询只读审计。
// 参数：ctx 上下文；f 服务器已校验的筛选。
// 返回值：审计页或数据库错误，不返回 diff、联系人与请求正文。
func (s *Service) Audits(ctx context.Context, f AuditFilter) (Page[Audit], error) {
	out := Page[Audit]{Items: []Audit{}}
	query := "SELECT id,action,entity_type,entity_id,result,request_id,created_at FROM audit_logs WHERE 1=1"
	args := []any{}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,action)>0"
		args = append(args, f.Q)
	}
	if f.Status != "" {
		query += " AND result=?"
		args = append(args, f.Status)
	}
	if f.Entity != "" {
		query += " AND LOWER(entity_type)=?"
		args = append(args, strings.ToLower(f.Entity))
	}
	if f.EntityID != "" {
		query += " AND entity_id=?"
		args = append(args, f.EntityID)
	}
	args = append(args, f.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Audit
		var entityID sql.NullString
		if err = rows.Scan(&item.ID, &item.Action, &item.Entity, &entityID, &item.Result, &item.RequestID, &item.Created); err != nil {
			return out, err
		}
		item.EntityID = entityID.String
		out.Items = append(out.Items, item)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	return out, rows.Err()
}

// Files 用途：分页列出授权可见文件元数据。
// 参数：ctx 上下文；a 管理员；f 状态与用途筛选。
// 返回值：文件页或数据库错误；不返回存储路径、摘要或原始文件名。
// 约束：私有文件缺失所有者不展示，文件导出管理员可管理有明确所有者的文件。
func (s *Service) Files(ctx context.Context, a identity.Actor, f Filter) (Page[File], error) {
	out := Page[File]{Items: []File{}}
	query := "SELECT id,kind,state,mime_type,size_bytes,expires_at FROM files WHERE (kind='IMAGE' OR owner_admin_id=?)"
	args := []any{a.AdminID}
	if a.Can("file.export") {
		query = "SELECT id,kind,state,mime_type,size_bytes,expires_at FROM files WHERE (kind='IMAGE' OR owner_admin_id IS NOT NULL)"
		args = []any{}
	}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND state=?"
		args = append(args, f.Status)
	}
	if f.Q != "" {
		query += " AND kind=?"
		args = append(args, f.Q)
	}
	args = append(args, f.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var file File
		var expiry sql.NullTime
		if err = rows.Scan(&file.ID, &file.Kind, &file.State, &file.MIME, &file.Size, &expiry); err != nil {
			return out, err
		}
		if expiry.Valid {
			file.ExpiresAt = &expiry.Time
		}
		out.Items = append(out.Items, file)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	return out, rows.Err()
}
