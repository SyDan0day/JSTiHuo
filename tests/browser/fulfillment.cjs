const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { randomUUID, createHash } = require('node:crypto');
const { solveChallenge } = require('./helpers.cjs');
const base = process.env.F1_BROWSER_BASE || 'http://localhost:8080';
const directory = process.env.F1_BROWSER_DIR;
const output =
  process.env.H1_BROWSER_OUTPUT || path.resolve(__dirname, '../../docs/development/evidence/h1');

/**
 * sensitive 用途：填写测试凭证并屏蔽自动化异常中的输入值。
 * 参数：locator 为真实表单控件；value 为仅驻留内存的测试凭证。
 * 返回值：填写 Promise，异常不附带原输入。
 */
async function sensitive(locator, value) {
  try {
    await locator.fill(value);
  } catch {
    throw new Error('测试凭证输入未能完成');
  }
}

/**
 * capture 用途：验收三档视口并保存无卡密的界面证据。
 * 参数：page 为页面；name 为证据类别。
 * 返回值：视口截图 Promise，横向溢出或重复布局立即失败。
 */
async function capture(page, name) {
  for (const width of [390, 768, 1440]) {
    await page.setViewportSize({ width, height: 844 });
    await page.evaluate(
      () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
    );
    assert.equal(
      await page.locator('.public-header').count(),
      name.startsWith('admin') || name === 'reconciliation' ? 0 : 1,
    );
    assert(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      name + ' 横向溢出',
    );
    await page.screenshot({
      path: path.join(output, `${name}-${width}.png`),
      fullPage: false,
      animations: 'disabled',
    });
  }
}

/**
 * login 用途：通过真实图形挑战登录管理员。
 * 参数：page 为页面；name 为隔离账号；password 为内存口令。
 * 返回值：登录 Promise，不保存含口令截图。
 */
async function login(page, name, password) {
  await page.goto(base + '/admin/login');
  await page.getByLabel('账号', { exact: true }).fill(name);
  await sensitive(page.getByLabel('密码', { exact: true }), password);
  const answer = await solveChallenge(page, page.getByAltText('四位数字验证码'));
  await page.getByLabel('图片验证码', { exact: true }).fill(answer);
  await page.getByRole('button', { name: '登录', exact: true }).click();
  await page.waitForURL((url) => ['/admin', '/admin/change-password'].includes(url.pathname));
}

/**
 * advance 用途：推进仅本地构建标签夹具的测试时钟。
 * 参数：seconds 为正向秒数；pages 为需同步 Date 的浏览器页面。
 * 返回值：推进 Promise，不开放生产 HTTP 时钟接口。
 */
async function advance(seconds, pages) {
  const id = randomUUID();
  await fs.writeFile(
    path.join(directory, 'clock.json'),
    JSON.stringify({ id, advance_seconds: seconds }),
    { mode: 0o600 },
  );
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    let ack;
    try {
      ack = JSON.parse(await fs.readFile(path.join(directory, 'clock-ack.json'), 'utf8'));
    } catch {}
    if (ack?.id === id) {
      for (const page of pages) await page.clock.setFixedTime(new Date(ack.current_time));
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('隔离时钟未应答');
}

/**
 * refreshTracking 用途：通过真实刷新按钮等待异步 worker 已发布的轨迹。
 * 参数：page 为物流页面；description 为固定 MOCK 文案。
 * 返回值：读取 Promise，十秒内未出现则验收失败。
 */
async function refreshTracking(page, description) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (await page.getByText(description, { exact: false }).count()) return;
    await page.getByRole('button', { name: '刷新物流', exact: true }).click();
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('MOCK worker 轨迹未及时发布');
}

/**
 * createProducts 用途：经真实上传、保存、发布 UI 建立三种商品。
 * 参数：page 为已登录管理页；suffix 为隔离测试后缀。
 * 返回值：商品引用列表，不写测试数据库捷径。
 */
