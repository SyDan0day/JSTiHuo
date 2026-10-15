<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { getOrderScope, request } from '../../api/client';
import { useCommand } from '../../api/command';
import { useSession } from '../../stores/session';
import OrderStepUp from '../../components/OrderStepUp.vue';
import type { components } from '../../api/generated/schema';
type Case = components['schemas']['AfterSale'];
const route = useRoute(),
  session = useSession();
const admin = route.path.startsWith('/admin/');
const { busy, error, pending, send } = useCommand();
const loading = ref(false),
  loadError = ref(''),
  writable = ref(false);
const cases = ref<Case[]>([]),
  next = ref(''),
  order = ref<components['schemas']['Order']>();
const selected = ref<Case>(),
  kind = ref<'COMPLAINT' | 'REPLACEMENT'>('COMPLAINT');
const reason = ref(''),
  items = ref<string[]>([]),
  resolution = ref(''),
  carrier = ref(''),
  tracking = ref('');
const status = ref(''),
  q = ref(typeof route.query.order_id === 'string' ? route.query.order_id : '');
const permissions = computed(() => session.session?.permissions ?? []);
const canApply = computed(
  () =>
    order.value &&
    !order.value.deleted &&
    ['SHIPPED', 'COMPLETED'].includes(order.value.status) &&
    order.value.fulfillment_mode === 'DELIVERY',
);
/**
 * load 用途：读取当前本单或管理员授权的案件页。
 * 参数：after 为上一页排他游标，空值重新加载。
 * 返回值：加载 Promise，读取失败保留原列表并给出恢复入口。
 */
