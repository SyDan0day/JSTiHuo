package integration

import (
	"context"
	"jstihuo/internal/bootstrap"
	"jstihuo/migrations"
	"os"
	"testing"
)

func TestMigrations(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("TEST_DB_DSN must reference an empty MySQL 8.4 test database")
	}
	db, e := bootstrap.OpenDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = migrations.Check(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE schema_migrations SET checksum=REPEAT('0',64) WHERE version='0001_identity_catalog.sql'"); e != nil {
		t.Fatal(e)
	}
	if migrations.Apply(ctx, db) == nil {
		t.Fatal("checksum alteration accepted")
	}
}
