package main

import (
	"context"
	"encoding/base64"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/voucher"
	"jstihuo/internal/bootstrap"
	"jstihuo/internal/infra/integrations/mock"
	"jstihuo/migrations"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// main 用途：运行生成、凭证维护与显式 MOCK 物流 worker。
// 参数：无。
// 返回值：无。
// 约束：配置或迁移失败退出，信号取消任务，关闭数据库，定期维护租约、TTL、孤儿密文，不发送真实通知。
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	c, e := bootstrap.Load()
	if e != nil {
		log.Error("configuration rejected")
		os.Exit(1)
	}
	db, e := bootstrap.OpenDB(c.DSN)
	if e != nil {
		log.Error("database unavailable")
		os.Exit(1)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	check, cancel := context.WithTimeout(ctx, 10*time.Second)
	e = migrations.Check(check, db)
	cancel()
	if e != nil {
		log.Error("migration unavailable")
		os.Exit(1)
	}
	key, version, e := bootstrap.ExportKey()
	if e != nil {
		log.Error("export keyring rejected")
		os.Exit(1)
	}
	raw, e := bootstrap.ReadSecret("REQUEST_HMAC_KEY_FILE")
	if e != nil {
		log.Error("request key rejected")
		os.Exit(1)
	}
	requestKey, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	root := os.Getenv("FILE_ROOT")
	if root == "" {
		root = "/var/lib/jst/files"
	}
	svc := voucher.New(db, root, requestKey, key, version)
	svc.ExportKeys, e = bootstrap.ExportKeys()
	if e != nil {
		log.Error("export keyring rejected")
		os.Exit(1)
	}
	trackingOrders := &order.Service{DB: db, Now: time.Now}
	tracking := mock.Tracking{DB: db, Now: time.Now, Publish: trackingOrders.RecordTracking}
	nextCleanup := time.Time{}
	nextMaintenance := time.Time{}
	log.Info("generation worker started")
	for ctx.Err() == nil {
		if _, err := tracking.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Error("mock tracking failed")
		}
		if !time.Now().Before(nextCleanup) {
			if e := svc.CleanupOrphans(ctx); e != nil && ctx.Err() == nil {
				log.Error("orphan cleanup failed")
			}
			nextCleanup = time.Now().Add(time.Hour)
		}
		if !time.Now().Before(nextMaintenance) {
			if e := svc.Maintain(ctx); e != nil && ctx.Err() == nil {
				log.Error("credential maintenance failed")
			}
			nextMaintenance = time.Now().Add(time.Minute)
		}
		worked, e := svc.RunOnce(ctx)
		if e != nil && ctx.Err() == nil {
			log.Error("generation failed")
		}
		if !worked || e != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}
