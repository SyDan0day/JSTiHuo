const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const { solveChallenge } = require('./helpers.cjs');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';

/**
 * main 用途：验收 F2 反馈页面、后台处理、匿名错误和限流提示。
 * 参数：环境指定浏览器夹具会话目录与可选截图目录。
 * 返回值：验收 Promise，失败抛断言；截图不包含联系人或凭证。
 */
async function main() {
  const output =
    process.env.F2_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/f2');
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    for (const width of [390, 768, 1440]) {
      await page.setViewportSize({ width, height: 844 });
      for (const [name, route, heading] of [
        ['verify', '/verify', '卡券鉴真与提货'],
        ['query', '/orders/query', '查询提货订单'],
        ['feedback', '/feedback', '售后反馈'],
      ]) {
        await page.goto(base + route);
        await page.getByRole('heading', { name: heading, exact: true }).waitFor();
        await page.locator('img[alt="四位数字验证码"]').waitFor();
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
        assert.equal(await page.locator('main').count(), 1);
        await page.screenshot({ path: path.join(output, `${name}-${width}.png`), fullPage: true });
      }
    }
    await page.goto(base + '/feedback');
    const image = page.locator('img[alt="四位数字验证码"]');
    const answer = await solveChallenge(page, image);
    await page.getByLabel('联系人', { exact: true }).fill('本地测试');
    await page.getByLabel('手机', { exact: true }).fill('13900000000');
    await page.getByLabel('问题描述').fill('<script>window.feedbackBad=1</script>配送咨询');
    await page.getByLabel('验证码', { exact: true }).fill(answer);
    await page.getByRole('button', { name: '提交反馈', exact: true }).click();
    await page.getByRole('heading', { name: '反馈已收到' }).waitFor();
    assert.equal(await page.evaluate(() => window.feedbackBad), undefined);
    await page.screenshot({ path: path.join(output, 'feedback-success.png'), fullPage: true });
    const meta = JSON.parse(
      await fs.readFile(path.join(process.env.F1_BROWSER_DIR, 'session.json'), 'utf8'),
    );
    await context.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    await page.goto(base + '/admin/feedback');
    await page
      .getByRole('heading', { name: /反馈 .* OPEN/ })
      .first()
      .waitFor();
    await page.getByLabel('处理说明').first().fill('已核实本地测试反馈');
    await page.getByRole('button', { name: '标记已处理' }).first().click();
    await page.getByText('暂无反馈。', { exact: true }).waitFor();
    await page.getByLabel('状态', { exact: true }).selectOption('RESOLVED');
    await page.getByRole('button', { name: '刷新', exact: true }).click();
    await page.getByText('处理说明：已核实本地测试反馈', { exact: true }).waitFor();
    assert.equal(await page.evaluate(() => window.feedbackBad), undefined);
    await page.screenshot({ path: path.join(output, 'feedback-admin.png'), fullPage: true });
    await page.route('**/api/v1/challenges', (route) =>
      route.fulfill({
        status: 429,
        headers: { 'Content-Type': 'application/json', 'Retry-After': '7' },
        body: JSON.stringify({
          code: 'RATE_LIMITED',
          message: '请求未完成',
          request_id: 'test',
          field_errors: [],
          retryable: true,
        }),
      }),
    );
    await page.goto(base + '/feedback');
    await page
      .getByRole('status')
      .filter({ hasText: /等待 [1-7] 秒/ })
      .waitFor();
    await page.unroute('**/api/v1/challenges');
    assert.deepEqual(errors, []);
    await context.close();
    console.log(
      'F2 browser passed: feedback persistence, admin resolution, XSS text, retry countdown and three responsive widths.',
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
