<script setup lang="ts">
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
  success = ref(false);
const pending = ref<{ body: string; key: string }>();
/**
 * load 用途：加载待发订单，有目录读权限时加载启用承运商。
 * 参数：无，使用路由订单标识。
 * 返回值：完成操作的 Promise，失败显示错误。
 */
async function load() {
  busy.value = true;
  try {
    order.value = await request(path);
    if (!session.session?.permissions.includes('carrier.read')) return;
    let after = '';
    do {
      const page = await request<components['schemas']['CarrierList']>(
        '/api/v1/admin/carriers?status=ENABLED&limit=100' +
          (after ? '&after_id=' + after : ''),
      );
      carriers.value.push(...page.items);
      after = page.next_after_id ?? '';
    } while (after);
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败';
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
  if (!order.value) return;
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
      e instanceof RequestError ? e.detail.code : '网络结果未确认，请重试原请求';
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
    <form v-else-if="order" @submit.prevent="submit">
      <p>{{ order.order_no }} · {{ order.status }} · 预约 {{ order.ship_date }}</p>
      <fieldset :disabled="busy || !!pending || order.status !== 'PENDING'">
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
      <button v-if="pending" type="button" :disabled="busy" @click="submit">
        重试原请求
      </button>
    </form>
  </main>
</template>
