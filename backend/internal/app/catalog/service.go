package catalog

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/infra/files"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type Service struct {
	DB         *sql.DB
	FileRoot   string
	RequestKey []byte
	Now        func() time.Time
}

func New(db *sql.DB, root string, key []byte) *Service {
	return &Service{db, root, key, func() time.Time { return time.Now().UTC() }}
}
func mapError(e error) error {
	var my *mysql.MySQLError
	if errors.As(e, &my) {
		if my.Number == 1062 {
			return fault(409, "VERSION_CONFLICT")
		}
		if my.Number == 1452 {
			return fault(422, "INVALID_ARGUMENT")
		}
	}
	return e
}
func (s *Service) mutate(ctx context.Context, a identity.Actor, operation, key, permission, requestID string, input any, fn func(*sql.Tx) (Ref, error)) (Ref, error) {
	if !a.Can(permission) {
		return Ref{}, fault(403, "FORBIDDEN")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(key) {
		return Ref{}, fault(422, "INVALID_ARGUMENT")
	}
	b, e := json.Marshal(input)
	if e != nil {
		return Ref{}, e
	}
	h := hmac.New(sha256.New, s.RequestKey)
	h.Write(b)
	hash := h.Sum(nil)
	scope := fmt.Sprintf("admin:%d:%s", a.AdminID, operation)
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return Ref{}, e
	}
	defer tx.Rollback()
	if e = authorizeTx(ctx, tx, a, permission); e != nil {
		return Ref{}, e
	}
	prior, oldHash, expiry, e := loadReplay(ctx, tx, scope, key)
	if e == nil && s.Now().Before(expiry) {
		if subtle.ConstantTimeCompare(hash, oldHash) != 1 {
			return Ref{}, fault(409, "IDEMPOTENCY_CONFLICT")
		}
		return prior, nil
	}
	if e != nil && e != sql.ErrNoRows {
		return Ref{}, e
	}
	if e == nil {
		if e = deleteReplay(ctx, tx, scope, key); e != nil {
			return Ref{}, e
		}
	}
	result, e := fn(tx)
	if e != nil {
		return Ref{}, mapError(e)
	}
	if e = saveReplay(ctx, tx, scope, key, hash, result, s.Now()); e != nil {
		return Ref{}, mapError(e)
	}
	reason := ""
	if v, ok := input.(struct {
		ID    string
		Input StatusInput
	}); ok {
		reason = v.Input.Reason
	}
	if e = appendAudit(ctx, tx, a.AdminID, requestID, operation, result, reason); e != nil {
		return Ref{}, e
	}
	return result, tx.Commit()
}
func (s *Service) Create(ctx context.Context, a identity.Actor, in Input, key, requestID string) (Product, error) {
	if e := Validate(&in, false); e != nil {
		return Product{}, e
	}
	raw := in
	clean, e := Sanitize(in.Description)
	if e != nil {
		return Product{}, e
	}
	if !length(clean, 0, 50000) {
		return Product{}, fault(422, "INVALID_ARGUMENT")
	}
	in.Description = clean
	ref, e := s.mutate(ctx, a, "createProduct", key, "product.write", requestID, raw, func(tx *sql.Tx) (Ref, error) {
		if e := validateRefs(ctx, tx, in); e != nil {
			return Ref{}, e
		}
		return createProduct(ctx, tx, in)
	})
	if e != nil {
		return Product{}, e
	}
	return s.Get(ctx, ref.ID, false)
}
func (s *Service) Update(ctx context.Context, a identity.Actor, n string, in Input, key, requestID string) (Product, error) {
	if _, e := parseID(n); e != nil {
		return Product{}, e
	}
	if e := Validate(&in, true); e != nil {
		return Product{}, e
	}
	raw := struct {
		ID    string
		Input Input
	}{n, in}
	clean, e := Sanitize(in.Description)
	if e != nil {
		return Product{}, e
	}
	if !length(clean, 0, 50000) {
		return Product{}, fault(422, "INVALID_ARGUMENT")
	}
	in.Description = clean
	ref, e := s.mutate(ctx, a, "updateProduct", key, "product.write", requestID, raw, func(tx *sql.Tx) (Ref, error) {
		p, e := loadProduct(ctx, tx, n, false, true)
		if e != nil {
			return Ref{}, e
		}
		if p.Version != in.Version {
			return Ref{}, fault(409, "VERSION_CONFLICT")
		}
		if e = validateRefs(ctx, tx, in); e != nil {
			return Ref{}, e
		}
		return updateProduct(ctx, tx, n, in, p.Status)
	})
	if e != nil {
		return Product{}, e
	}
	return s.Get(ctx, ref.ID, false)
}
func (s *Service) Status(ctx context.Context, a identity.Actor, n string, in StatusInput, key, requestID string) (Ref, error) {
	if _, e := parseID(n); e != nil {
		return Ref{}, e
	}
	if in.Version < 1 || in.Version > 9007199254740991 || (in.Status != "ENABLED" && in.Status != "DISABLED") || !length(in.Reason, 0, 200) {
		return Ref{}, fault(422, "INVALID_ARGUMENT")
	}
	return s.mutate(ctx, a, "productStatus", key, "product.write", requestID, struct {
		ID    string
		Input StatusInput
	}{n, in}, func(tx *sql.Tx) (Ref, error) {
		p, e := loadProduct(ctx, tx, n, false, true)
		if e != nil {
			return Ref{}, e
		}
		if p.Version != in.Version {
			return Ref{}, fault(409, "VERSION_CONFLICT")
		}
		if in.Status == "ENABLED" {
			if e = Validate(&p.Input, true); e != nil {
				return Ref{}, e
			}
			if e = validateRefs(ctx, tx, p.Input); e != nil {
				return Ref{}, e
			}
		}
		return changeStatus(ctx, tx, n, in)
	})
}
func (s *Service) Get(ctx context.Context, n string, public bool) (Product, error) {
	if _, e := parseID(n); e != nil {
		return Product{}, e
	}
	return loadProduct(ctx, s.DB, n, public, false)
}
func (s *Service) List(ctx context.Context, f Filter, public bool) (Page[Product], error) {
	return listProducts(ctx, s.DB, f, public)
}
func (s *Service) PublicList(ctx context.Context, f Filter) (Page[Public], error) {
	page, e := s.List(ctx, f, true)
	out := Page[Public]{Items: []Public{}, Next: page.Next}
	for _, p := range page.Items {
		out.Items = append(out.Items, project(p))
	}
	return out, e
}
func (s *Service) PublicGet(ctx context.Context, n string) (Public, error) {
	p, e := s.Get(ctx, n, true)
	return project(p), e
}
func (s *Service) Categories(ctx context.Context, f Filter) (Page[Category], error) {
	return listCategories(ctx, s.DB, f)
}
func (s *Service) CreateCategory(ctx context.Context, a identity.Actor, in CategoryInput, key, requestID string) (Category, error) {
	if !length(in.Name, 1, 80) || in.Sort < 0 || in.Sort > 1000000 {
		return Category{}, fault(422, "INVALID_ARGUMENT")
	}
	ref, e := s.mutate(ctx, a, "createCategory", key, "product.write", requestID, in, func(tx *sql.Tx) (Ref, error) { return createCategory(ctx, tx, in) })
	if e != nil {
		return Category{}, e
	}
	return getCategory(ctx, s.DB, ref.ID)
}
func (s *Service) Upload(ctx context.Context, a identity.Actor, raw []byte, key, requestID string) (File, error) {
	if !a.Can("file.write") {
		return File{}, fault(403, "FORBIDDEN")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(key) {
		return File{}, fault(422, "INVALID_ARGUMENT")
	}
	img, e := files.PrepareImage(raw)
	if e != nil {
		return File{}, fault(422, "INVALID_ARGUMENT")
	}
	if e = files.WriteImage(s.FileRoot, img); e != nil {
		return File{}, e
	}
	hash := sha256.Sum256(raw)
	ref, e := s.mutate(ctx, a, "uploadFile", key, "file.write", requestID, fmt.Sprintf("IMAGE:%x", hash), func(tx *sql.Tx) (Ref, error) { return insertImage(ctx, tx, a.AdminID, img) })
	if e != nil {
		var f *identity.Fault
		if errors.As(e, &f) {
			os.Remove(filepath.Join(s.FileRoot, img.Key))
		}
		return File{}, e
	}
	result, e := getFile(ctx, s.DB, ref.ID)
	if e != nil {
		return File{}, e
	}
	stored, e := imageKey(ctx, s.DB, ref.ID)
	if e == nil && stored != img.Key {
		os.Remove(filepath.Join(s.FileRoot, img.Key))
	}
	return result, e
}
func (s *Service) Image(ctx context.Context, n string) (*os.File, error) {
	if _, e := parseID(n); e != nil {
		return nil, fault(404, "NOT_FOUND")
	}
	key, e := imageKey(ctx, s.DB, n)
	if e != nil {
		return nil, fault(404, "NOT_FOUND")
	}
	f, e := files.OpenImage(s.FileRoot, key)
	if e != nil {
		return nil, fault(404, "NOT_FOUND")
	}
	return f, nil
}
