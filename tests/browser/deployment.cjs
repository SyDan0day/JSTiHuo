const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const { solveChallenge } = require('./helpers.cjs');
const assert = require('node:assert/strict'),
  fs = require('node:fs/promises'),
  path = require('node:path');
const base = process.env.DEPLOYMENT_BASE || 'http://localhost:28080';

/**
 * fillSecret 用途：填写仅由管道传入的运维演练口令。
 * 参数：locator 为输入控件；value 为内存口令。
 * 返回值：填写 Promise，失败不回显口令。
 */
async function fillSecret(locator, value) {
  try {
    await locator.fill(value);
  } catch {
    throw new Error('运维演练口令输入失败');
  }
}

/**
 * login 用途：经真实挑战登录已初始化的演练账户。
 * 参数：page 为页面；credentials 为仅驻留内存的账号口令。
 * 返回值：登录 Promise，首次登录完成强制改密并重新登录。
 */
async function login(page, credentials) {
  await page.goto(base + '/admin/login');
  await page.getByLabel('账号', { exact: true }).fill(credentials.login);
  await fillSecret(page.getByLabel('密码', { exact: true }), credentials.password);
  await page
    .getByLabel('图片验证码', { exact: true })
    .fill(await solveChallenge(page, page.getByAltText('四位数字验证码')));
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.waitForURL((url) => ['/admin', '/admin/change-password'].includes(url.pathname));
  if (new URL(page.url()).pathname === '/admin/change-password') {
    await fillSecret(page.getByLabel('当前密码', { exact: true }), credentials.password);
    await fillSecret(page.getByLabel('新密码', { exact: true }), credentials.permanent);
    await fillSecret(page.getByLabel('确认新密码', { exact: true }), credentials.permanent);
    await page.getByRole('button', { name: '保存并重新登录', exact: true }).click();
    await page.waitForURL('**/admin/login');
    credentials.password = credentials.permanent;
    await login(page, credentials);
  }
}

/**
 * main 用途：在实际容器和静态代理中验收初始化、PII、图片、生成或恢复读验。
 * 参数：stdin 接收演练凭据；verify 参数仅校验恢复后的现有数据。
 * 返回值：验收 Promise，stdout 仅输出对象引用和计数，不输出口令。
 */
