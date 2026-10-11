package bootstrap

import (
	"context"
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"time"
)

func OpenDB(dsn string) (*sql.DB, error) {
	c, e := mysql.ParseDSN(dsn)
	if e != nil {
		return nil, e
	}
	c.ParseTime = true
	c.Loc = time.UTC
	c.MultiStatements = false
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'+00:00'"
	c.Params["innodb_lock_wait_timeout"] = "3"
	c.Timeout = 5 * time.Second
	c.ReadTimeout = 10 * time.Second
	c.WriteTimeout = 10 * time.Second
	db, e := sql.Open("mysql", c.FormatDSN())
	if e == nil {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(5 * time.Minute)
	}
	return db, e
}
func Ping(ctx context.Context, db *sql.DB) error { return db.PingContext(ctx) }
