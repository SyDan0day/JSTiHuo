import { after, test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createServer } from 'vite';
import { parse, compileScript } from '@vue/compiler-sfc';
import ts from 'typescript';
import { createRenderer, nextTick } from 'vue';
import { createPinia } from 'pinia';
import { createRouter, createMemoryHistory } from 'vue-router';
const server = await createServer({ configFile: false,
  plugins: [{ name: 'unit-vue',
    /** 用途：将 Vue 页面编译为可运行的内存测试组件。
     * 参数：id 为组件文件路径。
     * 返回值：编译结果，非组件返回空。
     */
    async load(id) {
    if (!id.endsWith('.vue')) return;
    const { descriptor } = parse(await readFile(id, 'utf8'), { filename: id });
    return ts.transpileModule(compileScript(descriptor, { id, inlineTemplate: true }).content,
      { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText;
  } }], server: { middlewareMode: true, hmr: false, ws: false, watch: null } });
after(() => server.close());
globalThis.Document = class {};
globalThis.ShadowRoot = class {};
after(() => { delete globalThis.Document; delete globalThis.ShadowRoot; });
/**
 * settle 用途：等待组件异步请求和渲染完成。
 * 参数：无。
 * 返回值：当前任务队列完成的 Promise。
 */
async function settle() {
  for (let i = 0; i < 8; i++) { await new Promise(setImmediate); await nextTick(); }
}
/**
 * mount 用途：在内存中运行真实页面及事件处理。
 * 参数：file 为页面模块；path 为测试路由；permissions 为管理权限。
 * 返回值：控件查找和页面卸载方法。
 */
async function mount(file, path, permissions = []) {
  /** 用途：创建渲染器节点。
   * 参数：type 为节点种类；text 为文本。
   * 返回值：带必要表单属性的内存节点。
   */
  const node = (type, text = '') => ({
    type, tagName: type.toUpperCase(), text, props: {}, children: [], parent: null,
    addEventListener: () => {}, removeEventListener: () => {}, getRootNode: () => ({}),
    /** 用途：提供选择框选项。
     * 参数：无。
     * 返回值：当前 option 节点列表。
     */
    get options() { return this.children.filter((el) => el.type === 'option'); },
  });
  const renderer = createRenderer({ createElement: node,
    createText: (text) => node('text', text), createComment: (text) => node('comment', text),
    setText: (el, text) => { el.text = text; },
    setElementText: (el, text) => { el.text = text; el.children = []; },
    patchProp: (el, key, _, value) => { el.props[key] = value; if (key === 'value') el.value = value; },
    parentNode: (el) => el.parent, nextSibling: () => null,
    /** 用途：按渲染顺序插入或移动节点。
     * 参数：el 为节点；parent 为父节点；anchor 为插入锚点。
     * 返回值：无。
     */
    insert(el, parent, anchor) {
      if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
      el.parent = parent;
      const index = anchor ? parent.children.indexOf(anchor) : -1;
      parent.children.splice(index < 0 ? parent.children.length : index, 0, el);
    },
    /** 用途：移除卸载节点。
     * 参数：el 为待移除节点。
     * 返回值：无。
     */
    remove(el) { if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1); } });
  const { default: Page } = await server.ssrLoadModule(file);
  const pinia = createPinia();
  const { useSession } = await server.ssrLoadModule('/src/stores/session.ts');
  useSession(pinia).session = { permissions, csrf_token: 'unit' };
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: path.replace('/1/', '/:id/'), component: Page }, { path: '/:pathMatch(.*)*', component: { render: () => null } }] });
  await router.push(path);
  const app = renderer.createApp(Page).use(pinia).use(router), root = node('root');
  app.mount(root);
  await settle();
  /** 用途：遍历内存控件树。
   * 参数：el 为根节点。
   * 返回值：按顺序排列的全部节点。
   */
  const flatten = (el) => [el, ...el.children.flatMap(flatten)];
  /** 用途：提取控件的可见文案。
   * 参数：el 为控件节点。
   * 返回值：控件及子节点文本。
   */
  const text = (el) => el.text + el.children.map(text).join('');
  return { router, find: (predicate) => flatten(root).find(predicate),
    button: (label) => flatten(root).find((el) => el.type === 'button' && text(el).trim() === label),
    unmount: () => app.unmount() };
}
const order = { id: '1', order_no: 'O_TEST', version: 1, status: 'PENDING',
  ship_date: '2026-10-20', customer_note: '', recipient: { name: '测*', phone: '138****0000' } };
