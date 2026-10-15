const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';

/**
 * snapshots 用途：检查三档视口溢出并保存不含凭证的界面证据。
 * 参数：page 为真实浏览器页；output 为证据目录；name 为页面名。
 * 返回值：截图 Promise，横向溢出立即失败。
 */
async function snapshots(page, output, name) {
  for (const width of [390, 768, 1440]) {
    await page.setViewportSize({ width, height: 844 });
    assert(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      name + ' 横向溢出',
    );
    await page.screenshot({ path: path.join(output, `${name}-${width}.png`), fullPage: true });
  }
}

/**
 * main 用途：用真实 API 与 worker 验收售后、补发、面单和通知页面。
 * 参数：环境指定启用交易数据的浏览器夹具目录。
 * 返回值：验收 Promise，不记录卡密与原始联系人。
 */
async function main() {
  const output =
    process.env.G2_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/g2');
  await fs.mkdir(output, { recursive: true });
  const meta = JSON.parse(
    await fs.readFile(path.join(process.env.F1_BROWSER_DIR, 'session.json'), 'utf8'),
  );
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const customer = await browser.newContext();
    await customer.addCookies([
      {
        name: 'jst-order',
        value: meta.order_cookie,
        url: base,
        httpOnly: true,
        sameSite: 'Strict',
      },
    ]);
    const page = await customer.newPage();
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto(base + '/orders/after-sales');
    await page.getByRole('heading', { name: '售后申请与进度', exact: true }).waitFor();
    await page.getByLabel('申请类型', { exact: true }).selectOption('REPLACEMENT');
    const items = page.getByRole('checkbox');
    assert.equal(await items.count(), 2);
    await items.first().check();
    await page
      .getByLabel('申请原因（请勿填写完整地址或卡密）', { exact: true })
      .fill('浏览器验收补发申请');
    await page.getByRole('button', { name: '提交售后申请', exact: true }).click();
    await page.getByRole('heading', { name: /案件 \d+ · OPEN/ }).waitFor();
    await snapshots(page, output, 'customer-case');

    const admin = await browser.newContext();
    await admin.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    const ops = await admin.newPage();
    ops.on('pageerror', (e) => errors.push(e.message));
    await ops.goto(base + '/admin/after-sales?order_id=' + meta.order_id);
    await ops.getByRole('button', { name: /案件 \d+ · REPLACEMENT · OPEN/ }).click();
    await ops.getByLabel('处理说明', { exact: true }).fill('核对原订单，批准一次补发');
    await ops.getByRole('button', { name: '批准补发', exact: true }).click();
    await ops.getByRole('heading', { name: /案件 \d+ · APPROVED/ }).waitFor();
    await ops.getByLabel('启用的承运商 ID', { exact: true }).fill(meta.carrier_id);
    await ops.getByLabel('实际补发单号', { exact: true }).fill('BROWSER-REPLACEMENT-001');
    const shipped = ops.waitForResponse(
      (r) => r.url().endsWith('/shipments') && r.request().method() === 'POST',
    );
    await ops.getByRole('button', { name: '确认补发并扣实存', exact: true }).click();
    assert.equal((await shipped).status(), 200);
    await snapshots(ops, output, 'admin-case');

    await ops.goto(base + '/admin/orders/' + meta.label_order_id);
    await ops.getByLabel('操作原因', { exact: true }).fill('本地 MOCK 面单验收');
    await ops
      .getByRole('button', { name: '受理默认承运商自动面单（生成后需确认发货）', exact: true })
      .click();
    await ops.waitForURL(/\/admin\/jobs\/\d+$/);
    await ops.getByRole('link', { name: '使用此面单确认发货', exact: true }).waitFor();
    await snapshots(ops, output, 'label-ready');
    await ops.getByRole('link', { name: '使用此面单确认发货', exact: true }).click();
    await ops.getByRole('heading', { name: '全单人工发货', exact: true }).waitFor();
    await ops.getByLabel('运单号', { exact: true }).waitFor();
    assert((await ops.getByLabel('运单号', { exact: true }).inputValue()).startsWith('MOCK-'));
    await ops.getByRole('button', { name: '确认全单发货', exact: true }).click();
    await ops.getByRole('status').filter({ hasText: '已确认发货' }).waitFor();
    await ops.goto(base + '/admin/notifications');
    await ops
      .getByRole('button', { name: /通知 \d+.*MOCK.*SENT/ })
      .first()
      .waitFor();
    await snapshots(ops, output, 'notifications');
    assert.deepEqual(errors, []);
    await customer.close();
    await admin.close();
    console.log(
      'G2 browser passed: customer case, approval, replacement shipment, real label worker, bound shipment confirmation, MOCK notice and 390/768/1440 views.',
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
