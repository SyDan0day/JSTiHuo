package inventory

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/infra/businessid"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB  *sql.DB
	Key []byte
	Now func() time.Time
}

func New(db *sql.DB, key []byte) *Service { return &Service{db, key, time.Now} }

type ref struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// The uncommitted placeholder serializes a key before entity locks. Every failure rolls it back.
func (s *Service) mutate(ctx context.Context, a identity.Actor, op, key, rid string, body any, permission string, fn func(*sql.Tx) (ref, error)) (ref, error) {
	if !keyPattern.MatchString(key) {
		return ref{}, fault(400, "INVALID_ARGUMENT")
	}
	raw, _ := json.Marshal(body)
	mac := hmac.New(sha256.New, s.Key)
	mac.Write(raw)
	hash := mac.Sum(nil)
	scope := fmt.Sprintf("admin:%d:%s", a.AdminID, op)
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return ref{}, e
	}
	defer tx.Rollback()
	now := s.Now().UTC()
	_, e = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour))
	if e != nil {
		return ref{}, e
	}
	var previous, stored []byte
	var expiry time.Time
	e = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &previous, &expiry)
	if e != nil {
		return ref{}, e
	}
	// Shared admin lock revalidates authority without serializing unrelated reviewers.
	var enabled, force bool
	var version int64
	e = tx.QueryRowContext(ctx, "SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR SHARE", a.AdminID).Scan(&enabled, &force, &version)
	if e != nil {
		return ref{}, e
	}
	if !enabled || version != a.AuthVersion {
		return ref{}, fault(401, "AUTH_REQUIRED")
	}
	if force {
		return ref{}, fault(403, "PASSWORD_CHANGE_REQUIRED")
	}
	var count int
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?", a.AdminID, permission).Scan(&count)
	if e != nil {
		return ref{}, e
	}
	if count == 0 {
		return ref{}, fault(403, "FORBIDDEN")
	}
	var result ref
	if expiry.After(now) {
		if !hmac.Equal(stored, hash) {
			return result, fault(409, "IDEMPOTENCY_CONFLICT")
		}
		if e = json.Unmarshal(previous, &result); e != nil {
			return result, e
		}
		if result.ID != "" {
			return result, tx.Commit()
		}
	}
	result, e = fn(tx)
	if e != nil {
		return ref{}, e
	}
	audit := map[string]any{"operation": op}
	switch v := body.(type) {
	case struct {
		ID    string
		Input Review
	}:
		audit["reason"] = v.Input.Reason
		audit["version"] = v.Input.Version
	case struct {
		ID    string
		Input Warning
	}:
		audit["version"] = v.Input.Version
		audit["warning_level"] = v.Input.Level
	}
	diff, _ := json.Marshal(audit)
	_, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN',?,?,?,?,?,'SUCCESS')", a.AdminID, op, result.Type, result.ID, rid, diff)
	if e != nil {
		return ref{}, e
	}
	b, _ := json.Marshal(result)
	_, e = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=?,expires_at=? WHERE scope=? AND request_key=?", hash, b, map[bool]int{true: 201, false: 200}[op == "createStockDocument"], now.Add(24*time.Hour), scope, key)
	if e != nil {
		return ref{}, e
	}
	return result, tx.Commit()
}
func (s *Service) Create(ctx context.Context, a identity.Actor, in Input, key, rid string) (Document, error) {
	if (in.Direction != "IN" && in.Direction != "OUT") || (in.Kind != "PHYSICAL" && in.Kind != "VIRTUAL") || len(in.Items) < 1 || len(in.Items) > 100 || utf8.RuneCountInString(in.Note) > 500 || (in.Proof != "" && !ValidID(in.Proof)) {
		return Document{}, fault(422, "VALIDATION_FAILED")
	}
	seen := map[string]bool{}
	for _, l := range in.Items {
		if !ValidID(l.ProductID) || l.Quantity < 1 || l.Quantity > 100000 || seen[l.ProductID] {
			return Document{}, fault(422, "VALIDATION_FAILED")
		}
		seen[l.ProductID] = true
	}
	result, e := s.mutate(ctx, a, "createStockDocument", key, rid, in, "inventory.write", func(tx *sql.Tx) (ref, error) {
		if in.Proof != "" {
			var n int
			e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE id=? AND kind='IMAGE' AND state='READY'", in.Proof).Scan(&n)
			if e != nil {
				return ref{}, e
			}
			if n != 1 {
				return ref{}, fault(422, "VALIDATION_FAILED")
			}
		}
		for _, l := range in.Items {
			var n int
			e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM products WHERE id=? AND deleted_at IS NULL", l.ProductID).Scan(&n)
			if e != nil {
				return ref{}, e
			}
			if n != 1 {
				return ref{}, fault(422, "VALIDATION_FAILED")
			}
		}
		var proof any
		if in.Proof != "" {
			proof = in.Proof
		}
		var r sql.Result
		var e error
		for attempt := 0; attempt < 3; attempt++ {
			number, err := businessid.New("I_", s.Now())
			if err != nil {
				return ref{}, err
			}
			r, e = tx.ExecContext(ctx, "INSERT INTO inventory_documents(document_no,direction,stock_kind,note,proof_file_id,created_by) VALUES(?,?,?,?,?,?)", number, in.Direction, in.Kind, in.Note, proof, a.AdminID)
			if e == nil {
				break
			}
			var duplicate *mysql.MySQLError
			if !errors.As(e, &duplicate) || duplicate.Number != 1062 {
				return ref{}, e
			}
		}
		if e != nil {
			return ref{}, e
		}
		id, e := r.LastInsertId()
		if e != nil {
			return ref{}, e
		}
		for _, l := range in.Items {
			if _, e = tx.ExecContext(ctx, "INSERT INTO inventory_document_items(document_id,product_id,quantity) VALUES(?,?,?)", id, l.ProductID, l.Quantity); e != nil {
				return ref{}, e
			}
		}
		return ref{fmt.Sprint(id), "inventory_document"}, nil
	})
	if e != nil {
		return Document{}, e
	}
	return s.document(ctx, s.DB, result.ID, false)
}
func (s *Service) Review(ctx context.Context, a identity.Actor, id string, in Review, approve bool, key, rid string) (Document, error) {
	if !ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || utf8.RuneCountInString(in.Reason) > 200 || (!approve && strings.TrimSpace(in.Reason) == "") {
		return Document{}, fault(422, "VALIDATION_FAILED")
	}
	op, state := "rejectStockDocument", "REJECTED"
	if approve {
		op, state = "approveStockDocument", "APPROVED"
	}
	result, e := s.mutate(ctx, a, op, key, rid, struct {
		ID    string
		Input Review
	}{id, in}, "inventory.approve", func(tx *sql.Tx) (ref, error) {
		d, e := s.document(ctx, tx, id, true)
		if e != nil {
			return ref{}, e
		}
		if d.Version != in.Version {
			return ref{}, fault(409, "VERSION_CONFLICT")
		}
		if d.State != "PENDING" {
			return ref{}, fault(409, "ILLEGAL_TRANSITION")
		}
		if approve {
			sort.Slice(d.Items, func(i, j int) bool {
				x, _ := strconv.ParseInt(d.Items[i].ProductID, 10, 64)
				y, _ := strconv.ParseInt(d.Items[j].ProductID, 10, 64)
				return x < y
			})
			for _, l := range d.Items {
				var n int64
				e = tx.QueryRowContext(ctx, "SELECT id FROM products WHERE id=? AND deleted_at IS NULL FOR UPDATE", l.ProductID).Scan(&n)
				if e != nil {
					return ref{}, e
				}
			}
			balances := make([]Balance, len(d.Items))
			for i, l := range d.Items {
				b, e := s.balance(ctx, tx, l.ProductID, true)
				if e != nil {
					return ref{}, e
				}
				if d.Direction == "OUT" && b.Available < l.Quantity {
					return ref{}, fault(422, "OUT_OF_STOCK")
				}
				if d.Direction == "IN" && b.OnHand+l.Quantity > 1000000 {
					return ref{}, fault(422, "VALIDATION_FAILED")
				}
				balances[i] = b
			}
			for i, l := range d.Items {
				delta := l.Quantity
				if d.Direction == "OUT" {
					delta = -delta
				}
				r, e := tx.ExecContext(ctx, "UPDATE inventory_balances SET on_hand=on_hand+?,version=version+1 WHERE product_id=? AND version=?", delta, l.ProductID, balances[i].Version)
				if e != nil {
					return ref{}, e
				}
				n, e := r.RowsAffected()
				if e != nil {
					return ref{}, e
				}
				if n != 1 {
					return ref{}, fault(409, "VERSION_CONFLICT")
				}
				_, e = tx.ExecContext(ctx, "INSERT INTO inventory_transactions(product_id,on_hand_delta,event_key,document_id,reason) VALUES(?,?,?,?,?)", l.ProductID, delta, "document:"+id+":"+l.ProductID, id, in.Reason)
				if e != nil {
					return ref{}, e
				}
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE inventory_documents SET state=?,version=version+1,reviewed_by=?,reviewed_at=? WHERE id=?", state, a.AdminID, s.Now().UTC(), id)
		return ref{id, "inventory_document"}, e
	})
	if e != nil {
		return Document{}, e
	}
	return s.document(ctx, s.DB, result.ID, false)
}
func (s *Service) SetWarning(ctx context.Context, a identity.Actor, id string, in Warning, key, rid string) (Balance, error) {
	if !ValidID(id) || in.Version < 1 || in.Version > 9007199254740991 || in.Level < 0 || in.Level > 1000000 {
		return Balance{}, fault(422, "VALIDATION_FAILED")
	}
	r, e := s.mutate(ctx, a, "setInventoryWarning", key, rid, struct {
		ID    string
		Input Warning
	}{id, in}, "inventory.write", func(tx *sql.Tx) (ref, error) {
		b, e := s.balance(ctx, tx, id, true)
		if e != nil {
			return ref{}, e
		}
		if b.Version != in.Version {
			return ref{}, fault(409, "VERSION_CONFLICT")
		}
		_, e = tx.ExecContext(ctx, "UPDATE inventory_balances SET warning_level=?,version=version+1 WHERE product_id=?", in.Level, id)
		return ref{id, "inventory_balance"}, e
	})
	if e != nil {
		return Balance{}, e
	}
	return s.balance(ctx, s.DB, r.ID, false)
}
