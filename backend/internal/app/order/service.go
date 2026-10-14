package order

import (
	"context"
	"database/sql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"time"
)

type Service struct {
	DB         *sql.DB
	Auth       *identity.Service
	Redemption *redemption.Service
	FaultTx    func(context.Context, *sql.Tx, string) error
	Now        func() time.Time
}
type Filter struct {
	Limit     int
	After     string
	Q, Status string
	Deleted   bool
}
type Page struct {
	Items []redemption.Order `json:"items"`
	Next  string             `json:"next_after_id,omitempty"`
}

// New 用途：装配订单服务并复用成交快照读取。
// 参数：db 业务数据库；auth 身份服务依赖。
// 参数：redeem 兑换服务依赖。
// 返回值：服务指针。
// 约束：不连接供应商，不改订单状态。
func New(db *sql.DB, auth *identity.Service, redeem *redemption.Service) *Service {
	return &Service{DB: db, Auth: auth, Redemption: redeem, Now: redeem.Now}
}

// List 用途：列出后台授权范围内的脱敏订单。
// 参数：ctx 请求上下文；f 已校验列表筛选条件。
// 返回值：订单页或 SQL、解密错误。
// 约束：不查询手机号或详细地址，关闭 rows 后再加载详情避免连接池嵌套占用。
func (s *Service) List(ctx context.Context, f Filter) (Page, error) {
	query := "SELECT id FROM orders WHERE (deleted_at IS NOT NULL)=?"
	args := []any{f.Deleted}
	if f.After != "" {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Status != "" {
		query += " AND status=?"
		args = append(args, f.Status)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,order_no)>0"
		args = append(args, f.Q)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := s.DB.QueryContext(ctx, query, args...)
	if e != nil {
		return Page{}, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return Page{}, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return Page{}, e
	}
	out := Page{Items: []redemption.Order{}}
	if len(ids) > f.Limit {
		ids = ids[:f.Limit]
		out.Next = ids[len(ids)-1]
	}
	for _, id := range ids {
		o, e := s.Redemption.GetOrder(ctx, id, true)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, o)
	}
	return out, nil
}
