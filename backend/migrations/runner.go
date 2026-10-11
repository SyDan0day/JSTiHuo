package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
)

func Check(ctx context.Context, db *sql.DB) error {
	names, _ := fs.Glob(Files, "*.sql")
	for _, n := range names {
		b, _ := Files.ReadFile(n)
		var checksum string
		if e := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=?", n).Scan(&checksum); e != nil {
			return fmt.Errorf("migration missing: %s", n)
		}
		if checksum != fmt.Sprintf("%x", sha256.Sum256(b)) {
			return fmt.Errorf("migration checksum mismatch: %s", n)
		}
	}
	return nil
}

// MySQL DDL implicitly commits. Failed partial migrations require operator repair;
// never mark them applied or silently skip already-created tables.
func Apply(ctx context.Context, db *sql.DB) error {
	c, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer c.Close()
	var locked int
	if e = c.QueryRowContext(ctx, "SELECT GET_LOCK('jst_schema_migrations',10)").Scan(&locked); e != nil || locked != 1 {
		return fmt.Errorf("migration lock unavailable")
	}
	defer c.ExecContext(context.Background(), "SELECT RELEASE_LOCK('jst_schema_migrations')")
	if _, e = c.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(100) PRIMARY KEY, checksum CHAR(64) NOT NULL, applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))"); e != nil {
		return e
	}
	names, _ := fs.Glob(Files, "*.sql")
	for _, n := range names {
		b, _ := Files.ReadFile(n)
		sum := fmt.Sprintf("%x", sha256.Sum256(b))
		var prior string
		e = c.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version=?", n).Scan(&prior)
		if e == nil {
			if prior != sum {
				return fmt.Errorf("checksum changed: %s", n)
			}
			continue
		}
		if e != sql.ErrNoRows {
			return e
		}
		for _, stmt := range strings.Split(string(b), ";") {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, e = c.ExecContext(ctx, stmt); e != nil {
				return fmt.Errorf("migration %s failed; inspect partial DDL: %w", n, e)
			}
		}
		if _, e = c.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum) VALUES (?,?)", n, sum); e != nil {
			return e
		}
	}
	return nil
}
