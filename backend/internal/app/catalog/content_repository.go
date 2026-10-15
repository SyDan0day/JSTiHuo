package catalog

import (
	"context"
	"database/sql"
	"strconv"
)

const contentColumns = "id,version,slug,title,body_html,kind,published,sort_order"

// scanContent 用途：解码内容行并统一处理未找到语义。
// 参数：row 数据库单行或结果集的当前行。
// 返回值：内容；无记录返回 404，解码失败返回数据库错误。
func scanContent(row interface{ Scan(...any) error }) (Content, error) {
	var c Content
	err := row.Scan(&c.ID, &c.Version, &c.Slug, &c.Title, &c.Body,
		&c.Kind, &c.Published, &c.Sort)
	if err == sql.ErrNoRows {
		err = fault(404, "NOT_FOUND")
	}
	return c, err
}

// loadContent 用途：读取内容快照，公开读取始终限制发布状态。
// 参数：ctx 上下文；q 数据源；value 后台 ID 或公开 slug；public 公开查询标记。
// 返回值：内容或读取错误。
func loadContent(ctx context.Context, q queryer, value string, public bool) (Content, error) {
	where := "id=?"
	if public {
		where = "slug=? AND published=1"
	}
	return scanContent(q.QueryRowContext(ctx,
		"SELECT "+contentColumns+" FROM content_pages WHERE "+where, value))
}

// listContent 用途：按受控条件读取公开或后台内容分页。
// 参数：ctx 上下文；q 数据源；f 已校验筛选；public 是否限制已发布内容。
// 返回值：最多 f.Limit 项及排他游标，数据库失败返回错误。
func listContent(ctx context.Context, q queryer, f Filter, public bool) (Page[Content], error) {
	out := Page[Content]{Items: []Content{}}
	query, args := "SELECT "+contentColumns+" FROM content_pages WHERE 1=1", []any{}
	if public || f.Status != "" {
		query += " AND published=?"
		args = append(args, public || f.Status == "PUBLISHED")
	}
	if f.After > 0 {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		query += " AND (LOCATE(?,title)>0 OR LOCATE(?,slug)>0)"
		args = append(args, f.Q, f.Q)
	}
	rows, err := q.QueryContext(ctx, query+" ORDER BY id DESC LIMIT ?", append(args, f.Limit+1)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, c)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	return out, rows.Err()
}

// insertContent 用途：在调用方事务中保存完整内容。
// 参数：ctx 上下文；tx 写事务；in 已清洗并校验的内容。
// 返回值：新对象引用或数据库错误，唯一 slug 由数据库约束保证。
func insertContent(ctx context.Context, tx *sql.Tx, in ContentInput) (Ref, error) {
	result, err := tx.ExecContext(ctx,
		"INSERT INTO content_pages(slug,title,body_html,kind,published,sort_order) VALUES(?,?,?,?,?,?)",
		in.Slug, in.Title, in.Body, in.Kind, in.Published, in.Sort)
	if err != nil {
		return Ref{}, err
	}
	id, err := result.LastInsertId()
	return Ref{strconv.FormatInt(id, 10), 1, strconv.FormatBool(in.Published), "content", 201}, err
}

// updateContent 用途：在事务中以乐观锁替换内容。
// 参数：ctx 上下文；tx 写事务；id 内容 ID；in 已校验完整快照与预期版本。
// 返回值：新版本引用；不存在返回 404，版本冲突返回 409。
func updateContent(ctx context.Context, tx *sql.Tx, id string, in ContentUpdate) (Ref, error) {
	var version int64
	err := tx.QueryRowContext(ctx,
		"SELECT version FROM content_pages WHERE id=? FOR UPDATE", id).Scan(&version)
	if err == sql.ErrNoRows {
		return Ref{}, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return Ref{}, err
	}
	if version != in.Version {
		return Ref{}, fault(409, "VERSION_CONFLICT")
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE content_pages SET slug=?,title=?,body_html=?,kind=?,published=?,
		 sort_order=?,version=version+1 WHERE id=? AND version=?`,
		in.Slug, in.Title, in.Body, in.Kind, in.Published, in.Sort, id, in.Version)
	return Ref{id, in.Version + 1, strconv.FormatBool(in.Published), "content", 200}, err
}
