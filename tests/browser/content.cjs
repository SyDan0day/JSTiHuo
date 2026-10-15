const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');

const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';
const output =
  process.env.F1_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/f1');

/**
 * checkLayout 用途：检查断点内无整页溢出，并验证移动底栏不遮挡页尾。
 * 参数：page 当前浏览器页；width 当前视口宽度。
 * 返回值：完成校验的 Promise，布局违规抛断言错误。
 */
async function checkLayout(page, width) {
  assert(
    await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    `页面在 ${width}px 横向溢出`,
  );
  if (width === 390) {
    await page.evaluate(() => scrollTo(0, document.body.scrollHeight));
    const clear = await page.evaluate(() => {
      const nav = document.querySelector('.public-bottom-nav');
      const footer = document.querySelector('.public-footer');
      return (
        !nav || !footer || footer.getBoundingClientRect().bottom <= nav.getBoundingClientRect().top
      );
    });
    assert(clear, '移动底栏遮挡页尾');
    await page.evaluate(() => scrollTo(0, 0));
  }
}

/**
 * main 用途：在真实 Go/MySQL 夹具上验收 F1 页面、保存发布和异常恢复。
 * 参数：环境指定服务地址、临时管理员会话文件和 Playwright 模块位置。
 * 返回值：完成验收的 Promise，失败抛错误；截图不包含凭证。
 */
