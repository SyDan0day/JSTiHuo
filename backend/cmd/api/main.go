package main

import (
	"context"
	"encoding/base64"
	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/inventory"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/app/voucher"
	"jstihuo/internal/bootstrap"
	crypt "jstihuo/internal/infra/crypto"
	transport "jstihuo/internal/transport/http"
	"jstihuo/migrations"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// main 用途：加载配置与业务依赖并启动 API。
// 参数：无。
// 返回值：无。
// 约束：启动检查失败退出，信号触发有界关闭，退出释放数据库连接，不自动执行迁移。
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	c, e := bootstrap.Load()
	if e != nil {
		log.Error("configuration rejected", "reason", e.Error())
		os.Exit(1)
	}
	db, e := bootstrap.OpenDB(c.DSN)
	if e != nil {
		log.Error("database configuration rejected")
		os.Exit(1)
	}
	defer db.Close()
	ready := func(ctx context.Context) error {
		if e := db.PingContext(ctx); e != nil {
			return e
		}
		return migrations.Check(ctx, db)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	e = ready(ctx)
	cancel()
	if e != nil {
		log.Error("database or migration unavailable")
		os.Exit(1)
	}
	readKey := func(name string) []byte {
		b, _ := bootstrap.ReadSecret(name)
		v, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		return v
	}
	auth, e := identity.New(db, readKey("LOOKUP_HMAC_KEY_FILE"), readKey("CHALLENGE_HMAC_KEY_FILE"), readKey("REQUEST_HMAC_KEY_FILE"))
	if e != nil {
		log.Error("identity initialization failed")
		os.Exit(1)
	}
	root := os.Getenv("FILE_ROOT")
	if root == "" {
		root = "/var/lib/jst/files"
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		log.Error("file storage unavailable")
		os.Exit(1)
	}
	exportKey, keyVersion, e := bootstrap.ExportKey()
	if e != nil {
		log.Error("export keyring rejected")
		os.Exit(1)
	}
	vouchers := voucher.New(db, root, readKey("REQUEST_HMAC_KEY_FILE"), exportKey, keyVersion)
	vouchers.ExportKeys, e = bootstrap.ExportKeys()
	if e != nil {
		log.Error("export keyring rejected")
		os.Exit(1)
	}
	catalogue := catalog.New(db, root, readKey("REQUEST_HMAC_KEY_FILE"))
	piiRaw, e := bootstrap.ReadSecret("PII_KEYRING_FILE")
	if e != nil {
		log.Error("PII keyring unavailable")
		os.Exit(1)
	}
	pii, e := crypt.ParseKeyring(piiRaw)
	if e != nil {
		log.Error("PII keyring rejected")
		os.Exit(1)
	}
	redeem, e := redemption.New(db, auth, catalogue, pii, readKey("REQUEST_HMAC_KEY_FILE"), readKey("LOOKUP_HMAC_KEY_FILE"))
	if e != nil {
		log.Error("redemption initialization failed")
		os.Exit(1)
	}
	authHandler := transport.IdentityHandler{
		Service: auth, Origin: c.Origin,
		Development: os.Getenv("APP_ENV") == "development",
	}
	stock := inventory.New(db, readKey("REQUEST_HMAC_KEY_FILE"))
	api := transport.API(authHandler, catalogue, stock)
	api = transport.WithVouchers(vouchers, authHandler, api)
	api = transport.WithRedemption(redeem, authHandler, api)
	orders := order.New(db, auth, redeem)
	api = transport.WithOrders(orders, authHandler, api)
	api = transport.WithShipping(shipping.New(db, readKey("REQUEST_HMAC_KEY_FILE")), orders, authHandler, api)
	srv := &http.Server{
		Addr: c.Address, Handler: transport.Router(ready, log, api),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second,
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()
	log.Info("api started", "address", c.Address)
	if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Error("http server failed")
		os.Exit(1)
	}
}