test('发货版本冲突后可刷新版本重试，承运商不重复', async (t) => {
  let version = 1;
  const commands = [];
  t.mock.method(globalThis, 'fetch', async (path, options) => {
    if (path.includes('/carriers?')) return Response.json({ items: [{ id: '1', name: '测试', provider: 'MOCK' }] });
    if (path.endsWith('/shipments')) {
      commands.push(JSON.parse(options.body));
      return commands.length === 1 ? Response.json({ code: 'VERSION_CONFLICT', message: 'changed' }, { status: 409 }) : Response.json({ id: '1' });
    }
    return Response.json({ ...order, version });
  });
  const page = await mount('/src/features/admin/shipping/Ship.vue', '/admin/orders/1/ship', ['carrier.read']);
  t.after(page.unmount);
  page.find((el) => el.props.id === 'ship-carrier').props['onUpdate:modelValue']('1');
  page.find((el) => el.props.id === 'ship-tracking').props['onUpdate:modelValue']('MOCK-UNIT');
  await page.find((el) => el.type === 'form').props.onSubmit({ preventDefault() {} });
  version = 2;
  const refresh = page.button('刷新订单与承运商');
  assert.ok(refresh, '版本冲突后缺少恢复入口');
  await refresh.props.onClick(); await settle();
  assert.equal(page.find((el) => el.props.id === 'ship-carrier').options.length, 2);
  await page.find((el) => el.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.deepEqual(commands.map((command) => command.version), [1, 2]);
});
test('修改页日历失败时禁止提交，刷新恢复后开放', async (t) => {
  let unavailable = true;
  t.mock.method(globalThis, 'fetch', async (path) => {
    if (path.includes('/regions')) return Response.json({ items: [] });
    if (path.endsWith('/calendar')) {
      if (unavailable) throw new TypeError('offline');
      return Response.json({ items: [{ date: order.ship_date, available: true }] });
    }
    return Response.json(order, { headers: { 'X-Order-Scope': 'WRITE' } });
  });
  const page = await mount('/src/features/orders/Edit.vue', '/orders/edit');
  t.after(page.unmount);
  assert.equal(page.button('确认替换收货信息').props.disabled, true);
  unavailable = false;
  await page.button('刷新权威版本').props.onClick(); await settle();
  assert.equal(page.button('确认替换收货信息').props.disabled, false);
});


test('查询表单使用专用挑战，成功后清空手机号并进入结果页', async (t) => {
  let submitted;
  t.mock.method(globalThis, 'fetch', async (path, options) => {
    if (path.endsWith('/bootstrap')) return Response.json({ csrf_token: 'preauth-unit' });
    if (path.endsWith('/challenges')) {
      assert.deepEqual(JSON.parse(options.body), { purpose: 'ORDER_QUERY' });
      return Response.json({ id: 'challenge-unit', image_data_url: 'data:image/png;base64,unit' });
    }
    assert.equal(path, '/api/v1/order-sessions');
    submitted = JSON.parse(options.body);
    assert.equal(options.headers.get('X-CSRF-Token'), 'preauth-unit');
    return Response.json({ csrf_token: 'order-unit' }, { headers: { 'X-Order-Scope': 'READ' } });
  });
  const page = await mount('/src/features/orders/Query.vue', '/orders/query');
  t.after(page.unmount);
  for (const [id, value] of [['order-identifier', 'O_TEST'], ['query-phone', '13900000000'], ['query-challenge', '0000']])
    page.find((el) => el.props.id === id).props['onUpdate:modelValue'](value);
  await page.find((el) => el.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.deepEqual(submitted, { identifier: 'O_TEST', phone: '13900000000',
    challenge_id: 'challenge-unit', challenge_answer: '0000' });
  assert.equal(page.find((el) => el.props.id === 'query-phone').value, '');
  assert.equal(page.router.currentRoute.value.path, '/orders/result');
});

test('发货网络结果未知时冻结刷新，重试沿用原正文和幂等键', async (t) => {
  const commands = [];
  t.mock.method(globalThis, 'fetch', async (path, options) => {
    if (path.includes('/carriers?')) return Response.json({ items: [{ id: '1', name: '测试', provider: 'MOCK' }] });
    if (path.endsWith('/shipments')) {
      commands.push({ body: options.body, key: options.headers.get('Idempotency-Key') });
      if (commands.length === 1) throw new TypeError('response lost');
      return Response.json({ id: '1' });
    }
    return Response.json(order);
  });
  const page = await mount('/src/features/admin/shipping/Ship.vue', '/admin/orders/1/ship', ['carrier.read']);
  t.after(page.unmount);
  page.find((el) => el.props.id === 'ship-carrier').props['onUpdate:modelValue']('1');
  page.find((el) => el.props.id === 'ship-tracking').props['onUpdate:modelValue']('MOCK-RETRY');
  await page.find((el) => el.type === 'form').props.onSubmit({ preventDefault() {} });
  await settle();
  assert.equal(page.button('刷新订单与承运商').props.disabled, true);
  await page.button('重试原请求').props.onClick();
  assert.equal(commands.length, 2);
  assert.deepEqual(commands[0], commands[1]);
});
