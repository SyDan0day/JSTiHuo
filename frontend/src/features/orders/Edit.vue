<script setup lang="ts">
import { flowError } from '../../api/flow-error';
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import AddressFields from '../../components/AddressFields.vue';
import OrderStepUp from '../../components/OrderStepUp.vue';
import { getOrderScope, request, RequestError } from '../../api/client';
import { useSession } from '../../stores/session';
import type { components } from '../../api/generated/schema';

const route = useRoute();
const router = useRouter();
const session = useSession();
const isAdmin = route.path.startsWith('/admin/');
const path = isAdmin ? '/api/v1/admin/orders/' + route.params.id : '/api/v1/order';
const order = ref<components['schemas']['Order']>();
const form = ref<components['schemas']['OrderEdit']>({
  version: 1,
  address: {
    name: '',
    phone: '',
    province_code: '',
    city_code: '',
    district_code: '',
    detail: '',
  },
  ship_date: '',
  customer_note: '',
});
const days = ref<components['schemas']['CalendarDay'][]>([]);
const writable = ref(isAdmin);
const busy = ref(false);
const ready = ref(false);
const error = ref('');
const pending = ref<{ body: string; key: string }>();
const canEdit = computed(() =>
  isAdmin ? session.session?.permissions.includes('order.write') : writable.value,
);

/**
 * load 用途：读取脱敏订单和日期建议，完整加载后开放编辑。
 * 参数：无，使用当前路由与同源 Cookie。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，顾客读取日历不占名额，不预填旧 PII。
 */
async function load() {
  if (busy.value || pending.value) return;
  busy.value = true;
  ready.value = false;
  days.value = [];
  error.value = '';
  try {
    order.value = await request(path);
    if (!order.value) return;
    form.value.version = order.value.version;
    form.value.ship_date = order.value.ship_date ?? '';
    form.value.customer_note = order.value.customer_note;
    if (!isAdmin) {
      writable.value = getOrderScope() === 'WRITE';
      days.value = (
        await request<components['schemas']['Calendar']>('/api/v1/redemption/calendar')
      ).items;
    }
    ready.value = true;
  } catch (e) {
    error.value = e instanceof RequestError ? flowError(e) : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}

/**
 * submit 用途：提交完整替换地址和日期，并支持结果不确定时的同键恢复。
 * 参数：无，使用页面版本快照和地址表单。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功清理 PII 并返回详情。
 * 约束：业务失败解除待确认，网络错误保留原正文。
 */
async function submit() {
  if (busy.value || !ready.value || !canEdit.value) return;
  pending.value ??= { body: JSON.stringify(form.value), key: crypto.randomUUID() };
  busy.value = true;
  error.value = '';
  try {
    await request(path, { method: 'PATCH', body: pending.value.body }, pending.value.key);
    pending.value = undefined;
    form.value.address = {
      name: '',
      phone: '',
      province_code: '',
      city_code: '',
      district_code: '',
      detail: '',
    };
    await router.replace(isAdmin ? '/admin/orders/' + route.params.id : '/orders/result');
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ((
            {
              VERSION_CONFLICT: '订单已变化，请刷新后核对新版本',
              CAPACITY_EXCEEDED: '新日期已满，原名额保持不变，请重选',
              REGION_RESTRICTED: '所选商品不配送至该地区',
              ILLEGAL_TRANSITION: '已超过修改截止时间或订单不再待发',
            } as Record<string, string>
          )[e.detail.code] ?? flowError(e))
        : '结果未确认，请重试原请求';
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}

onMounted(load);
</script>
<template>
  <component :is="isAdmin ? 'main' : 'section'" class="auth flow-page">
    <RouterLink :to="isAdmin ? '/admin/orders/' + route.params.id : '/orders/result'"
      >返回详情</RouterLink
    >
    <h1>修改收货信息</h1>
    <p>只支持待发配送订单，预约日前一天 17 点前修改；需输入完整新地址。</p>
    <p v-if="order">
      {{ order.order_no }} · {{ order.recipient.name }} · {{ order.recipient.phone }}
    </p>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy || !!pending" @click="load">刷新权威版本</button>
    <OrderStepUp v-if="!isAdmin && !writable" @verified="writable = true" />
    <form v-if="canEdit && order?.status === 'PENDING'" @submit.prevent="submit">
      <AddressFields v-model="form.address" :disabled="busy || !ready || !!pending" />
      <label for="edit-ship-date">预约发货日期</label>
      <input
        v-if="isAdmin"
        id="edit-ship-date"
        v-model="form.ship_date"
        type="date"
        required
        :disabled="busy || !ready || !!pending"
      />
      <select
        v-else
        id="edit-ship-date"
        v-model="form.ship_date"
        required
        :disabled="busy || !ready || !!pending"
      >
        <option v-for="d in days" :key="d.date" :value="d.date" :disabled="!d.available">
          {{ d.date }} · {{ d.available ? '可预约' : '不可预约' }}
        </option>
      </select>
      <label for="edit-note">备注</label
      ><textarea
        id="edit-note"
        v-model="form.customer_note"
        maxlength="200"
        :disabled="busy || !ready || !!pending"
      />
      <button :disabled="busy || !ready">{{ pending ? '重试原请求' : '确认替换收货信息' }}</button>
    </form>
  </component>
</template>
<style scoped>
select {
  font: inherit;
  min-height: 44px;
  padding: 8px;
}
</style>