async function createProducts(page, suffix) {
  const products = [];
  const publicPage = await (await page.request.get(base + '/api/v1/products')).json();
  const png = await (
    await page.request.get(new URL(publicPage.items[0].image_urls[0], base).href)
  ).body();
  for (const letter of ['A', 'B', 'C']) {
    await page.goto(base + '/admin/products/new');
    await page.getByLabel('商品编码', { exact: true }).fill('H1-' + suffix + '-' + letter);
    await page.getByLabel('商品名称', { exact: true }).fill('验收礼品 ' + letter + ' ' + suffix);
    const uploaded = page.waitForResponse(
      (r) => r.url().endsWith('/api/v1/admin/files') && r.request().method() === 'POST',
    );
    await page
      .getByLabel('上传 PNG/JPEG（≤5 MiB）', { exact: true })
      .setInputFiles({ name: 'own-fixture.png', mimeType: 'image/png', buffer: png });
    assert.equal((await uploaded).status(), 201);
    await page.getByRole('button', { name: '保存商品', exact: true }).click();
    await page.waitForURL(/\/admin\/products\/\d+\/edit$/);
    const id = /\/products\/(\d+)\//.exec(page.url())[1];
    await page.getByRole('button', { name: '上架', exact: true }).click();
    await page.getByRole('status').filter({ hasText: '商品已上架' }).waitFor();
    products.push({ id, letter, name: '验收礼品 ' + letter + ' ' + suffix });
  }
  return products;
}

/**
 * receiveStock 用途：经真实库存单 UI 为三个商品入库并审核。
 * 参数：page 为管理员页；products 为商品引用。
 * 返回值：提交 Promise，每商品实际入库十件。
 */
async function receiveStock(page, products) {
  await page.goto(base + '/admin/inventory/documents');
  await page.getByRole('button', { name: '新建单据', exact: true }).click();
  for (let i = 0; i < products.length; i++) {
    if (i) await page.getByRole('button', { name: '添加商品', exact: true }).click();
    await page.getByLabel('商品 ID', { exact: true }).nth(i).fill(products[i].id);
    await page.getByRole('spinbutton').nth(i).fill('10');
  }
  const creating = page.waitForResponse(
    (r) => r.url().endsWith('/api/v1/admin/inventory/documents') && r.request().method() === 'POST',
  );
  await page.getByRole('button', { name: '提交待审核', exact: true }).click();
  const createdResponse = await creating;
  assert.equal(createdResponse.status(), 201);
  const created = await createdResponse.json();
  const row = page.getByRole('row').filter({ hasText: created.document_no });
  await row.getByRole('button', { name: '批准', exact: true }).click();
  const approving = page.waitForResponse((r) =>
    r.url().endsWith('/inventory/documents/' + created.id + '/approve'),
  );
  await page.getByRole('button', { name: '确认审核', exact: true }).click();
  const approved = await approving;
  assert.equal(approved.status(), 200);
  assert((await approved.json()).state === 'APPROVED');
  await row.getByText('已批准', { exact: true }).waitFor();
}

/**
 * issueCards 用途：经卡型、草稿、生成、受控下载、封存和激活 UI 发行两张卡。
 * 参数：page 为近期复核管理员页；suffix 为唯一名称；products 为权益商品。
 * 返回值：仅驻留 Node 内存的凭证行与批次引用。
 */
