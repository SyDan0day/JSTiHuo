<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';
const items = ref<components['schemas']['Order'][]>([]),
  status = ref('PENDING'),
  q = ref(''),
  next = ref(''),
  error = ref(''),
  busy = ref(false),
  deleted = ref(false);
/**
 * load 用途：读取后台脱敏订单。
 * 参数：after 为可空排他游标。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，成功更新列表，不执行订单状态变更。
 */
async function load(after = '') {
  busy.value = true;
  error.value = '';
  try {
    const p = new URLSearchParams({
      limit: '20',
      status: status.value,
      q: q.value,
      deleted: String(deleted.value),
    });
    if (after) p.set('after_id', after);
    const r = await request<components['schemas']['OrderList']>(
      '/api/v1/admin/orders?' + p,
    );
    items.value = r.items;
    next.value = r.next_after_id ?? '';
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <div class="page-title">
      <h1>订单管理</h1>
      <RouterLink to="/admin">工作台</RouterLink>
    </div>
    <form class="filters" @submit.prevent="load()">
      <input v-model="q" placeholder="订单号" maxlength="100" /><select v-model="status">
        <option value="">全部状态</option>
        <option value="PENDING">待发货</option>
        <option value="SHIPPED">已发货</option>
        <option value="COMPLETED">已完成</option>
        <option value="CANCELLED">已取消</option></select
      ><label><input v-model="deleted" type="checkbox" />回收站</label
      ><button :disabled="busy">查询</button>
    </form>
    <p role="alert">{{ error }}</p>
    <article v-for="o in items" :key="o.id">
      <h2>
        <RouterLink :to="'/admin/orders/' + o.id">{{ o.order_no }}</RouterLink>
      </h2>
      <p>{{ o.status }} · 预约 {{ o.ship_date }}</p>
      <ul>
        <li v-for="i in o.items" :key="i.id">{{ i.name }} × {{ i.quantity }}</li>
      </ul>
      <p>{{ o.recipient.name }} · {{ o.recipient.phone }} · {{ o.recipient.region }}</p>
      <p>{{ o.customer_note || '无备注' }}</p>
    </article>
    <p v-if="!items.length">暂无匹配订单</p>
    <button v-if="next" :disabled="busy" @click="load(next)">下一页</button>
  </main>
</template>