async function main() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const credentials = JSON.parse(Buffer.concat(chunks).toString('utf8'));
  chunks.forEach((chunk) => chunk.fill(0));
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: base.startsWith('https:') });
    const page = await context.newPage(),
      errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await login(page, credentials);
    if (base.startsWith('https:')) {
      const cookie = (await context.cookies()).find((cookie) => cookie.name === '__Host-jst-admin');
      assert(
        cookie &&
          cookie.httpOnly &&
          cookie.secure &&
          cookie.path === '/' &&
          cookie.sameSite === 'Strict',
        'HTTPS Cookie 策略不符',
      );
    }
    await page.goto(base + '/admin/security');
    await fillSecret(page.getByLabel('当前密码', { exact: true }), credentials.password);
    await page.getByRole('button', { name: '确认身份', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '密码复核成功' }).waitFor();
    const verify = process.argv.includes('verify');
    let product, batch, feedback;
    if (!verify) {
      await page.goto(base + '/admin/products/new');
      await page.getByLabel('商品编码', { exact: true }).fill('DEPLOYMENT-OWN-001');
      await page.getByLabel('商品名称', { exact: true }).fill('部署演练自有礼品');
      const upload = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/admin/files') && r.request().method() === 'POST',
      );
      await page
        .getByLabel('上传 PNG/JPEG（≤5 MiB）', { exact: true })
        .setInputFiles(path.resolve(__dirname, '../fixtures/own-gift.png'));
      assert.equal((await upload).status(), 201);
      await page.getByRole('button', { name: '保存商品', exact: true }).click();
      await page.waitForURL(/\/products\/\d+\/edit$/);
      product = page.url().split('/').at(-2);
      await page.getByRole('button', { name: '上架', exact: true }).click();
      await page.getByRole('status').filter({ hasText: '商品已上架' }).waitFor();
      await page.goto(base + '/feedback');
      await page.getByLabel('联系人', { exact: true }).fill('部署演练联系资料');
      await page.getByLabel('手机', { exact: true }).fill('13900000000');
      await page.getByLabel('问题描述', { exact: true }).fill('部署恢复演练问题记录');
      await page
        .getByLabel('验证码', { exact: true })
        .fill(await solveChallenge(page, page.getByAltText('四位数字验证码')));
      const submitted = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/feedback') && r.request().method() === 'POST',
      );
      await page.getByRole('button', { name: '提交反馈', exact: true }).click();
      const created = await submitted;
      assert.equal(created.status(), 200);
      feedback = (await created.json()).id;
      await page.getByRole('heading', { name: '反馈已收到', exact: true }).waitFor();
      await page.goto(base + '/admin/voucher-batches');
      await page.getByLabel('名称', { exact: true }).fill('部署演练卡型');
      await page.getByRole('button', { name: '保存卡型', exact: true }).click();
      const option = page.getByLabel(/^卡型/).locator('option').filter({ hasText: '部署演练卡型' });
      await option.waitFor({ state: 'attached' });
      await page.getByLabel(/^卡型/).selectOption(await option.getAttribute('value'));
      await page.getByLabel('批次名称', { exact: true }).fill('恢复后不恢复凭证导出');
      await page.getByRole('checkbox', { name: /部署演练自有礼品/ }).check();
      await page.getByLabel('张数', { exact: true }).fill('1');
      await page.getByLabel('生效时间', { exact: true }).fill('2026-10-01T00:00');
      await page.getByLabel('到期时间', { exact: true }).fill('2027-10-01T00:00');
      await page.getByRole('button', { name: '保存草稿', exact: true }).click();
      await page.waitForURL(/\/voucher-batches\/\d+$/);
      batch = page.url().split('/').pop();
      await page.getByRole('button', { name: '提交生成任务', exact: true }).click();
      await page.getByRole('button', { name: '下载卡密文件', exact: true }).waitFor();
    }
    const products = await (await page.request.get(base + '/api/v1/products')).json();
    assert.equal(products.items.length, 1);
    assert.equal(products.items[0].name, '部署演练自有礼品');
    const image = await page.request.get(new URL(products.items[0].image_urls[0], base).href);
    assert.equal(image.status(), 200);
    assert((await image.body()).length > 0);
    await page.goto(base + '/admin/feedback');
    await page.getByText('部署恢复演练问题记录', { exact: true }).waitFor();
    await page.goto(base + '/admin/files');
    await page.getByRole('heading', { name: '受控文件目录', exact: true }).waitFor();
    const restored = process.argv.includes('restored');
    if (restored) {
      const files = await (await page.request.get(base + '/api/v1/admin/files')).json();
      assert(
        files.items
          .filter((file) => file.kind !== 'IMAGE')
          .every((file) => ['EXPIRED', 'DELETED'].includes(file.state)),
        '恢复后的私有导出仍可用',
      );
      const batches = await (await page.request.get(base + '/api/v1/admin/voucher-batches')).json();
      assert(
        batches.items.every((batch) => batch.state === 'SEALED'),
        '恢复后的生成批次未封存',
      );
    }
    assert.deepEqual(errors, []);
    const output = path.resolve(__dirname, '../../docs/development/evidence/h2');
    await fs.mkdir(output, { recursive: true });
    await page.screenshot({
      path: path.join(
        output,
        restored ? 'restored-files.png' : verify ? 'https-files.png' : 'deployed-files.png',
      ),
      fullPage: true,
    });
    console.log(
      JSON.stringify({
        verified: verify,
        restored,
        product_id: product,
        batch_id: batch,
        feedback_id: feedback,
        products: products.items.length,
        tls: base.startsWith('https:'),
      }),
    );
    credentials.password = '';
    credentials.permanent = '';
    await context.close();
  } finally {
    await browser.close();
  }
}
main().catch(() => {
  console.error('部署浏览器验收失败；口令与联系人不写异常输出。');
  process.exitCode = 1;
});