async function issueCards(page, suffix, products) {
  // 拦截客户端的程序化下载点击，真实 HTTP 字节留在 Blob/Node 内存，不落下载目录。
  await page.addInitScript(() => {
    const blobs = new Map(),
      create = URL.createObjectURL.bind(URL),
      click = HTMLAnchorElement.prototype.click;
    URL.createObjectURL = function (blob) {
      const url = create(blob);
      blobs.set(url, blob);
      return url;
    };
    HTMLAnchorElement.prototype.click = function () {
      if (this.download && blobs.has(this.href)) {
        window.__h1Download = blobs.get(this.href);
        blobs.delete(this.href);
        return;
      }
      return click.call(this);
    };
  });
  await page.goto(base + '/admin/voucher-batches');
  await page.getByLabel('名称', { exact: true }).fill('H1 卡型 ' + suffix);
  await page.getByLabel(/^选品规则/).selectOption('N_OF');
  await page.getByLabel('种数', { exact: true }).fill('2');
  await page.getByRole('button', { name: '保存卡型', exact: true }).click();
  const typeOption = page
    .getByLabel(/^卡型/)
    .locator('option')
    .filter({ hasText: 'H1 卡型 ' + suffix });
  await typeOption.waitFor({ state: 'attached' });
  await page.getByLabel(/^卡型/).selectOption(await typeOption.getAttribute('value'));
  await page.getByLabel('批次名称', { exact: true }).fill('H1 发行 ' + suffix);
  for (const product of products)
    await page.getByRole('checkbox', { name: new RegExp(product.name + ' ·') }).check();
  await page.getByLabel('张数', { exact: true }).fill('2');
  await page.getByLabel('生效时间', { exact: true }).fill('2026-10-01T00:00');
  await page.getByLabel('到期时间', { exact: true }).fill('2027-10-01T00:00');
  await page.getByRole('button', { name: '保存草稿', exact: true }).click();
  await page.waitForURL(/\/admin\/voucher-batches\/\d+$/);
  const batchID = page.url().split('/').pop(),
    batchURL = page.url();
  await page.getByRole('button', { name: '提交生成任务', exact: true }).click();
  await page.getByRole('button', { name: '下载卡密文件', exact: true }).waitFor();
  await page.getByRole('button', { name: '下载卡密文件', exact: true }).click();
  await page.waitForFunction(() => !!window.__h1Download);
  const csv = await page.evaluate(async () => {
    const blob = window.__h1Download;
    delete window.__h1Download;
    return blob.text();
  });
  const rows = csv
    .trim()
    .split(/\r?\n/)
    .map((line) => line.split(','));
  assert(rows.length === 3 && rows[0][2] === 'secret', '内存凭证数量或列不符');
  await page.getByLabel('封存原因', { exact: true }).fill('本地验收已接收凭证');
  await page.getByRole('button', { name: '确认封存', exact: true }).click();
  await page.getByText(/· SEALED$/).waitFor();
  await page.getByRole('link', { name: '查看本批卡券', exact: true }).click();
  const checkboxes = page.getByRole('checkbox', { name: /^选择卡券 / });
  await checkboxes.first().waitFor();
  assert.equal(await checkboxes.count(), 2);
  for (let i = 0; i < 2; i++) await checkboxes.nth(i).check();
  await page.getByLabel('原因', { exact: true }).fill('封存后激活');
  await page.getByRole('button', { name: '确认批量操作', exact: true }).click();
  await page.getByText('已激活 / 未消费', { exact: true }).first().waitFor();
  return { cards: rows.slice(1), batchID, batchURL };
}

/**
 * redeem 用途：在真实顾客界面鉴真、限选、填写地址并模拟提交响应丢失。
 * 参数：page 为顾客页；card 为内存凭证行；weak 是否执行弱网分支。
 * 返回值：脱敏订单引用，卡密与完整地址不进入截图。
 */
async function redeem(page, card, weak) {
  await page.goto(base + '/verify');
  await sensitive(page.getByLabel('卡号', { exact: true }), card[1]);
  await sensitive(page.getByLabel('卡密', { exact: true }), card[2]);
  await page
    .getByLabel('四位图片验证码', { exact: true })
    .fill(await solveChallenge(page, page.getByAltText('四位数字验证码')));
  await page.getByRole('button', { name: '鉴真并继续', exact: true }).click();
  await page.waitForURL('**/redeem/select');
  const choices = page.getByRole('button', { name: '选择商品', exact: true });
  await choices.nth(0).click();
  await choices.nth(0).click();
  assert(await choices.nth(0).isDisabled(), '不能限制第三种商品');
  if (weak) await capture(page, 'selection');
  await page.getByRole('button', { name: '继续填写收货信息', exact: true }).click();
  await page.getByLabel('姓名', { exact: true }).fill('本地验收收货人');
  await page.getByLabel('手机', { exact: true }).fill('13800000000');
  await page.getByLabel('省份', { exact: true }).selectOption('110000');
  await page.getByLabel('城市', { exact: true }).selectOption('110100');
  await page.getByLabel('区县', { exact: true }).selectOption('110101');
  await page.getByLabel('详细地址', { exact: true }).fill('本地验收用虚构地址一号');
  const date = await page
    .getByLabel('预约发货日', { exact: true })
    .locator('option:not([disabled])')
    .first()
    .getAttribute('value');
  await page.getByLabel('预约发货日', { exact: true }).selectOption(date);
  await page.getByRole('button', { name: '核对并继续', exact: true }).click();
  await page.waitForURL('**/redeem/confirm');
  const attempts = [];
  let order, inputBody, csrf;
  await page.route('**/api/v1/redemptions', async (route) => {
    const request = route.request();
    attempts.push({
      key: request.headers()['idempotency-key'],
      hash: createHash('sha256').update(request.postData()).digest('hex'),
    });
    inputBody = request.postData();
    csrf = request.headers()['x-csrf-token'];
    const response = await route.fetch();
    if (response.status() !== 201 && response.status() !== 200) {
      const failure = await response.json();
      console.error('兑换接口拒绝：' + response.status() + ' ' + failure.code);
      await route.fulfill({ response });
      return;
    }
    order = await response.json();
    if (weak && attempts.length === 1) await route.abort('failed');
    else await route.fulfill({ response });
  });
  await page.getByRole('button', { name: '确认提交', exact: true }).dblclick({ delay: 10 });
  if (weak) {
    await page.getByRole('button', { name: '重试原请求', exact: true }).waitFor();
    await page.getByRole('button', { name: '重试原请求', exact: true }).click();
  }
  await page.waitForURL('**/orders/result');
  assert.equal(attempts.length, weak ? 2 : 1);
  if (weak)
    assert(
      attempts[0].key === attempts[1].key && attempts[0].hash === attempts[1].hash,
      '弱网重试改变命令',
    );
  await page.unroute('**/api/v1/redemptions');
  return { order, inputBody, csrf };
}

