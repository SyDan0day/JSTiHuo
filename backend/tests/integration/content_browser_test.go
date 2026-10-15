//go:build browserfixture

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/shipping"
	"jstihuo/internal/infra/integrations/mock"
)

// TestContentBrowser 用途：提供真实 MySQL、登录会话和自有数据的 F1 浏览器夹具。
// 参数：t 测试上下文，需显式启用 browserfixture 标签与 TEST_DB_DSN。
// 返回值：无，停止文件出现后关闭服务、清理会话文件与隔离数据库。
// 约束：仅绑定 localhost:8080，非 API 请求代理到本地 Vite，不使用业务库。
func TestContentBrowser(t *testing.T) {
	f := newFulfillment(t)
	seedContentPages(t, f)
	seedContentGifts(t, f)
	metadata := map[string]string{"cookie": f.client.cookies["jst-admin"].Value}
	if os.Getenv("F1_BROWSER_TRANSACTIONS") == "1" {
		seedBrowserTransactions(t, f, metadata)
	}
	clock := new(atomic.Int64)
	clock.Store(f.now.UnixMicro())
	now := func() time.Time { return time.UnixMicro(clock.Load()).UTC() }
	f.auth.Now, f.orders.Now, f.orders.Redemption.Now, f.operations.Now, f.vouchers.Now, f.carriers.Now = now, now, now, now, now, now
	metadata["current_time"] = now().Format(time.RFC3339)
	if os.Getenv("F1_BROWSER_WORKER") == "1" {
		defer startBrowserWorker(t, f)()
	}
	directory := os.Getenv("F1_BROWSER_DIR")
	if directory == "" {
		t.Fatal("F1_BROWSER_DIR required")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(directory, "session.json")
	if err = os.WriteFile(sessionFile, meta, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(sessionFile)
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse("http://127.0.0.1:5173")
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/media/") {
			f.handler.ServeHTTP(w, r)
		} else {
			proxy.ServeHTTP(w, r)
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	failures := make(chan error, 1)
	go func() { failures <- server.Serve(listener) }()
	t.Log("F1 local browser fixture ready")
	for {
		applyBrowserClock(t, f, directory, clock)
		if _, err = os.Stat(filepath.Join(directory, "stop")); err == nil {
			return
		}
		select {
		case err = <-failures:
			t.Fatal(err)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// startBrowserWorker 用途：为浏览器夹具启动真实生成与文件 worker。
// 参数：t 测试上下文；f 隔离业务服务。
// 返回值：停止并等待协程退出的清理函数，不暴露到生产构建。
func startBrowserWorker(t *testing.T, f *fulfillmentFixture) func() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			adapter := mock.Tracking{DB: f.db, Now: f.auth.Now, Publish: f.orders.RecordTracking}
			if _, err := adapter.RunOnce(ctx); err != nil && ctx.Err() == nil {
				t.Error("浏览器 MOCK 轨迹 worker 失败")
			}
			if _, err := f.operations.RunOutbox(ctx); err != nil && ctx.Err() == nil {
				t.Error("浏览器 Outbox worker 失败")
			}
			if _, err := f.vouchers.RunOnce(ctx); err != nil && ctx.Err() == nil {
				t.Error("浏览器生成 worker 失败")
			}
			if _, err := f.operations.RunOnce(ctx); err != nil && ctx.Err() == nil {
				t.Error("浏览器文件 worker 失败")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}

// applyBrowserClock 用途：响应仅本地构建标签夹具的文件时钟指令。
// 参数：t 测试上下文；f 隔离夹具；directory 私有控制目录；clock 原子测试时钟。
// 返回值：无；仅允许至多三十一天正向推进，不增加任何生产 HTTP 控制入口。
func applyBrowserClock(t *testing.T, f *fulfillmentFixture, directory string, clock *atomic.Int64) {
	path := filepath.Join(directory, "clock.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		Seconds int64  `json:"advance_seconds"`
		ID      string `json:"id"`
	}
	if json.Unmarshal(raw, &command) != nil || command.Seconds < 1 || command.Seconds > 31*24*3600 || len(command.ID) > 80 {
		t.Fatal("非法浏览器时钟指令")
	}
	now := time.UnixMicro(clock.Add(command.Seconds * 1_000_000)).UTC()
	if _, err = f.db.Exec("UPDATE sessions SET expires_at=?,last_seen_at=?,reauth_at=?", now.Add(8*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}
	ack, err := json.Marshal(map[string]string{"id": command.ID, "current_time": now.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "clock-ack.json"), ack, 0600); err != nil {
		t.Fatal(err)
	}
}

// seedBrowserTransactions 用途：装配仅本地浏览器验收的已发与待发真实订单。
// 参数：t 测试上下文；f 隔离夹具；metadata 只保存会话和对象引用的临时元数据。
// 返回值：无，卡密仅驻留进程内存，不写浏览器截图或文件。
func seedBrowserTransactions(t *testing.T, f *fulfillmentFixture, metadata map[string]string) {
	t.Helper()
	f.seedFulfillment(t, 2)
	c, _ := f.customer(t, 0)
	first := response[redemption.Order](t, c.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	other, _ := f.customer(t, 1)
	second := response[redemption.Order](t, other.call("POST", "redemptions", f.input(), "voucher", testRequestKey(t)), 201)
	carrier := response[shipping.Carrier](t, f.client.call("POST", "admin/carriers", shipping.CarrierInput{Code: "MOCK", Name: "MOCK 测试承运商", Provider: "MOCK", Enabled: true}, "admin", testRequestKey(t)), 201)
	advanceFulfillment(t, f, 24*time.Hour)
	response[order.Shipment](t, f.client.call("POST", "admin/orders/"+first.ID+"/shipments", order.ShipInput{Version: first.Version, CarrierID: carrier.ID, Tracking: "BROWSER-PRIMARY-001"}, "admin", testRequestKey(t)), 200)
	config, err := json.Marshal(map[string]any{"min_lead_days": 1, "horizon_days": 30, "cutoff_hour": 17, "daily_capacity": 100, "blackout_dates": []string{}, "notification_mode": "MOCK", "default_carrier_id": carrier.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec("INSERT INTO system_configs(config_key,value) VALUES('shipping_calendar',?)", config); err != nil {
		t.Fatal(err)
	}
	metadata["order_cookie"], metadata["order_id"], metadata["label_order_id"], metadata["carrier_id"] = c.cookies["jst-order"].Value, first.ID, second.ID, carrier.ID
}

// seedContentPages 用途：通过真实内容 API 发布自有的须知和说明夹具。
// 参数：t 测试上下文；f 隔离服务夹具。
// 返回值：无，任一持久化失败终止测试。
func seedContentPages(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	pages := []struct{ slug, title, body, kind string }{
		{"guide", "提货须知", "<h2>从鉴真到预约</h2><ol><li>输入卡号和密码，核验卡券权益。</li><li>按权益规则选择礼品，每种一件。</li><li>填写收货信息，选择系统提供的日期。</li><li>核对后提交，保存订单号以便查询。</li></ol><p>鉴真与预览不会消费卡券。最终提交成功后，一张卡只生成一个订单。</p>", "PAGE"},
		{"about", "关于礼品提货", "<p>JSTiHuo 是独立开发的礼品提货系统，提供卡券鉴真、预约提货与订单查询。</p>", "PAGE"},
		{"contact", "联系我们", "<p>本页为本地测试内容。正式运营联系方式须由管理员核实并发布。</p>", "PAGE"},
		{"notice", "预约与提货提醒", "<p>可预约日期以系统实时结果为准，请核对收货信息后提交。</p>", "ANNOUNCEMENT"},
		{"gift-guide", "每份心意，按需预约", "<p>先了解权益，再安排合适的提货时间。</p>", "BANNER"},
		{"order-guide", "从预约到送达，进度可查", "<p>使用订单号与收货手机查询订单。</p>", "BANNER"},
	}
	for _, page := range pages {
		in := catalog.ContentInput{Slug: page.slug, Title: page.title, Body: page.body, Kind: page.kind, Published: true}
		response[catalog.Content](t, f.client.call("POST", "admin/content", in, "admin", "preview-content-"+page.slug), 201)
	}
}

// seedContentGifts 用途：创建四个带自有合成图片的真实公开商品。
// 参数：t 测试上下文；f 隔离服务夹具。
// 返回值：无，上传、保存或发布失败立即终止。
func seedContentGifts(t *testing.T, f *fulfillmentFixture) {
	t.Helper()
	cat := f.orders.Redemption.Catalog
	ctx := context.Background()
	for i, name := range []string{"秋日暖意礼盒", "团圆甄选礼包", "田园果香礼盒", "日常好物礼盒"} {
		var raw bytes.Buffer
		if err := png.Encode(&raw, contentGiftImage(i)); err != nil {
			t.Fatal(err)
		}
		file, err := cat.Upload(ctx, f.admin, raw.Bytes(), fmt.Sprintf("preview-image-%04d", i), "preview")
		if err != nil {
			t.Fatal(err)
		}
		in := catalog.Input{Code: fmt.Sprintf("PREVIEW-%d", i), Name: name,
			Attributes:  []catalog.Attribute{{Name: "包装", Value: "礼盒装"}},
			Description: "<p>此商品为本地自动化验收生成的自有测试数据。</p>",
			MediaIDs:    []string{file.ID}, Excluded: []string{}, Sort: i}
		p, err := cat.Create(ctx, f.admin, in, fmt.Sprintf("preview-product-%04d", i), "preview")
		if err != nil {
			t.Fatal(err)
		}
		status := catalog.StatusInput{Version: p.Version, Status: "ENABLED", Reason: "本地验收"}
		if _, err = cat.Status(ctx, f.admin, p.ID, status, fmt.Sprintf("preview-publish-%04d", i), "preview"); err != nil {
			t.Fatal(err)
		}
	}
}

// contentGiftImage 用途：生成不依赖外部版权素材的礼盒测试图片。
// 参数：index 商品序号，用于区分包装颜色。
// 返回值：480 像素方形合成图片，无外部文件读写。
func contentGiftImage(index int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 480, 480))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{246, 238, 225, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(100, 160, 380, 360), &image.Uniform{color.RGBA{uint8(130 + index*20), 45, 42, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(92, 140, 388, 188), &image.Uniform{color.RGBA{170, 65, 55, 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(223, 140, 257, 360), &image.Uniform{color.RGBA{227, 190, 126, 255}}, image.Point{}, draw.Src)
	return img
}
