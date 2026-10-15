const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs = require('node:fs/promises'),
  path = require('node:path'),
  assert = require('node:assert/strict');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';

/**
 * main 用途：从实际 Router 目录遍历全部静态与有夹具对象的活动路由。
 * 参数：环境提供当前隔离管理员会话与证据目录。
 * 返回值：验收 Promise，检查 JS 异常、500、三档横向溢出和页面主内容。
 */
async function main() {
  const output = path.resolve(__dirname, '../../docs/development/evidence/h1-routes');
  await fs.mkdir(output, { recursive: true });
  const meta = JSON.parse(
    await fs.readFile(path.join(process.env.F1_BROWSER_DIR, 'session.json'), 'utf8'),
  );
  const source = await fs.readFile(
    path.resolve(__dirname, '../../frontend/src/app/router.ts'),
    'utf8',
  );
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext();
    await context.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    const page = await context.newPage(),
      errors = [],
      failures = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('response', (response) => {
      if (response.status() >= 500)
        failures.push({ path: new URL(response.url()).pathname, status: response.status() });
    });
    const products = await (await page.request.get(base + '/api/v1/products')).json();
    const batches = await (await page.request.get(base + '/api/v1/admin/voucher-batches')).json();
    const content = await (await page.request.get(base + '/api/v1/admin/content')).json();
    const orders = await (await page.request.get(base + '/api/v1/admin/orders')).json();
    const routes = [...source.matchAll(/path:\s*'([^']+)'/g)]
      .map((match) => match[1])
      .filter((route) => !route.includes('pathMatch'));
    const entries = [];
    for (const template of [...new Set(routes)]) {
      let route = template.replace(':slug', 'guide');
      if (route.includes(':id')) {
        const id = route.includes('/products/')
          ? products.items[0]?.id
          : route.includes('/content/')
            ? content.items[0]?.id
            : route.includes('/voucher-batches/')
              ? batches.items[0]?.id
              : route.includes('/orders/')
                ? orders.items[0]?.id
                : undefined;
        if (!id) {
          assert(route.includes('/jobs/'), '路由缺少夹具对象：' + template);
          continue;
        }
        route = route.replace(':id', id);
      }
      await page.goto(base + route);
      await page.locator('main').first().waitFor();
      for (const width of [390, 768, 1440]) {
        await page.setViewportSize({ width, height: 844 });
        await page.evaluate(
          () =>
            new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
        );
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          route + ' ' + width + ' 横向溢出',
        );
        const mask = route === '/admin/vouchers' ? [page.locator('tbody td:nth-child(2)')] : [];
        await page.screenshot({
          path: path.join(output, template.replace(/[^a-zA-Z0-9-]/g, '_') + '-' + width + '.png'),
          fullPage: true,
          mask,
        });
      }
      entries.push({
        template,
        path: route,
        resolved: new URL(page.url()).pathname,
        headings: await page.locator('h1').allTextContents(),
        widths: [390, 768, 1440],
      });
    }
    assert.deepEqual(errors, []);
    assert.deepEqual(failures, []);
    await fs.writeFile(
      path.join(output, 'routes.json'),
      JSON.stringify(
        {
          entries,
          note: '任务动态路由由 G1/G2 实际 worker 浏览器用例覆盖；缺少顾客 Cookie 的页面验证真实拒绝与恢复状态。',
        },
        null,
        2,
      ) + '\n',
    );
    console.log(
      'H1 route browser passed: ' +
        entries.length +
        ' route templates, three widths, no JS exception, no server 500; card code cells masked.',
    );
    await context.close();
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
