<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
import { useSession } from '../../../stores/session';

const session = useSession();
const items = ref<components['schemas']['Feedback'][]>([]);
const status = ref('OPEN'),
  next = ref(''),
  error = ref('');
const resolution = ref<Record<string, string>>({});
const busy = ref(false);
const pending = ref<{ id: string; body: string; key: string }>();

/**
 * load 用途：读取反馈脱敏列表与处理状态。
 * 参数：after 固定降序游标。
 * 返回值：加载 Promise；失败保留现有数据。
 */
async function load(after = '') {
  busy.value = true;
  error.value = '';
  try {
    const page = await request<components['schemas']['FeedbackList']>(
      '/api/v1/admin/feedback?limit=20&status=' +
        status.value +
        (after ? '&after_id=' + after : ''),
    );
    items.value = after ? [...items.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch {
    error.value = '反馈读取失败或没有权限，请重试。';
  } finally {
    busy.value = false;
  }
}

/**
 * resolve 用途：按版本标记反馈处理，网络不确定时重用原请求。
 * 参数：item 当前反馈快照。
 * 返回值：完成 Promise；冲突要求刷新，不自动通知联系人。
 */
async function resolve(item: components['schemas']['Feedback']) {
  if (busy.value || (pending.value && pending.value.id !== item.id)) return;
  pending.value ??= {
    id: item.id,
    key: crypto.randomUUID(),
    body: JSON.stringify({ version: item.version, resolution: resolution.value[item.id] }),
  };
  busy.value = true;
  error.value = '';
  try {
    await request(
      '/api/v1/admin/feedback/' + item.id + '/resolve',
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
    await load();
  } catch (e) {
    if (e instanceof RequestError && e.status < 500) {
      pending.value = undefined;
      error.value =
        e.status === 409 ? '版本已变化，请刷新核对。' : '处理被拒绝，请核对权限和字段。';
    } else error.value = '结果尚未确认，请使用同一条反馈的原请求重试。';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>

<template>
  <main class="admin-page">
    <RouterLink to="/admin">管理工作台</RouterLink>
    <h1>反馈处理</h1>
    <p>联系人加密保存在服务端，此列表不返回联系人或授予订单访问权限。</p>
    <form class="filters" @submit.prevent="load()">
      <label for="feedback-status">状态</label
      ><select id="feedback-status" v-model="status" :disabled="!!pending">
        <option value="">全部</option>
        <option value="OPEN">待处理</option>
        <option value="RESOLVED">已处理</option></select
      ><button :disabled="busy || !!pending">刷新</button>
    </form>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="busy" role="status">处理中…</p>
    <p v-else-if="!items.length">暂无反馈。</p>
    <article v-for="item in items" :key="item.id">
      <h2>反馈 {{ item.id }} · {{ item.state }}</h2>
      <p>{{ item.message }}</p>
      <p v-if="item.resolution">处理说明：{{ item.resolution }}</p>
      <form
        v-if="item.state === 'OPEN' && session.session?.permissions.includes('feedback.write')"
        @submit.prevent="resolve(item)"
      >
        <label :for="'resolution-' + item.id">处理说明</label
        ><textarea
          :id="'resolution-' + item.id"
          v-model="resolution[item.id]"
          required
          maxlength="500"
          :disabled="busy || !!pending"
        /><button :disabled="busy || (!!pending && pending.id !== item.id)">
          {{ pending?.id === item.id ? '重试原请求' : '标记已处理' }}
        </button>
      </form>
    </article>
    <button v-if="next" :disabled="busy || !!pending" @click="load(next)">加载更多</button>
  </main>
</template>
<style scoped>
article {
  border: 1px solid #ddd;
  padding: 20px;
  margin: 20px 0;
}
select {
  font: inherit;
  min-height: 44px;
}
</style>
