<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
const route = useRoute(),
  router = useRouter();
const items = ref<components['schemas']['Audit'][]>([]),
  next = ref(''),
  error = ref(''),
  busy = ref(false);
const q = ref(typeof route.query.q === 'string' ? route.query.q : '');
const entity = ref(
  typeof route.query.entity_type === 'string' ? route.query.entity_type.toLowerCase() : '',
);
const entityId = ref(typeof route.query.entity_id === 'string' ? route.query.entity_id : ''),
  status = ref('');
/**
 * load 用途：按白名单条件分页读取不可删除的审计摘要。
 * 参数：after 为排他游标，空值重新筛选。
 * 返回值：加载 Promise，不展示请求正文或审计 diff。
 */
async function load(after = '') {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const params = new URLSearchParams({
      q: q.value,
      entity_type: entity.value,
      status: status.value,
    });
    if (entityId.value) params.set('entity_id', entityId.value);
    await router.replace({
      query: Object.fromEntries([...params].filter(([, value]) => value !== '')),
    });
    if (after) params.set('after_id', after);
    const page = await request<components['schemas']['AuditList']>(
      '/api/v1/admin/audit-logs?' + params,
    );
    items.value = after ? [...items.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch {
    error.value = '审计读取失败，请核对权限与筛选条件。';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>只读审计日志</h1>
    <p>保留操作与结果摘要，无在线删除入口。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <form @submit.prevent="load()">
      <fieldset :disabled="busy">
        <label for="audit-q">操作名称搜索</label
        ><input id="audit-q" v-model="q" maxlength="100" /><label for="audit-entity">对象类型</label
        ><select id="audit-entity" v-model="entity">
          <option value="">全部</option>
          <option
            v-for="kind in [
              'order',
              'admin',
              'role',
              'product',
              'carrier',
              'voucher',
              'voucher_batch',
              'inventory_document',
              'inventory_balance',
              'file',
              'content',
              'feedback',
              'after_sale',
              'shipment',
              'job',
              'outbox',
              'config',
            ]"
            :key="kind"
          >
            {{ kind }}
          </option></select
        ><label for="audit-id">对象 ID</label
        ><input id="audit-id" v-model="entityId" pattern="[1-9][0-9]*" /><label for="audit-status"
          >结果</label
        ><select id="audit-status" v-model="status">
          <option value="">全部</option>
          <option>SUCCESS</option>
          <option>AUTHORIZED</option>
          <option>DENIED</option></select
        ><button>筛选审计</button>
      </fieldset>
    </form>
    <p v-if="!items.length && !busy">暂无符合条件的审计。</p>
    <ol>
      <li v-for="item in items" :key="item.id">
        <strong>{{ item.action }} · {{ item.result }}</strong>
        <p>
          {{ item.entity_type }} {{ item.entity_id || '无对象' }} ·
          {{ new Date(item.created_at).toLocaleString('zh-CN') }}
        </p>
        <p>请求标识 {{ item.request_id }}</p>
      </li>
    </ol>
    <button v-if="next" :disabled="busy" @click="load(next)">加载更多审计</button>
  </main>
</template>
<style scoped>
main {
  overflow-wrap: anywhere;
}
input,
select {
  min-width: 0;
  max-width: 100%;
}
li {
  margin: 1rem 0;
}
</style>
