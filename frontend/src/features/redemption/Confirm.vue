<script setup lang="ts">
import { flowError } from '../../api/flow-error';
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError } from '../../api/client';
import { useRedemption } from '../../stores/redemption';
import type { components } from '../../api/generated/schema';
const store = useRedemption(),
  router = useRouter(),
  error = ref(''),
  busy = ref(false);

/**
 * submit 用途：提交最终兑换并在结果不确定时保留原请求。
 * 参数：无，使用内存选品、地址和日期。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功清理 PII 并进入订单页。
 * 约束：网络/服务端异常保留相同正文和幂等键，确定业务失败允许重新修改。
 */
async function submit() {
  if (busy.value) return;
  store.pending ??= {
    body: JSON.stringify({
      policy_version: store.options?.policy_version,
      items: store.selected.map((id) => ({ product_id: id, quantity: 1 })),
      address: store.address,
      ship_date: store.date,
      customer_note: store.note,
    }),
    key: crypto.randomUUID(),
  };
  busy.value = true;
  error.value = '';
  try {
    await request<components['schemas']['Order']>(
      '/api/v1/redemptions',
      { method: 'POST', body: store.pending.body },
      store.pending.key,
    );
    store.clear();
    await router.replace('/orders/result');
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ({
            OUT_OF_STOCK: '商品库存不足，请重新选品',
            CAPACITY_EXCEEDED: '预约日期已满，请重新选择',
            ALREADY_REDEEMED: '此卡已兑换，原请求可重试确认结果',
            REGION_RESTRICTED: '所选商品不配送到该地区',
            DATE_UNAVAILABLE: '日期或商品有效期已变化，请重新选择',
            SELECTION_INVALID: '商品选择不符合权益规则',
          }[e.detail.code] ?? flowError(e))
        : '结果尚未确认，请使用原请求重试';
    if (e instanceof RequestError && e.status < 500 && e.detail.code !== 'ALREADY_REDEEMED')
      store.pending = undefined;
  } finally {
    busy.value = false;
  }
}
onMounted(() => {
  if (!store.options || store.selected.length !== store.options.selection_count || !store.date)
    router.replace('/redeem/select');
});
</script>
<template>
  <section class="flow-page auth">
    <h1>确认提货</h1>
    <p>提交后卡券将消费一次，并生成配送订单。</p>
    <ul>
      <li v-for="id in store.selected" :key="id">
        {{ store.options?.products.find((p) => p.id === id)?.name }} × 1
      </li>
    </ul>
    <p>{{ store.address.name }} · {{ store.address.phone }}</p>
    <p>{{ store.regionLabel }} {{ store.address.detail }}</p>
    <p>预约发货日：{{ store.date }}</p>
    <p>备注：{{ store.note || '无' }}</p>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy" @click="submit">
      {{ busy ? '正在确认…' : store.pending ? '重试原请求' : '确认提交' }}</button
    ><RouterLink v-if="!store.pending" to="/redeem/address">返回修改</RouterLink>
  </section>
</template>
