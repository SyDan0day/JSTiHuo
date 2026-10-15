package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"jstihuo/internal/app/catalog"
)

// TestContentAuditRollback 用途：验证审计写入失败时内容和幂等记录一并回滚。
// 参数：t 测试上下文，需隔离 MySQL 与建库权限。
// 返回值：无，失败返回 500 且不存在部分内容或幂等记录。
func TestContentAuditRollback(t *testing.T) {
	f := newFulfillment(t)
	_, err := f.db.Exec(`CREATE TRIGGER content_audit_failure BEFORE INSERT ON audit_logs
		FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='synthetic audit failure'`)
	if err != nil {
		t.Fatal(err)
	}
	in := catalog.ContentInput{Slug: "audit-rollback", Title: "回滚测试", Kind: "PAGE", Published: true}
	response[map[string]any](t, f.client.call("POST", "admin/content", in, "admin", "content-rollback-01"), 500)
	for _, query := range []string{
		"SELECT COUNT(*) FROM content_pages WHERE slug='audit-rollback'",
		"SELECT COUNT(*) FROM idempotency_keys WHERE request_key='content-rollback-01'",
	} {
		var count int
		if err := f.db.QueryRow(query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("审计失败产生部分提交: %d, %v", count, err)
		}
	}
}

// TestContentConcurrentReplay 用途：验证同一管理员并发使用同键只创建一份内容和审计。
// 参数：t 测试上下文，需隔离 MySQL 与建库权限。
// 返回值：无，所有成功结果必须为同一对象。
func TestContentConcurrentReplay(t *testing.T) {
	f := newFulfillment(t)
	svc := f.orders.Redemption.Catalog
	in := catalog.ContentInput{Slug: "concurrent", Title: "并发创建", Kind: "PAGE"}
	var wg sync.WaitGroup
	ids, failures := make(chan string, 8), make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			content, err := svc.CreateContent(context.Background(), f.admin, in,
				"content-concurrent-01", "test")
			ids <- content.ID
			failures <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := ""
	for id := range ids {
		if first != "" && first != id {
			t.Fatal("同键并发创建了多个内容")
		}
		first = id
	}
	var count int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='createContent'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("审计重复: %d, %v", count, err)
	}
}

// TestContent 用途：验证真实内容 API 的发布、隔离、幂等、并发与权限。
// 参数：t 测试上下文，环境须提供可创建临时子库的 TEST_DB_DSN。
// 返回值：无，测试结束删除隔离子库，不访问业务库。
func TestContent(t *testing.T) {
	f := newFulfillment(t)
	c := f.client
	in := catalog.ContentInput{Slug: "notice", Title: "自有公告", Kind: "ANNOUNCEMENT",
		Body: `<p onclick="bad()">安全正文</p><script>alert(1)</script>`}
	created := response[catalog.Content](t, c.call("POST", "admin/content", in, "admin", "content-create-0001"), 201)
	if created.Body != "<p>安全正文</p>" || created.Version != 1 || created.Published {
		t.Fatal("清洗或草稿状态不符")
	}
	path := "admin/content/" + created.ID
	replay := response[catalog.Content](t, c.call("POST", "admin/content", in, "admin", "content-create-0001"), 201)
	if replay.ID != created.ID {
		t.Fatal("同键重复创建内容")
	}
	response[map[string]any](t, c.call("POST", "admin/content", in, "admin", "content-duplicate-01"), 409)
	in.Title = "不同请求"
	response[map[string]any](t, c.call("POST", "admin/content", in, "admin", "content-create-0001"), 409)
	response[map[string]any](t, c.call("GET", "content/notice", nil, "", ""), 404)
	page := response[catalog.Page[catalog.Content]](t, c.call("GET", "content?q=notice", nil, "", ""), 200)
	if len(page.Items) != 0 {
		t.Fatal("公开列表泄露草稿")
	}
	response[catalog.Content](t, c.call("GET", path, nil, "admin", ""), 200)
	update := catalog.ContentUpdate{ContentInput: in, Version: 1}
	update.Published = true
	response[catalog.Content](t, c.call("PATCH", path, update, "admin", "content-publish-001"), 200)
	response[catalog.Content](t, c.call("PATCH", path, update, "admin", "content-publish-001"), 200)
	public := response[catalog.Content](t, c.call("GET", "content/notice", nil, "", ""), 200)
	if !public.Published || public.Version != 2 {
		t.Fatal("公开详情未读取持久化版本")
	}
	response[map[string]any](t, c.call("PATCH", path, update, "admin", "content-stale-0001"), 409)
	for _, query := range []string{"status=DRAFT", "status=ENABLED", "unknown=1", "limit=0", "after_id=01", "sort=title", "q=a&q=b"} {
		response[map[string]any](t, c.call("GET", "content?"+query, nil, "", ""), 400)
	}
	response[map[string]any](t, c.call("PATCH", path, map[string]any{"title": "缺少快照"}, "admin", "content-missing-01"), 400)
	response[map[string]any](t, c.call("POST", "admin/content", update, "admin", "content-extra-001"), 400)
	in.Slug, in.Published = "guide", true
	second := response[catalog.Content](t, c.call("POST", "admin/content", in, "admin", "content-second-01"), 201)
	page = response[catalog.Page[catalog.Content]](t, c.call("GET", "content?limit=1", nil, "", ""), 200)
	if len(page.Items) != 1 || page.Items[0].ID != second.ID || page.Next != second.ID {
		t.Fatal("分页首项或游标不符")
	}
	page = response[catalog.Page[catalog.Content]](t, c.call("GET", "content?limit=1&after_id="+page.Next, nil, "", ""), 200)
	if len(page.Items) != 1 || page.Items[0].ID != created.ID || page.Next != "" {
		t.Fatal("分页漏项或重复")
	}
	// 固定 Cookie 和 CSRF 发起并发请求，不共享会修改令牌的测试客户端。
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			x := update
			x.Version, x.Published, x.Title = 2, false, fmt.Sprint("撤下", i)
			b, err := json.Marshal(x)
			if err != nil {
				results <- 0
				return
			}
			r := httptest.NewRequest("PATCH", "/api/v1/"+path, strings.NewReader(string(b)))
			r.AddCookie(c.cookies["jst-admin"])
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "http://localhost:8080")
			r.Header.Set("X-CSRF-Token", c.tokens["admin"])
			r.Header.Set("Idempotency-Key", fmt.Sprintf("content-race-%04d", i))
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			results <- w.Code
		}(i)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("并发写入结果不符: %v", counts)
	}
	response[map[string]any](t, c.call("GET", "content/notice", nil, "", ""), 404)
	var audits int
	if err := f.db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action IN ('createContent','updateContent')").Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("审计非原子或幂等重复: %d, %v", audits, err)
	}
	if _, err := f.db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='content.write'"); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, c.call("PATCH", path, update, "admin", "content-forbid-01"), 403)
	response[catalog.Content](t, c.call("GET", path, nil, "admin", ""), 200)
	c.tokens["admin"] = "invalid-csrf"
	response[map[string]any](t, c.call("POST", "admin/content", in, "admin", "content-csrf-0001"), 403)
	if _, err := f.db.Exec("DELETE rp FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE p.permission_key='content.read'"); err != nil {
		t.Fatal(err)
	}
	response[map[string]any](t, c.call("GET", path, nil, "admin", ""), 403)
	response[catalog.Content](t, c.call("GET", "content/guide", nil, "", ""), 200)
	delete(c.cookies, "jst-admin")
	response[map[string]any](t, c.call(http.MethodGet, path, nil, "", ""), 401)
}
