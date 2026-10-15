<script setup lang="ts">
import { flowError } from '../../api/flow-error';
import { ref, onMounted } from 'vue';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';
const order = ref<components['schemas']['Order']>(),
  error = ref(''),
  busy = ref(false);
const statusLabels: Record<string, string> = {
  PENDING: '待发货',
  SHIPPED: '已发货',
  COMPLETED: '已完成',
  CANCELLED: '已取消',
};

/**
 * load 用途：读取当前授权订单的脱敏详情。
 * 参数：无，使用 ORDER Cookie。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：更新显示状态并接收订单 CSRF，错误显示重试入口。
 */
async function load() {
  busy.value = true;
  error.value = '';
  try {
    order.value = await request('/api/v1/order');
  } catch (e) {
    error.value = e instanceof RequestError ? flowError(e) : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <section class="flow-page">
    <RouterLink to="/">首页</RouterLink>
    <h1>提货订单</h1>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy" @click="load">刷新订单</button
    ><template v-if="order"
      ><h2>{{ order.order_no }}</h2>
      <p>状态：{{ statusLabels[order.status] }}</p>
      <p>预约发货日：{{ order.ship_date }}</p>
      <ul>
        <li v-for="i in order.items" :key="i.id">{{ i.name }} × {{ i.quantity }}</li>
      </ul>
      <p>{{ order.recipient.name }} · {{ order.recipient.phone }}</p>
      <p>{{ order.recipient.region }}</p>
      <p>备注：{{ order.customer_note || '无' }}</p>
      <RouterLink to="/orders/tracking">查看物流</RouterLink>
      <RouterLink v-if="['SHIPPED', 'COMPLETED'].includes(order.status)" to="/orders/after-sales"
        >售后申请与进度</RouterLink
      >
      <RouterLink v-if="order.status === 'PENDING' && !order.deleted" to="/orders/edit"
        >修改收货信息</RouterLink
      ></template
    ><RouterLink v-if="error" to="/orders/query">查询订单</RouterLink>
  </section>
</template>
