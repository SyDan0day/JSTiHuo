package main

import (
	"context"
	"jstihuo/internal/bootstrap"
	"jstihuo/migrations"
	"log"
	"strings"
	"time"
)

func main() {
	b, e := bootstrap.ReadSecret("DB_DSN_FILE")
	if e != nil {
		log.Fatal(e)
	}
	db, e := bootstrap.OpenDB(strings.TrimSpace(string(b)))
	if e != nil {
		log.Fatal("invalid database configuration")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = migrations.Apply(ctx, db); e != nil {
		log.Fatal(e)
	}
	log.Print("migrations applied")
}
