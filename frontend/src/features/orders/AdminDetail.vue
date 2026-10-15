<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, RequestError } from '../../api/client';
import { useSession } from '../../stores/session';
import type { components } from '../../api/generated/schema';
import Tracking from './Tracking.vue';
const route = useRoute(),
  session = useSession(),
  item = ref<components['schemas']['Order']>(),
  address = ref<components['schemas']['OrderPrivate']>(),
  reason = ref(''),
  busy = ref(false),
  error = ref(''),
  pending = ref<{ action: string; body: string; key: string }>();
const path = '/api/v1/admin/orders/' + route.params.id;
const router = useRouter();
const permissions = computed(() => session.session?.permissions ?? []);
/**
 * load 用途：刷新后台脱敏订单详情。
 * 参数：无，使用已授权路由对象。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，清除先前展开的完整地址。
 */
async function load() {
  busy.value = true;
  error.value = '';
  address.value = undefined;
  try {
    item.value = await request(path);
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
/**
 * action 用途：执行订单状态操作、审计查看或受理自动面单。
 * 参数：name 为固定动作后缀。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：私密查看只存内存，失败显示错误，同键重试保留原正文。
 */
async function action(name: string) {
  if (!item.value) return;
  pending.value ??= {
    action: name,
    body: JSON.stringify({ version: item.value.version, reason: reason.value }),
    key: crypto.randomUUID(),
  };
  busy.value = true;
  error.value = '';
  try {
    const v = await request<
      | components['schemas']['Order']
      | components['schemas']['OrderPrivate']
      | components['schemas']['Job']
    >(
      path + '/' + pending.value.action,
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    if (pending.value.action === 'label-jobs') {
      pending.value = undefined;
      await router.push('/admin/jobs/' + (v as components['schemas']['Job']).id);
      return;
    }
    if (pending.value.action === 'private-view')
      address.value = v as components['schemas']['OrderPrivate'];
    else {
      item.value = v as components['schemas']['Order'];
      address.value = undefined;
    }
    pending.value = undefined;
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '网络结果未确认，请重试原请求';
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}
onMounted(load);
onUnmounted(() => (address.value = undefined));
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin/orders">订单列表</RouterLink>
    <h1>订单详情</h1>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy || !!pending" @click="load">刷新</button
    ><template v-if="item"
      ><h2>{{ item.order_no }}</h2>
      <p>
        {{ item.status }} · {{ item.deleted ? '回收站' : '正常显示' }} · 预约
        {{ item.ship_date }}
      </p>
      <ul>
        <li v-for="i in item.items" :key="i.id">{{ i.name }} × {{ i.quantity }}</li>
      </ul>
      <p>
        {{ item.recipient.name }} · {{ item.recipient.phone }} ·
        {{ item.recipient.region }}
      </p>
      <p>顾客备注：{{ item.customer_note || '无' }}</p>
      <RouterLink
        v-if="permissions.includes('after_sale.read')"
        :to="'/admin/after-sales?order_id=' + item.id"
        >查看关联售后</RouterLink
      >
      <RouterLink
        v-if="permissions.includes('order.ship') && item.status === 'PENDING' && !item.deleted"
        :to="'/admin/orders/' + item.id + '/ship'"
        >全单人工发货</RouterLink
      >
      <RouterLink
        v-if="permissions.includes('order.write') && item.status === 'PENDING' && !item.deleted"
        :to="'/admin/orders/' + item.id + '/edit'"
        >修改完整收货信息</RouterLink
      ><label for="action-reason">操作原因</label
      ><textarea
        id="action-reason"
        v-model="reason"
        maxlength="200"
        :disabled="!!pending || busy" /><button
        v-if="permissions.includes('order.pii') && item.fulfillment_mode === 'DELIVERY'"
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('private-view')"
      >
        审计查看完整地址</button
      ><button
        v-if="
          permissions.includes('order.ship') &&
          permissions.includes('file.read') &&
          item.status === 'PENDING' &&
          !item.deleted
        "
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('label-jobs')"
      >
        受理默认承运商自动面单（生成后需确认发货）</button
      ><button
        v-if="permissions.includes('order.cancel') && item.status === 'PENDING' && !item.deleted"
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('cancel')"
      >
        确认取消（释放预占，不返卡）</button
      ><button
        v-if="permissions.includes('order.complete') && item.status === 'SHIPPED' && !item.deleted"
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('complete')"
      >
        人工确认完成</button
      ><button
        v-if="
          permissions.includes('order.trash') &&
          ['CANCELLED', 'COMPLETED'].includes(item.status) &&
          !item.deleted
        "
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('trash')"
      >
        移入回收站</button
      ><button
        v-if="permissions.includes('order.restore') && item.deleted"
        :disabled="busy || !!pending || !reason.trim()"
        @click="action('restore')"
      >
        恢复显示</button
      ><button v-if="pending" :disabled="busy" @click="action(pending.action)">重试原请求</button>
      <aside v-if="address">
        <h3>完整地址（本次访问已审计）</h3>
        <p>{{ address.address.name }} · {{ address.address.phone }}</p>
        <p>
          {{ address.address.province_code }} / {{ address.address.city_code }} /
          {{ address.address.district_code }}
        </p>
        <p>{{ address.address.detail }}</p>
        <button @click="address = undefined">隐藏完整地址</button>
      </aside>
      <Tracking v-if="item.fulfillment_mode === 'DELIVERY'" :admin-order-id="item.id"
    /></template>
  </main>
</template>
<style scoped>
main { overflow-wrap: anywhere; }
button, input, textarea { max-width: 100%; min-width: 0; }
main > a { margin-inline-end: 12px; }
</style>