async function main() {
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    const products = await (await page.request.get(base + '/api/v1/products?limit=8')).json();
    assert(products.items.length >= 4, '夹具须提供四个已发布商品');
    const productPath = '/products/' + products.items[0].id;
    for (const width of [390, 768, 1440]) {
      await page.setViewportSize({ width, height: 844 });
      for (const [name, route, heading] of [
        ['home', '/', '美好心意'],
        ['products', '/products', '礼品目录'],
        ['guide', '/pages/guide', '提货须知'],
        ['product', productPath, products.items[0].name],
      ]) {
        await page.goto(base + route);
        await page.getByRole('heading', { level: 1, name: new RegExp(heading) }).waitFor();
        await page.waitForFunction(() => !document.querySelector('[aria-busy="true"]'));
        await page
          .locator('img')
          .evaluateAll((images) => Promise.all(images.map((img) => img.decode())));
        await checkLayout(page, width);
        await page.screenshot({
          path: path.join(output, `${name}-${width}.png`),
          fullPage: true,
        });
      }
    }
    await page.goto(base + '/');
    const recommendation = page.getByRole('region', { name: '服务推荐' });
    const firstTitle = await recommendation.getByRole('heading').textContent();
    await recommendation.getByRole('button', { name: '下一条推荐' }).click();
    assert.notEqual(await recommendation.getByRole('heading').textContent(), firstTitle);
    await recommendation.getByRole('button', { name: '上一条推荐' }).click();
    assert.equal(await recommendation.getByRole('heading').textContent(), firstTitle);
    await page.goto(base + '/');
    await page.getByRole('heading', { level: 1, name: /美好心意/ }).waitFor();
    await page.keyboard.press('Tab');
    assert.equal(
      await page.evaluate(() => document.activeElement.textContent.trim()),
      '跳到主要内容',
    );
    await page.keyboard.press('Enter');
    assert.equal(await page.evaluate(() => document.activeElement.id), 'public-main');
    await page.route('**/api/v1/products?limit=8', (route) => route.abort('failed'));
    await page.goto(base + '/');
    await page.getByRole('alert').filter({ hasText: '礼品加载失败' }).waitFor();
    await recommendation.getByRole('heading').waitFor();
    await page.unroute('**/api/v1/products?limit=8');
    await page.getByRole('button', { name: '重试', exact: true }).click();
    await page.locator('.product-card').first().waitFor();
    await page.goto(base + '/home/index/about_us.html');
    await page.waitForURL('**/pages/about');
    await page.getByRole('heading', { name: '关于礼品提货' }).waitFor();
    await page.getByRole('link', { name: '联系我们', exact: true }).click();
    await page.getByRole('heading', { name: '联系我们', exact: true }).waitFor();
    assert.equal(await (await page.request.get(base + '/api/v1/admin/content')).status(), 401);

    const meta = JSON.parse(
      await fs.readFile(
        process.env.F1_BROWSER_SESSION || path.join(process.env.F1_BROWSER_DIR, 'session.json'),
        'utf8',
      ),
    );
    await context.addCookies([
      {
        name: 'jst-admin',
        value: meta.cookie,
        url: base,
        httpOnly: true,
        sameSite: 'Strict',
      },
    ]);
    const slug = 'browser-' + Date.now();
    await page.goto(base + '/admin/content/new');
    await page.getByLabel('标题', { exact: true }).fill('浏览器内容发布验收');
    await page.getByLabel('slug（公开地址）').fill(slug);
    await page
      .getByLabel('正文 HTML')
      .fill('<p onclick="window.bad=1">浏览器发布正文</p><script>window.bad=1</script>');
    await page.getByRole('button', { name: '保存完整内容' }).click();
    await page.waitForURL(/\/admin\/content\/[0-9]+$/);
    await page.getByLabel('标题', { exact: true }).waitFor();
    const editURL = page.url();
    assert.equal((await page.request.get(base + '/api/v1/content/' + slug)).status(), 404);
    await page.getByLabel('发布到公开页面', { exact: false }).check();
    await page.getByRole('button', { name: '保存完整内容' }).click();
    await page.getByRole('status').filter({ hasText: '公开页面已发布' }).waitFor();
    await page.screenshot({
      path: path.join(output, 'content-editor-1440.png'),
      fullPage: true,
    });
    await page.goto(base + '/pages/' + slug);
    await page.getByRole('heading', { name: '浏览器内容发布验收' }).waitFor();
    assert.equal(await page.evaluate(() => window.bad), undefined);
    const clean = await (await page.request.get(base + '/api/v1/content/' + slug)).json();
    assert.equal(clean.body_html, '<p>浏览器发布正文</p>');

    // 丢弃已提交的响应，随后必须用同一键和正文确认，而不是重复编辑。
    await page.goto(editURL);
    await page.getByLabel('标题', { exact: true }).fill('响应丢失后的内容');
    let first = true;
    const keys = [];
    await page.route('**/api/v1/admin/content/*', async (route) => {
      if (route.request().method() !== 'PATCH') return route.continue();
      keys.push(route.request().headers()['idempotency-key']);
      if (!first) return route.continue();
      first = false;
      await route.fetch();
      await route.abort('failed');
    });
    await page.getByRole('button', { name: '保存完整内容' }).click();
    await page.getByRole('alert').filter({ hasText: '保存结果尚未确认' }).waitFor();
    assert(await page.getByLabel('标题', { exact: true }).isDisabled());
    await page.getByRole('button', { name: '使用原请求确认结果' }).click();
    await page.getByRole('status').filter({ hasText: '已保存' }).waitFor();
    assert.equal(keys.length, 2);
    assert.equal(keys[0], keys[1]);
    await page.unroute('**/api/v1/admin/content/*');
    await page.getByLabel('发布到公开页面', { exact: false }).uncheck();
    await page.getByRole('button', { name: '保存完整内容' }).click();
    await page.getByRole('status').filter({ hasText: '当前为草稿' }).waitFor();
    assert.equal((await page.request.get(base + '/api/v1/content/' + slug)).status(), 404);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      path: path.join(output, 'content-editor-390.png'),
      fullPage: true,
    });
    await checkLayout(page, 390);
    await page.goto(base + '/admin/content');
    await page.getByText('查看与编辑', { exact: true }).first().waitFor();
    await checkLayout(page, 390);
    await page.screenshot({
      path: path.join(output, 'content-list-390.png'),
      fullPage: true,
    });
    await page.goto(base + '/pages/' + slug);
    await page.getByRole('alert').filter({ hasText: '尚未发布或已撤下' }).waitFor();
    const session = await (await page.request.get(base + '/api/v1/admin/session')).json();
    for (const product of products.items) {
      const detail = await (
        await page.request.get(base + '/api/v1/admin/products/' + product.id)
      ).json();
      const disabled = await page.request.post(
        base + '/api/v1/admin/products/' + product.id + '/status',
        {
          headers: {
            Origin: base,
            'X-CSRF-Token': session.csrf_token,
            'Idempotency-Key': require('node:crypto').randomUUID(),
          },
          data: { version: detail.version, status: 'DISABLED', reason: 'F1 空目录验收' },
        },
      );
      assert.equal(disabled.status(), 200);
    }
    await page.goto(base + '/products');
    await page.getByText('暂无上架商品，请稍后再来看看。', { exact: true }).waitFor();
    await page.goto(base + '/');
    await page.getByText('礼品正在准备中，请稍后再来看看。', { exact: true }).waitFor();
    assert.deepEqual(errors, [], '出现浏览器运行错误');
    console.log(
      'F1 browser: 12 public screenshots + 3 admin screenshots; publish/unpublish, XSS, lost-response replay, keyboard, aliases and responsive layout passed.',
    );
    await context.close();
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
