<script setup lang="ts">
import { flowError } from '../../../api/flow-error';
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
import { useSession } from '../../../stores/session';
const session = useSession();
const route = useRoute(),
  path = '/api/v1/admin/orders/' + route.params.id;
const order = ref<components['schemas']['Order']>();
const carriers = ref<components['schemas']['Carrier'][]>([]);
const carrierId = ref(''),
  tracking = ref(''),
  error = ref(''),
  busy = ref(false),
  success = ref(false),
  ready = ref(false);
const pending = ref<{ body: string; key: string }>();
/**
 * load 用途：刷新权威订单版本与承运商，加载失败时禁止发货。
 * 参数：无，使用路由订单标识。
 * 返回值：完成操作的 Promise，失败显示错误。
 */
async function load() {
  if (busy.value || pending.value) return;
  busy.value = true;
  ready.value = false;
  error.value = '';
  carriers.value = [];
  try {
    order.value = await request(path);
    if (typeof route.query.label_job === 'string') {
      const label = await request<components['schemas']['Job']>(
        '/api/v1/admin/jobs/' + route.query.label_job,
      );
      if (label.kind !== 'LABEL' || label.state !== 'READY' || label.order_id !== order.value?.id)
        throw new Error('invalid label');
      carrierId.value = label.carrier_id ?? '';
      tracking.value = label.tracking_no ?? '';
    }
    if (!session.session?.permissions.includes('carrier.read')) {
      ready.value = true;
      return;
    }
    let after = '';
    do {
      const page = await request<components['schemas']['CarrierList']>(
        '/api/v1/admin/carriers?status=ENABLED&limit=100' + (after ? '&after_id=' + after : ''),
      );
      carriers.value.push(...page.items);
      after = page.next_after_id ?? '';
    } while (after);
    ready.value = true;
  } catch (e) {
    error.value = flowError(e);
  } finally {
    busy.value = false;
  }
}
/**
 * submit 用途：以固定正文和幂等键确认全单人工发货。
 * 参数：无，使用订单版本、承运商与单号。
 * 返回值：完成操作的 Promise，结果未知时保留原命令供重试。
 */
async function submit() {
  if (!order.value || busy.value || !ready.value || success.value) return;
  pending.value ??= {
    key: crypto.randomUUID(),
    body: JSON.stringify({
      version: order.value.version,
      carrier_id: carrierId.value,
      tracking_no: tracking.value,
    }),
  };
  busy.value = true;
  error.value = '';
  try {
    await request(
      path + '/shipments',
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
    success.value = true;
  } catch (e) {
    error.value =
      e instanceof RequestError && e.detail.code === 'SHIP_DATE_NOT_REACHED'
        ? '尚未到预约发货日，请到期后刷新订单再发货'
        : flowError(e);
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <main class="admin-page">
    <RouterLink :to="'/admin/orders/' + route.params.id">返回订单</RouterLink>
    <h1>全单人工发货</h1>
    <p role="alert">{{ error }}</p>
    <p v-if="success" role="status">已确认发货，库存预占已转为实际出库。</p>
    <button v-if="!success" :disabled="busy || !!pending" @click="load">
      刷新订单与承运商
    </button>
    <form v-if="!success && order" @submit.prevent="submit">
      <p>{{ order.order_no }} · {{ order.status }} · 预约 {{ order.ship_date }}</p>
      <fieldset :disabled="busy || !ready || !!pending || order.status !== 'PENDING'">
        <label for="ship-carrier">承运商</label>
        <select
          v-if="session.session?.permissions.includes('carrier.read')"
          id="ship-carrier"
          v-model="carrierId"
          required
        >
          <option value="">请选择</option>
          <option v-for="carrier in carriers" :key="carrier.id" :value="carrier.id">
            {{ carrier.name }}（{{ carrier.provider }}）
          </option>
        </select>
        <input
          v-else
          id="ship-carrier"
          v-model="carrierId"
          required
          pattern="[1-9][0-9]*"
          placeholder="承运商 ID"
        />
        <p>MOCK 仅模拟物流，不会实际寄送。MANUAL 显示实际接收的轨迹。</p>
        <label for="ship-tracking">运单号</label
        ><input
          id="ship-tracking"
          v-model="tracking"
          minlength="3"
          maxlength="80"
          pattern="[A-Za-z0-9-]+"
          required
        />
        <button>确认全单发货</button>
      </fieldset>
      <button v-if="pending" type="button" :disabled="busy" @click="submit">重试原请求</button>
    </form>
  </main>
</template>
