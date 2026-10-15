<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { request, RequestError } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
import { ElTable, ElTableColumn, ElButton, ElInput, ElInputNumber, ElTag } from 'element-plus';
import 'element-plus/dist/index.css';
type Balance = components['schemas']['Inventory'];
const items = ref<Balance[]>([]),
  ledger = ref<components['schemas']['Ledger'][]>([]),
  next = ref(''),
  ledgerNext = ref(''),
  q = ref(''),
  low = ref(false),
  busy = ref(false),
  error = ref(''),
  selected = ref<Balance>(),
  warning = ref(0),
  ledgerProduct = ref(''),
  store = useSession();
const reconciliation = ref<components['schemas']['Reconcile']>();
const canWrite = computed(() => store.session?.permissions.includes('inventory.write'));
const pending = ref<{ path: string; body: string; key: string }>();
/**
 * message 用途：将库存接口失败映射为可恢复提示。
 * 参数：e 为业务或网络错误。
 * 返回值：用户可读的错误说明。
 */
function message(e: unknown) {
  return e instanceof RequestError
    ? ({
        VERSION_CONFLICT: '库存已变化，请刷新后重新设置',
        IDEMPOTENCY_CONFLICT: '请求标识冲突，请刷新后重试',
        FORBIDDEN: '没有库存操作权限',
      }[e.detail.code] ?? '操作失败，请重试')
    : '网络异常，可重试原请求';
}
/**
 * load 用途：读取筛选后的库存余额页。
 * 参数：after 为排他游标，空值读取首页。
 * 返回值：加载 Promise，失败显示错误。
 */
