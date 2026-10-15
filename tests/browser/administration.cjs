const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';

/**
 * capture 用途：验证三档视口并保存不含临时密码的界面证据。
 * 参数：page 为页面；output 为目录；name 为截图类别。
 * 返回值：截图 Promise，横向溢出即失败。
 */
async function capture(page, output, name) {
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
 * main 用途：验证真实权限、管理员、设置、审计、地区和本地二维码界面。
 * 参数：环境提供隔离浏览器夹具目录。
 * 返回值：验收 Promise，不输出临时密码与会话。
 */
async function main() {
  const output =
    process.env.G3_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/g3');
  await fs.mkdir(output, { recursive: true });
  const meta = JSON.parse(
    await fs.readFile(path.join(process.env.F1_BROWSER_DIR, 'session.json'), 'utf8'),
  );
  const suffix = Date.now().toString(36),
    roleName = '浏览器订单只读-' + suffix;
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext();
    await context.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    const page = await context.newPage(),
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.goto(base + '/admin');
    await page.getByRole('heading', { name: '业务统计', exact: true }).waitFor();
    await capture(page, output, 'dashboard');
    await page.goto(base + '/admin/roles');
    await page.getByLabel('角色名称', { exact: true }).fill(roleName);
    await page.getByRole('checkbox', { name: /^order\.read ·/ }).check();
    await page.getByRole('checkbox', { name: /^audit\.read ·/ }).check();
    await page.getByRole('button', { name: '保存角色权限', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '已保存' }).waitFor();
    await capture(page, output, 'roles');
    await page.goto(base + '/admin/admins');
    await page.getByLabel('登录名', { exact: true }).fill('browser-readonly-' + suffix);
    await page
      .getByLabel('临时密码（首次登录必须改密）', { exact: true })
      .fill('Synthetic-Browser-Temporary-123');
    await page.getByRole('checkbox', { name: new RegExp('^' + roleName) }).check();
    await page.getByRole('button', { name: '保存管理员', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '已保存' }).waitFor();
    assert.equal(await page.locator('input[type=password]').count(), 0);
    await capture(page, output, 'admins');
    await page.goto(base + '/admin/settings');
    await page.getByLabel('每日容量', { exact: true }).fill('200');
    await page.getByLabel('业务通知', { exact: true }).selectOption('DISABLED');
    await page.getByRole('button', { name: '保存固定配置', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '配置已保存' }).waitFor();
    await capture(page, output, 'settings');
    await page.goto(base + '/admin/audit?entity_type=role');
    await page.getByText('createRole · SUCCESS', { exact: true }).first().waitFor();
    await capture(page, output, 'audit');
    await page.goto(base + '/admin/regions');
    await page.getByRole('button', { name: '北京市 · 110000', exact: true }).click();
    await page.getByText('父级 110000', { exact: true }).waitFor();
    await page.locator('li button').first().click();
    await page.getByText('父级 110100', { exact: true }).waitFor();
    await capture(page, output, 'regions');
    await page.goto(base + '/admin/tools/qr');
    await page.getByRole('link', { name: '下载公开入口二维码 PNG', exact: true }).waitFor();
    let writes = 0;
    page.on('request', (request) => {
      if (request.url().includes('/api/') && request.method() !== 'GET') writes++;
    });
    await page.getByLabel('入口页面', { exact: true }).selectOption('/products');
    await page
      .getByRole('paragraph')
      .filter({ hasText: base + '/products' })
      .waitFor();
    const downloading = page.waitForEvent('download');
    await page.getByRole('link', { name: '下载公开入口二维码 PNG', exact: true }).click();
    const download = await downloading,
      stream = await download.createReadStream(),
      chunks = [];
    for await (const chunk of stream) chunks.push(chunk);
    assert.deepEqual(
      Buffer.concat(chunks).subarray(0, 8),
      Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    );
    assert.equal(writes, 0);
    await capture(page, output, 'public-qr');
    await page.goto(base + '/admin/files');
    await page.getByRole('heading', { name: '受控文件目录', exact: true }).waitFor();
    await capture(page, output, 'file-directory');
    assert.deepEqual(errors, []);
    await context.close();
    console.log(
      'G3 browser passed: real role/admin writes, fixed config, dashboard/audit/files, read-only regions, local PNG QR, no secrets in screenshots and three widths.',
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