async function load(after = '') {
  if (busy.value || pending.value || loading.value) return;
  loading.value = true;
  loadError.value = '';
  try {
    if (!admin) {
      order.value = await request('/api/v1/order');
      writable.value = getOrderScope() === 'WRITE';
    }
    const query = new URLSearchParams({ limit: '20', status: status.value });
    if (after) query.set('after_id', after);
    if (admin && q.value) query.set('q', q.value);
    const page = await request<components['schemas']['AfterSaleList']>(
      (admin ? '/api/v1/admin/after-sales' : '/api/v1/order/after-sales') + '?' + query,
    );
    cases.value = after ? [...cases.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
    if (selected.value) selected.value = cases.value.find((item) => item.id === selected.value?.id);
  } catch {
    loadError.value = '售后读取失败或没有权限，请重试。';
  } finally {
    loading.value = false;
  }
}
/**
 * choose 用途：读取管理员案件权威详情或展开顾客本单案件。
 * 参数：item 为当前列表案件。
 * 返回值：加载 Promise，切换前清空处置输入。
 */
async function choose(item: Case) {
  if (busy.value || pending.value) return;
  resolution.value = '';
  carrier.value = '';
  tracking.value = '';
  try {
    selected.value = admin ? await request('/api/v1/admin/after-sales/' + item.id) : item;
  } catch {
    loadError.value = '案件详情读取失败。';
  }
}
/**
 * apply 用途：创建当前已发配送订单的投诉或补发申请。
 * 参数：无，使用用户选择的冻结订单项与原因。
 * 返回值：提交 Promise，结果不确定时只允许同键重试。
 */
async function apply() {
  const result = await send<Case>('/api/v1/order/after-sales', {
    kind: kind.value,
    reason: reason.value,
    items:
      kind.value === 'COMPLAINT'
        ? []
        : items.value.map((id) => ({ order_item_id: id, quantity: 1 })),
  });
  if (result) {
    reason.value = '';
    items.value = [];
    selected.value = result;
    await load();
  }
}
/**
 * decide 用途：提交明确的案件处理结论。
 * 参数：decision 为批准、拒绝或关闭投诉。
 * 返回值：提交 Promise，失败显示状态或复核提示。
 */
async function decide(decision: 'APPROVE' | 'REJECT' | 'CLOSE') {
  if (!selected.value) return;
  const result = await send<Case>('/api/v1/admin/after-sales/' + selected.value.id + '/decision', {
    version: selected.value.version,
    decision,
    resolution: resolution.value,
  });
  if (result) {
    selected.value = result;
    await load();
  }
}
/**
 * ship 用途：确认已批准案件的补发出库。
 * 参数：无，使用案件版本、承运商和实际单号。
 * 返回值：提交 Promise，成功刷新案件，不改变原单状态。
 */
async function ship() {
  if (!selected.value) return;
  const result = await send('/api/v1/admin/after-sales/' + selected.value.id + '/shipments', {
    version: selected.value.version,
    carrier_id: carrier.value,
    tracking_no: tracking.value,
  });
  if (result) {
    carrier.value = '';
    tracking.value = '';
    await load();
  }
}
/**
 * retry 用途：重放尚未确认的原命令，不采用当前已变化的输入。
 * 参数：无。
 * 返回值：恢复 Promise，成功后重新读取案件状态。
 */
async function retry() {
  if (!pending.value) return;
  if (await send(pending.value.path, {})) await load();
}
onMounted(() => load());
</script>
<template>
  <component :is="admin ? 'main' : 'section'" class="flow-page admin-page">
    <RouterLink :to="admin ? '/admin' : '/orders/result'"
      >返回{{ admin ? '工作台' : '订单' }}</RouterLink
    >
    <h1>售后{{ admin ? '案件管理' : '申请与进度' }}</h1>
    <p>售后独立于原订单；补发签收后关闭案件，原卡券不可再次兑换。</p>
    <p v-if="error || loadError" role="alert">{{ error || loadError }}</p>
    <RouterLink v-if="admin" to="/admin/security">身份复核</RouterLink>
    <button :disabled="busy || loading || !!pending" @click="load()">刷新案件</button>
    <template v-if="!admin">
      <OrderStepUp v-if="order && !writable" @verified="writable = true" />
      <form
        v-if="
          writable && canApply && !cases.some((item) => ['OPEN', 'APPROVED'].includes(item.state))
        "
        @submit.prevent="apply"
      >
        <fieldset :disabled="busy || !!pending">
          <legend>申请售后</legend>
          <label for="case-kind">申请类型</label
          ><select id="case-kind" v-model="kind">
            <option value="COMPLAINT">投诉咨询</option>
            <option value="REPLACEMENT">按原单补发</option>
          </select>
          <fieldset v-if="kind === 'REPLACEMENT'">
            <legend>选择需要补发的原单商品</legend>
            <label v-for="item in order?.items" :key="item.id"
              ><input v-model="items" type="checkbox" :value="item.id" />{{ item.name }} × 1</label
            >
          </fieldset>
          <label for="case-reason">申请原因（请勿填写完整地址或卡密）</label
          ><textarea id="case-reason" v-model="reason" required maxlength="500" />
          <button :disabled="kind === 'REPLACEMENT' && !items.length">提交售后申请</button>
        </fieldset>
      </form>
      <p v-if="order && !canApply">当前订单不能申请配送售后，请通过反馈联系工作人员。</p>
    </template>
    <form v-if="admin" @submit.prevent="load()">
      <fieldset :disabled="busy || loading || !!pending">
        <label for="case-order">订单 ID</label
        ><input id="case-order" v-model="q" maxlength="100" /><label for="case-state">状态</label
        ><select id="case-state" v-model="status">
          <option value="">全部</option>
          <option v-for="state in ['OPEN', 'APPROVED', 'REJECTED', 'CLOSED']" :key="state">
            {{ state }}
          </option></select
        ><button>筛选</button>
      </fieldset>
    </form>
    <p v-if="!cases.length && !loading">暂无售后案件。</p>
    <ul>
      <li v-for="item in cases" :key="item.id">
        <button :disabled="busy || !!pending" @click="choose(item)">
          案件 {{ item.id }} · {{ item.kind }} · {{ item.state }}
        </button>
      </li>
    </ul>
    <button v-if="next" :disabled="busy || loading || !!pending" @click="load(next)">
      加载更多案件
    </button>
    <section v-if="selected">
      <h2>案件 {{ selected.id }} · {{ selected.state }}</h2>
      <p>订单 {{ selected.order_id }} · 版本 {{ selected.version }}</p>
      <p>{{ selected.reason }}</p>
      <p>处理说明：{{ selected.resolution || '尚未处理' }}</p>
      <ul>
        <li v-for="item in selected.items" :key="item.order_item_id">
          订单项 {{ item.order_item_id }} × {{ item.quantity }}
        </li>
      </ul>
      <RouterLink
        v-if="admin && permissions.includes('order.read')"
        :to="'/admin/orders/' + selected.order_id"
        >查看原订单和运单</RouterLink
      >
      <form
        v-if="admin && selected.state === 'OPEN' && permissions.includes('after_sale.write')"
        @submit.prevent="decide(selected.kind === 'REPLACEMENT' ? 'APPROVE' : 'CLOSE')"
      >
        <fieldset :disabled="busy || !!pending">
          <label for="case-resolution">处理说明</label
          ><textarea id="case-resolution" v-model="resolution" required maxlength="1000" /><button>
            {{ selected.kind === 'REPLACEMENT' ? '批准补发' : '关闭投诉' }}</button
          ><button type="button" :disabled="!resolution.trim()" @click="decide('REJECT')">
            驳回申请
          </button>
        </fieldset>
      </form>
      <form
        v-if="
          admin &&
          selected.kind === 'REPLACEMENT' &&
          selected.state === 'APPROVED' &&
          permissions.includes('order.ship')
        "
        @submit.prevent="ship"
      >
        <fieldset :disabled="busy || !!pending">
          <legend>补发出库（每案件仅一次）</legend>
          <label for="replacement-carrier">启用的承运商 ID</label
          ><input
            id="replacement-carrier"
            v-model="carrier"
            required
            pattern="[1-9][0-9]*"
          /><RouterLink v-if="permissions.includes('carrier.read')" to="/admin/carriers"
            >查看承运商目录</RouterLink
          ><label for="replacement-tracking">实际补发单号</label
          ><input
            id="replacement-tracking"
            v-model="tracking"
            required
            minlength="3"
            maxlength="80"
            pattern="[A-Za-z0-9-]+"
          /><button>确认补发并扣实存</button>
        </fieldset>
      </form>
    </section>
    <button v-if="pending" :disabled="busy" @click="retry">重试原售后请求</button>
  </component>
</template>
<style scoped>
main,
section {
  overflow-wrap: anywhere;
}
input,
select,
textarea {
  max-width: 100%;
  min-width: 0;
}
input[type='checkbox'] {
  width: auto;
}
</style>