async function load(after = '') {
  busy.value = true;
  error.value = '';
  try {
    const p = new URLSearchParams({ limit: '20', q: q.value, status: low.value ? 'LOW' : '' });
    if (after) p.set('after_id', after);
    const result = await request<components['schemas']['InventoryList']>(
      '/api/v1/admin/inventory?' + p,
    );
    items.value = result.items;
    next.value = result.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
/**
 * edit 用途：复制所选商品的警戒值。
 * 参数：b 为当前余额及版本。
 * 返回值：无，未确认保存期间禁止切换。
 */
function edit(b: Balance) {
  if (pending.value) return;
  selected.value = b;
  warning.value = b.warning_level;
}
/**
 * save 用途：按版本保存警戒值，网络不确定时同键重试。
 * 参数：无，使用选中余额和警戒值。
 * 返回值：保存 Promise，不改变库存数量。
 */
async function save() {
  if (!selected.value) return;
  busy.value = true;
  error.value = '';
  if (!pending.value)
    pending.value = {
      path: '/api/v1/admin/inventory/' + selected.value.product_id + '/warning',
      body: JSON.stringify({ version: selected.value.version, warning_level: warning.value }),
      key: crypto.randomUUID(),
    };
  try {
    await request(
      pending.value.path,
      { method: 'PATCH', body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
    selected.value = undefined;
    await load();
  } catch (e) {
    error.value = message(e);
    if (e instanceof RequestError && e.status < 500) {
      pending.value = undefined;
      if (e.status === 409) selected.value = undefined;
    }
  } finally {
    busy.value = false;
  }
}
/**
 * loadLedger 用途：读取全部或指定商品流水。
 * 参数：product 为商品 ID，空值查全部；after 为排他游标。
 * 返回值：加载 Promise，失败显示恢复提示。
 */
async function loadLedger(product = ledgerProduct.value, after = '') {
  busy.value = true;
  error.value = '';
  ledgerProduct.value = product;
  try {
    const p = new URLSearchParams({ limit: '20' });
    if (product) p.set('product_id', product);
    if (after) p.set('after_id', after);
    const result = await request<components['schemas']['LedgerList']>(
      '/api/v1/admin/inventory/transactions?' + p,
    );
    ledger.value = result.items;
    ledgerNext.value = result.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
/**
 * reconcile 用途：读取全局库存对账结果，不执行自动补账。
 * 参数：无。
 * 返回值：检查 Promise，失败或漂移明确显示。
 */
async function reconcile() {
  if (busy.value || pending.value) return;
  busy.value = true;
  error.value = '';
  try {
    reconciliation.value = await request('/api/v1/admin/inventory/reconciliation');
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <div class="page-title">
      <h1>库存余额</h1>
      <RouterLink to="/admin">工作台</RouterLink
      ><RouterLink to="/admin/inventory/documents">入出库单</RouterLink>
    </div>
    <form class="filters" @submit.prevent="load()">
      <ElInput
        v-model="q"
        maxlength="100"
        placeholder="商品名称或编码"
        aria-label="搜索商品"
      /><label><input v-model="low" type="checkbox" />仅低库存</label
      ><ElButton native-type="submit" :loading="busy">查询</ElButton>
    </form>
    <p v-if="error" role="alert">{{ error }}</p>
    <section aria-label="库存对账">
      <ElButton :disabled="busy || !!pending" @click="reconcile">执行只读库存对账</ElButton>
      <p v-if="reconciliation" role="status">
        {{
          reconciliation.balanced
            ? '账本与余额、活动预占一致'
            : '发现库存漂移，请停止相关操作并人工核查'
        }}
        · {{ reconciliation.checked_at }}
      </p>
      <p v-if="reconciliation && !reconciliation.balanced">
        漂移商品（最多一百项）：{{ reconciliation.product_ids_with_drift.join('、') }}
      </p>
    </section>
    <ElTable :data="items" empty-text="暂无库存记录"
      ><ElTableColumn prop="product_id" label="商品 ID" /><ElTableColumn
        prop="on_hand"
        label="实际库存"
      /><ElTableColumn prop="reserved" label="已预占" /><ElTableColumn
        prop="available"
        label="可用库存"
      /><ElTableColumn label="警戒值"
        ><template #default="{ row }"
          >{{ row.warning_level }}
          <ElTag v-if="row.available <= row.warning_level" type="warning">低库存</ElTag></template
        ></ElTableColumn
      ><ElTableColumn label="操作"
        ><template #default="{ row }"
          ><ElButton :disabled="busy" @click="loadLedger(row.product_id)">流水</ElButton
          ><ElButton v-if="canWrite" :disabled="busy || !!pending" @click="edit(row)"
            >设置警戒值</ElButton
          ></template
        ></ElTableColumn
      ></ElTable
    ><ElButton v-if="next" :disabled="busy" @click="load(next)">下一页</ElButton
    ><ElButton :disabled="busy" @click="load()">刷新 / 首页</ElButton>
    <form v-if="selected" class="filters" @submit.prevent="save">
      <label
        >商品 {{ selected.product_id }} 警戒值
        <ElInputNumber
          v-model="warning"
          :min="0"
          :max="1000000"
          :precision="0"
          :disabled="!!pending" /></label
      ><ElButton native-type="submit" :loading="busy">{{
        pending ? '重试原请求' : '保存'
      }}</ElButton
      ><ElButton v-if="!pending" @click="selected = undefined">取消</ElButton>
    </form>
    <h2>库存流水 {{ ledgerProduct ? '· 商品 ' + ledgerProduct : '' }}</h2>
    <ElButton :disabled="busy" @click="loadLedger('')">全部流水</ElButton
    ><ElTable :data="ledger" empty-text="暂无流水"
      ><ElTableColumn prop="product_id" label="商品 ID" /><ElTableColumn
        prop="on_hand_delta"
        label="实际库存变动" /><ElTableColumn
        prop="reserved_delta"
        label="预占变动" /><ElTableColumn prop="event_key" label="事件" /><ElTableColumn
        prop="reason"
        label="原因" /><ElTableColumn prop="created_at" label="时间" /></ElTable
    ><ElButton v-if="ledgerNext" :disabled="busy" @click="loadLedger(ledgerProduct, ledgerNext)"
      >下一页流水</ElButton
    >
  </main>
</template>
