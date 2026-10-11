package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/infra/files"
	"time"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func authorizeTx(ctx context.Context, tx *sql.Tx, a identity.Actor, permission string) error {
	var enabled, force bool
	var version int64
	if e := tx.QueryRowContext(ctx, "SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR UPDATE", a.AdminID).Scan(&enabled, &force, &version); e != nil {
		return e
	}
	if !enabled || version != a.AuthVersion {
		return fault(401, "AUTH_REQUIRED")
	}
	if force {
		return fault(403, "PASSWORD_CHANGE_REQUIRED")
	}
	var n int
	e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?", a.AdminID, permission).Scan(&n)
	if e != nil {
		return e
	}
	if n == 0 {
		return fault(403, "FORBIDDEN")
	}
	return nil
}
func loadReplay(ctx context.Context, tx *sql.Tx, scope, key string) (Ref, []byte, time.Time, error) {
	var ref Ref
	var raw, hash []byte
	var expiry time.Time
	e := tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&hash, &raw, &expiry)
	if e == nil {
		e = json.Unmarshal(raw, &ref)
	}
	return ref, hash, expiry, e
}
func deleteReplay(ctx context.Context, tx *sql.Tx, scope, key string) error {
	_, e := tx.ExecContext(ctx, "DELETE FROM idempotency_keys WHERE scope=? AND request_key=?", scope, key)
	return e
}
func saveReplay(ctx context.Context, tx *sql.Tx, scope, key string, hash []byte, ref Ref, now time.Time) error {
	b, e := json.Marshal(ref)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,response_status,result_ref,expires_at) VALUES(?,?,?,?,?,?)", scope, key, hash, ref.HTTP, b, now.Add(24*time.Hour))
	return e
}
func appendAudit(ctx context.Context, tx *sql.Tx, admin int64, requestID, action string, ref Ref, reason string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN',?,?,?,?,JSON_OBJECT('version',?,'state',?,'reason',?),'SUCCESS')", admin, action, ref.Type, ref.ID, requestID, ref.Version, ref.State, reason)
	return e
}
func validateRefs(ctx context.Context, tx *sql.Tx, in Input) error {
	if in.CategoryID != "" {
		var id int64
		if e := tx.QueryRowContext(ctx, "SELECT id FROM categories WHERE id=?", in.CategoryID).Scan(&id); e != nil {
			if e == sql.ErrNoRows {
				return fault(422, "INVALID_ARGUMENT")
			}
			return e
		}
	}
	for _, n := range in.MediaIDs {
		var kind, state string
		if e := tx.QueryRowContext(ctx, "SELECT kind,state FROM files WHERE id=? FOR SHARE", n).Scan(&kind, &state); e != nil {
			if e == sql.ErrNoRows {
				return fault(422, "INVALID_ARGUMENT")
			}
			return e
		}
		if kind != "IMAGE" || state != "READY" {
			return fault(422, "INVALID_ARGUMENT")
		}
	}
	return nil
}
func values(in Input) []any {
	attrs, _ := json.Marshal(in.Attributes)
	regions, _ := json.Marshal(in.Excluded)
	var category any
	if in.CategoryID != "" {
		category = in.CategoryID
	}
	return []any{in.Code, in.Name, category, attrs, in.Description, in.ExternalCode, in.ExternalName, in.From, in.Until, regions, in.Sort}
}
func replaceMedia(ctx context.Context, tx *sql.Tx, n string, ids []string) error {
	if _, e := tx.ExecContext(ctx, "DELETE FROM product_media WHERE product_id=?", n); e != nil {
		return e
	}
	for position, file := range ids {
		if _, e := tx.ExecContext(ctx, "INSERT INTO product_media(product_id,file_id,position) VALUES(?,?,?)", n, file, position); e != nil {
			return e
		}
	}
	return nil
}
func createProduct(ctx context.Context, tx *sql.Tx, in Input) (Ref, error) {
	r, e := tx.ExecContext(ctx, "INSERT INTO products(code,name,category_id,attributes,description_html,external_spec_code,external_spec_name,redeem_from,redeem_until,excluded_region_codes,sort_order) VALUES(?,?,?,?,?,?,?,?,?,?,?)", values(in)...)
	if e != nil {
		return Ref{}, e
	}
	n, e := r.LastInsertId()
	if e != nil {
		return Ref{}, e
	}
	id := fmt.Sprint(n)
	if e = replaceMedia(ctx, tx, id, in.MediaIDs); e != nil {
		return Ref{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO inventory_balances(product_id) VALUES(?)", id); e != nil {
		return Ref{}, e
	}
	return Ref{id, 1, "DRAFT", "product", 201}, nil
}
func updateProduct(ctx context.Context, tx *sql.Tx, n string, in Input, status string) (Ref, error) {
	args := append(values(in), n, in.Version)
	r, e := tx.ExecContext(ctx, "UPDATE products SET code=?,name=?,category_id=?,attributes=?,description_html=?,external_spec_code=?,external_spec_name=?,redeem_from=?,redeem_until=?,excluded_region_codes=?,sort_order=?,version=version+1 WHERE id=? AND version=? AND deleted_at IS NULL", args...)
	if e != nil {
		return Ref{}, e
	}
	count, e := r.RowsAffected()
	if e != nil {
		return Ref{}, e
	}
	if count != 1 {
		return Ref{}, fault(409, "VERSION_CONFLICT")
	}
	if e = replaceMedia(ctx, tx, n, in.MediaIDs); e != nil {
		return Ref{}, e
	}
	return Ref{n, in.Version + 1, status, "product", 200}, nil
}
func changeStatus(ctx context.Context, tx *sql.Tx, n string, in StatusInput) (Ref, error) {
	_, e := tx.ExecContext(ctx, "UPDATE products SET status=?,version=version+1 WHERE id=?", in.Status, n)
	return Ref{n, in.Version + 1, in.Status, "product", 200}, e
}

const columns = "id,code,name,category_id,attributes,description_html,external_spec_code,external_spec_name,redeem_from,redeem_until,excluded_region_codes,sort_order,version,status"

func scanProduct(row interface{ Scan(...any) error }) (Product, error) {
	var p Product
	var id int64
	var cat, extCode, extName sql.NullString
	var attrs, regions []byte
	var from, until sql.NullTime
	e := row.Scan(&id, &p.Code, &p.Name, &cat, &attrs, &p.Description, &extCode, &extName, &from, &until, &regions, &p.Sort, &p.Version, &p.Status)
	if e != nil {
		return p, e
	}
	p.ID = fmt.Sprint(id)
	p.CategoryID = cat.String
	p.ExternalCode = extCode.String
	p.ExternalName = extName.String
	if from.Valid {
		p.From = &from.Time
	}
	if until.Valid {
		p.Until = &until.Time
	}
	if e = json.Unmarshal(attrs, &p.Attributes); e != nil {
		return p, e
	}
	if e = json.Unmarshal(regions, &p.Excluded); e != nil {
		return p, e
	}
	p.MediaIDs = []string{}
	return p, nil
}
func loadMedia(ctx context.Context, q queryer, p *Product) error {
	rows, e := q.QueryContext(ctx, "SELECT file_id FROM product_media WHERE product_id=? ORDER BY position", p.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var n int64
		if e = rows.Scan(&n); e != nil {
			return e
		}
		p.MediaIDs = append(p.MediaIDs, fmt.Sprint(n))
	}
	return rows.Err()
}
func loadProduct(ctx context.Context, q queryer, n string, public, lock bool) (Product, error) {
	query := "SELECT " + columns + " FROM products WHERE id=? AND deleted_at IS NULL"
	if public {
		query += " AND status='ENABLED'"
	}
	if lock {
		query += " FOR UPDATE"
	}
	p, e := scanProduct(q.QueryRowContext(ctx, query, n))
	if e == sql.ErrNoRows {
		return Product{}, fault(404, "NOT_FOUND")
	}
	if e != nil {
		return p, e
	}
	e = loadMedia(ctx, q, &p)
	return p, e
}
func listProducts(ctx context.Context, q queryer, f Filter, public bool) (Page[Product], error) {
	out := Page[Product]{Items: []Product{}}
	query := "SELECT " + columns + " FROM products WHERE deleted_at IS NULL"
	args := []any{}
	if public {
		query += " AND status='ENABLED'"
	} else if f.Status != "" {
		query += " AND status=?"
		args = append(args, f.Status)
	}
	if f.After > 0 {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		query += " AND (LOCATE(?,name)>0 OR LOCATE(?,code)>0)"
		args = append(args, f.Q, f.Q)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		p, e := scanProduct(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	for i := range out.Items {
		if e = loadMedia(ctx, q, &out.Items[i]); e != nil {
			return out, e
		}
	}
	return out, nil
}
func createCategory(ctx context.Context, tx *sql.Tx, in CategoryInput) (Ref, error) {
	r, e := tx.ExecContext(ctx, "INSERT INTO categories(name,sort_order) VALUES(?,?)", in.Name, in.Sort)
	if e != nil {
		return Ref{}, e
	}
	n, e := r.LastInsertId()
	return Ref{fmt.Sprint(n), 1, "ENABLED", "category", 201}, e
}
func getCategory(ctx context.Context, q queryer, n string) (Category, error) {
	var c Category
	var id int64
	e := q.QueryRowContext(ctx, "SELECT id,name,sort_order,version FROM categories WHERE id=?", n).Scan(&id, &c.Name, &c.Sort, &c.Version)
	c.ID = fmt.Sprint(id)
	return c, e
}
func listCategories(ctx context.Context, q queryer, f Filter) (Page[Category], error) {
	out := Page[Category]{Items: []Category{}}
	query := "SELECT id,name,sort_order,version FROM categories WHERE 1=1"
	args := []any{}
	if f.After > 0 {
		query += " AND id<?"
		args = append(args, f.After)
	}
	if f.Q != "" {
		query += " AND LOCATE(?,name)>0"
		args = append(args, f.Q)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var c Category
		var id int64
		if e = rows.Scan(&id, &c.Name, &c.Sort, &c.Version); e != nil {
			return out, e
		}
		c.ID = fmt.Sprint(id)
		out.Items = append(out.Items, c)
	}
	if len(out.Items) > f.Limit {
		out.Items = out.Items[:f.Limit]
		out.Next = out.Items[len(out.Items)-1].ID
	}
	return out, rows.Err()
}
func insertImage(ctx context.Context, tx *sql.Tx, admin int64, img files.Image) (Ref, error) {
	r, e := tx.ExecContext(ctx, "INSERT INTO files(storage_key,kind,mime_type,size_bytes,sha256,state,owner_admin_id) VALUES(?,'IMAGE','image/png',?,?,'READY',?)", img.Key, len(img.Data), img.SHA, admin)
	if e != nil {
		return Ref{}, e
	}
	n, e := r.LastInsertId()
	return Ref{fmt.Sprint(n), 1, "READY", "file", 201}, e
}
func getFile(ctx context.Context, q queryer, n string) (File, error) {
	var f File
	var id int64
	e := q.QueryRowContext(ctx, "SELECT id,kind,state,mime_type,size_bytes FROM files WHERE id=?", n).Scan(&id, &f.Kind, &f.State, &f.MIME, &f.Size)
	f.ID = fmt.Sprint(id)
	return f, e
}
func imageKey(ctx context.Context, q queryer, n string) (string, error) {
	var key string
	e := q.QueryRowContext(ctx, "SELECT storage_key FROM files WHERE id=? AND kind='IMAGE' AND state='READY' AND deleted_at IS NULL", n).Scan(&key)
	return key, e
}
