const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { randomUUID } = require('node:crypto');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';

/**
 * main 用途：在真实 worker 上验收 G1 上传、预检确认、错误文件和导出页面。
 * 参数：环境指定启用 F1_BROWSER_WORKER 的浏览器夹具目录。
 * 返回值：验收 Promise，失败只输出状态和断言，不打印卡密。
 */
async function main() {
  const output =
    process.env.G1_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/g1');
  await fs.mkdir(output, { recursive: true });
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    const meta = JSON.parse(
      await fs.readFile(path.join(process.env.F1_BROWSER_DIR, 'session.json'), 'utf8'),
    );
    await context.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    await page.goto(base + '/admin/files');
    await page.getByRole('heading', { name: '导入与导出', exact: true }).waitFor();
    const session = await (await page.request.get(base + '/api/v1/admin/session')).json();
    const products = await (await page.request.get(base + '/api/v1/products')).json();
    const send = async (url, data) => {
      const response = await page.request.post(base + url, {
        headers: {
          Origin: base,
          'X-CSRF-Token': session.csrf_token,
          'Idempotency-Key': randomUUID(),
        },
        data,
      });
      assert(response.ok(), '夹具操作状态 ' + response.status());
      return response.json();
    };
    const type = await send('/api/v1/admin/voucher-types', {
      name: '浏览器导入卡型',
      selection_mode: 'ONE_OF',
      selection_count: 1,
    });
    const batch = await send('/api/v1/admin/voucher-batches', {
      type_id: type.id,
      label: '浏览器导入测试',
      product_ids: [products.items[0].id],
      quantity: 2,
      code_prefix: 'BR',
      code_length: 12,
      code_mode: 'RANDOM',
      secret_length: 12,
      secret_format: 'MIXED',
      valid_from: '2026-10-01T00:00:00Z',
      valid_until: '2027-10-01T00:00:00Z',
      activate_on_seal: false,
    });
    await page.reload();
    await page.getByRole('heading', { name: '导入与导出', exact: true }).waitFor();
    for (const width of [390, 768, 1440]) {
      await page.setViewportSize({ width, height: 844 });
      assert(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        '文件页横向溢出',
      );
      await page.screenshot({ path: path.join(output, `files-${width}.png`), fullPage: true });
    }
    const prefix = randomUUID().replaceAll('-', '').slice(0, 10).toUpperCase();
    const raw = Buffer.from(
      `code,secret,valid_from,valid_until\n${prefix}01,ABCDEFGHJK23,2026-10-01T00:00:00Z,2027-10-01T00:00:00Z\n${prefix}02,ABCDEFGHJK23,2026-10-01T00:00:00Z,2027-10-01T00:00:00Z\n`,
    );
    await page
      .getByLabel('CSV 文件')
      .setInputFiles({ name: 'import.csv', mimeType: 'text/csv', buffer: raw });
    await page.getByRole('button', { name: '加密上传', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '已暂存文件' }).waitFor();
    await page.getByLabel('草稿批次 ID').fill(batch.id);
    await page.getByRole('button', { name: '创建预检任务', exact: true }).click();
    await page.waitForURL(/\/admin\/jobs\/[0-9]+$/);
    await page.getByRole('button', { name: '确认执行实际导入', exact: true }).waitFor();
    const jobURL = page.url();
    for (const width of [390, 768, 1440]) {
      await page.setViewportSize({ width, height: 844 });
      assert(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        '任务页横向溢出',
      );
      await page.screenshot({ path: path.join(output, `preview-${width}.png`), fullPage: true });
    }
    await page.getByLabel('确认说明').fill('核对本地预检');
    await page.getByRole('button', { name: '确认执行实际导入', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '实际处理已结束。' }).waitFor();
    const vouchers = await (
      await page.request.get(base + '/api/v1/admin/vouchers?batch_id=' + batch.id)
    ).json();
    assert.equal(vouchers.items.length, 2);
    assert(vouchers.items.every((item) => item.credential_state === 'SEALED' && !item.active));
    await page.goto(base + '/admin/files');
    await page.getByLabel('报表种类').selectOption('VOUCHER_METADATA');
    await page.getByLabel('批次 ID（可选）').fill(batch.id);
    await page.getByRole('button', { name: '创建脱敏导出任务', exact: true }).click();
    await page.getByRole('button', { name: '下载脱敏结果文件', exact: true }).waitFor();
    const downloading = page.waitForEvent('download');
    await page.getByRole('button', { name: '下载脱敏结果文件', exact: true }).click();
    const download = await downloading;
    const stream = await download.createReadStream();
    const chunks = [];
    for await (const chunk of stream) chunks.push(chunk);
    const csv = Buffer.concat(chunks).toString('utf8');
    assert(csv.includes('****') && !csv.includes('ABCDEFGHJK23') && !csv.includes(prefix + '01'));
    await page.screenshot({ path: path.join(output, 'export-ready.png'), fullPage: true });
    await page.goto(jobURL);
    await page.getByRole('status').filter({ hasText: '实际处理已结束。' }).waitFor();
    assert.deepEqual(errors, []);
    await context.close();
    console.log(
      'G1 browser passed: encrypted upload, real preview/commit worker, sealed vouchers, masked CSV download and three responsive widths.',
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