/**
 * main 用途：执行发行至模拟签收的完整浏览器主链与取消、弱网、账本分支。
 * 参数：环境提供无预置交易数据的真实 API/worker 夹具。
 * 返回值：验收 Promise，凭证明文只驻留进程内存。
 */
async function main() {
  await fs.mkdir(output, { recursive: true });
  const meta = JSON.parse(await fs.readFile(path.join(directory, 'session.json'), 'utf8'));
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  try {
    const context = await browser.newContext(),
      setup = await context.newPage();
    await context.addCookies([
      { name: 'jst-admin', value: meta.cookie, url: base, httpOnly: true, sameSite: 'Strict' },
    ]);
    await setup.goto(base + '/admin/admins');
    const suffix = Date.now().toString(36),
      account = 'h1-browser-' + suffix,
      temporary = 'Synthetic-Temporary-123',
      password = 'Synthetic-Permanent-123';
    await setup.getByLabel('登录名', { exact: true }).fill(account);
    await sensitive(setup.getByLabel('临时密码（首次登录必须改密）', { exact: true }), temporary);
    await setup.getByRole('checkbox', { name: /^Bootstrap administrator/ }).check();
    await setup.getByRole('button', { name: '保存管理员', exact: true }).click();
    await setup.getByRole('status').filter({ hasText: '已保存' }).waitFor();
    await context.clearCookies();
    await login(setup, account, temporary);
    await sensitive(setup.getByLabel('当前密码', { exact: true }), temporary);
    await sensitive(setup.getByLabel('新密码', { exact: true }), password);
    await sensitive(setup.getByLabel('确认新密码', { exact: true }), password);
    await setup.getByRole('button', { name: '保存并重新登录', exact: true }).click();
    await setup.waitForURL('**/admin/login');
    await login(setup, account, password);
    await setup.goto(base + '/admin/security');
    await sensitive(setup.getByLabel('当前密码', { exact: true }), password);
    await setup.getByRole('button', { name: '确认身份', exact: true }).click();
    await setup.getByRole('status').filter({ hasText: '密码复核成功' }).waitFor();
    console.log('H1 browser: login/change-password/reauth passed');
    const products = await createProducts(setup, suffix);
    await receiveStock(setup, products);
    console.log('H1 browser: three products and approved stock passed');
    const issued = await issueCards(setup, suffix, products);
    console.log('H1 browser: generation/memory download/seal/activation passed');
    const customer = await browser.newContext(),
      page = await customer.newPage();
    await page.clock.setFixedTime(new Date(meta.current_time));
    const redeemed = await redeem(page, issued.cards[0], true),
      order = redeemed.order;
    await capture(page, 'pending-order');
    await setup.goto(base + '/admin/orders/' + order.id);
    await setup.getByRole('heading', { name: '订单详情', exact: true }).waitFor();
    await capture(setup, 'admin-pending');
    await advance(24 * 3600, [setup, page]);
    await setup.goto(base + '/admin/carriers');
    await setup.getByRole('button', { name: '新增承运商', exact: true }).click();
    await setup.getByLabel('编码', { exact: true }).fill('H1MOCK' + suffix);
    await setup.getByLabel('名称', { exact: true }).fill('H1 MOCK 测试物流');
    await setup.getByLabel('模式', { exact: true }).selectOption('MOCK');
    const savingCarrier = setup.waitForResponse(
      (r) => r.url().endsWith('/api/v1/admin/carriers') && r.request().method() === 'POST',
    );
    await setup.getByRole('button', { name: '保存', exact: true }).click();
    assert.equal((await savingCarrier).status(), 201);
    await setup.goto(base + '/admin/orders/' + order.id + '/ship');
    const carrier = setup
      .getByLabel('承运商', { exact: true })
      .locator('option')
      .filter({ hasText: 'H1 MOCK 测试物流' });
    await carrier.waitFor({ state: 'attached' });
    await setup
      .getByLabel('承运商', { exact: true })
      .selectOption(await carrier.getAttribute('value'));
    await setup.getByLabel('运单号', { exact: true }).fill('H1-MOCK-PRIMARY');
    await setup.getByRole('button', { name: '确认全单发货', exact: true }).click();
    await setup.getByRole('status').filter({ hasText: '已确认发货' }).waitFor();
    await page.goto(base + '/orders/query');
    await page.getByLabel('卡号 / 订单号', { exact: true }).fill(order.order_no);
    await page.getByLabel('收货手机号', { exact: true }).fill('13800000000');
    await page
      .getByLabel('四位验证码', { exact: true })
      .fill(await solveChallenge(page, page.getByAltText('四位数字验证码')));
    await page.getByRole('button', { name: '查询订单', exact: true }).click();
    await page.waitForURL('**/orders/result');
    await page.getByText('状态：已发货', { exact: true }).waitFor();
    await advance(30, [setup, page]);
    await page.getByRole('link', { name: '查看物流', exact: true }).click();
    await refreshTracking(page, 'MOCK 模拟运输');
    await capture(page, 'mock-transit');
    await advance(30, [setup, page]);
    await page.reload();
    await refreshTracking(page, 'MOCK 模拟签收');
    await page.goto(base + '/orders/result');
    await page.getByText('状态：已完成', { exact: true }).waitFor();
    await capture(page, 'completed-order');
    const second = await browser.newContext(),
      secondPage = await second.newPage();
    await secondPage.clock.setFixedTime(new Date(new Date(meta.current_time).getTime() + 86460000));
    const cancelled = (await redeem(secondPage, issued.cards[1], false)).order;
    await setup.goto(base + '/admin/orders/' + cancelled.id);
    await setup.getByLabel('操作原因', { exact: true }).fill('取消分支验收');
    await setup.getByRole('button', { name: '确认取消（释放预占，不返卡）', exact: true }).click();
    await setup.getByText(/CANCELLED ·/).waitFor();
    await secondPage.reload();
    await secondPage.getByText('状态：已取消', { exact: true }).waitFor();
    await setup.goto(base + '/admin/inventory');
    await setup.getByRole('button', { name: '执行只读库存对账', exact: true }).click();
    await setup.getByRole('status').filter({ hasText: '账本与余额、活动预占一致' }).waitFor();
    await capture(setup, 'reconciliation');
    const reuse = await page.request.post(base + '/api/v1/redemptions', {
      headers: { Origin: base, 'X-CSRF-Token': redeemed.csrf, 'Idempotency-Key': randomUUID() },
      data: JSON.parse(redeemed.inputBody),
    });
    assert.equal(reuse.status(), 409, '原已消费卡被不同键再次兑换');
    const inventory = await (
      await setup.request.get(
        base +
          '/api/v1/admin/inventory?q=' +
          encodeURIComponent(('H1-' + suffix).toUpperCase()) +
          '&limit=100',
      )
    ).json();
    for (const product of products) {
      const balance = inventory.items.find((item) => item.product_id === product.id);
      assert(
        balance && balance.reserved === 0 && balance.on_hand === (product.letter === 'C' ? 10 : 9),
        '履约或取消库存不符',
      );
    }
    const orders = await (
      await setup.request.get(base + '/api/v1/admin/orders?q=' + encodeURIComponent(order.order_no))
    ).json();
    assert.equal(orders.items.length, 1, '弱网恢复产生重复订单');
    const vouchers = await (
      await setup.request.get(base + '/api/v1/admin/vouchers?batch_id=' + issued.batchID)
    ).json();
    assert(
      vouchers.items.length === 2 && vouchers.items.every((card) => card.redeemed),
      '取消错误恢复了卡券权益',
    );
    issued.cards.forEach((row) => row.fill(''));
    await context.close();
    await customer.close();
    await second.close();
    console.log(
      'H1 browser passed: real login/password change, products/stock/issue, memory-only credential download, weak-network same-key redeem, ship/query/transit/delivery/cancel and reconciliation.',
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error('H1 浏览器主链失败：' + error.message);
  process.exitCode = 1;
});
