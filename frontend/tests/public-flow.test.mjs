import { after, test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({
  server: { middlewareMode: true, hmr: false, ws: false, watch: null },
});
after(() => server.close());
const { useChallenge } = await server.ssrLoadModule('/src/composables/useChallenge.ts');

/**
 * mockChallenge 用途：模拟真实请求层的预认证与图形挑战响应。
 * 参数：t 为测试上下文；purpose 为预期的挑战用途。
 * 返回值：无；请求契约不匹配时测试失败。
 */
function mockChallenge(t, purpose) {
  t.mock.method(globalThis, 'fetch', async (path, options) => {
    if (path === '/api/v1/session/bootstrap')
      return Response.json({ csrf_token: 'test-csrf' });
    assert.equal(path, '/api/v1/challenges');
    assert.equal(options.headers.get('X-CSRF-Token'), 'test-csrf');
    assert.deepEqual(JSON.parse(options.body), { purpose });
    return Response.json({ id: 'new-challenge', image_data_url: 'data:image/png;base64,test' });
  });
}

for (const purpose of ['VOUCHER', 'ORDER_QUERY']) {
  test(`${purpose} 刷新失败移除旧挑战，恢复后清除错误`, async (t) => {
    const state = useChallenge(purpose);
    state.challenge.value = { id: 'old-challenge' };
    state.answer.value = '1234';
    t.mock.method(globalThis, 'fetch', async () => { throw new TypeError('offline'); });
    const refresh = state.refresh();
    assert.equal(state.loading.value, true);
    assert.equal(state.challenge.value, undefined);
    assert.equal(state.answer.value, '');
    await refresh;
    assert.equal(state.loading.value, false);
    assert.match(state.error.value, /验证码加载失败/);
    t.mock.restoreAll();
    mockChallenge(t, purpose);
    await state.refresh();
    assert.equal(state.challenge.value.id, 'new-challenge');
    assert.equal(state.error.value, '');
  });
}

test('刷新中的重复点击只发起一次预认证', async (t) => {
  const state = useChallenge('VOUCHER');
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const fetch = t.mock.method(globalThis, 'fetch', async () => {
    await gate;
    throw new TypeError('offline');
  });
  const first = state.refresh();
  await state.refresh();
  assert.equal(fetch.mock.callCount(), 1);
  release();
  await first;
  assert.equal(state.loading.value, false);
});

test('业务失败后的自动换图保留业务错误', async (t) => {
  mockChallenge(t, 'VOUCHER');
  const state = useChallenge('VOUCHER');
  state.error.value = '卡号或密码不正确';
  await state.refresh(true);
  assert.equal(state.error.value, '卡号或密码不正确');
  assert.equal(state.challenge.value.id, 'new-challenge');
});

test('确认地址包含完整地区，切换父级或缺少选项时不沿用旧名称', async () => {
  const { addressRegionLabel } = await server.ssrLoadModule('/src/components/address-region.ts');
  const address = { province_code: '110000', city_code: '110100', district_code: '110101' };
  const levels = [
    [{ code: '110000', name: '北京市', parent_code: '' }],
    [{ code: '110100', name: '市辖区', parent_code: '110000' }],
    [{ code: '110101', name: '东城区', parent_code: '110100' }],
  ];
  assert.equal(addressRegionLabel(address, levels), '北京市 市辖区 东城区');
  assert.equal(addressRegionLabel(address, [levels[0], [], levels[2]]), '');
  assert.equal(addressRegionLabel({ ...address, province_code: '120000' }, levels), '');
  const mismatched = [[{ code: '120000', name: '天津市' }], levels[1], levels[2]];
  assert.equal(addressRegionLabel({ ...address, province_code: '120000' }, mismatched), '');
});

test('清理兑换会话同时清理地区名称和收货信息', async () => {
  const { createPinia } = await import('pinia');
  const { useRedemption } = await server.ssrLoadModule('/src/stores/redemption.ts');
  const store = useRedemption(createPinia());
  store.regionLabel = '北京市 市辖区 东城区';
  store.address.detail = '合成测试地址一号';
  store.clear();
  assert.equal(store.regionLabel, '');
  assert.equal(store.address.detail, '');
});

test('确认页渲染完整地址，地区名称不混入 API 地址对象', async () => {
  const { createSSRApp } = await import('vue');
  const { renderToString } = await import('vue/server-renderer');
  const { createPinia } = await import('pinia');
  const { createRouter, createMemoryHistory } = await import('vue-router');
  const { default: Confirm } = await server.ssrLoadModule('/src/features/redemption/Confirm.vue');
  const { useRedemption } = await server.ssrLoadModule('/src/stores/redemption.ts');
  const pinia = createPinia();
  const store = useRedemption(pinia);
  store.regionLabel = '北京市 市辖区 东城区';
  store.address.detail = '合成测试地址一号';
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/redeem/confirm', component: Confirm },
    { path: '/redeem/address', component: { template: '<div />' } },
  ] });
  await router.push('/redeem/confirm');
  const html = await renderToString(createSSRApp(Confirm).use(pinia).use(router));
  assert.match(html, /北京市 市辖区 东城区 合成测试地址一号/);
  assert.equal('regionLabel' in store.address, false);
});
