<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { request } from '../../../api/client';
import { useCommand } from '../../../api/command';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
type Notice = components['schemas']['Notification'];
const session = useSession(),
  permissions = computed(() => session.session?.permissions ?? []);
const { busy, error, pending, send } = useCommand();
const notices = ref<Notice[]>([]),
  next = ref(''),
  selected = ref<Notice>();
const status = ref(''),
  q = ref(''),
  reason = ref(''),
  evidence = ref(''),
  loadError = ref(''),
  loading = ref(false);
/**
 * load 用途：分页读取通知状态，不读取联系人和消息正文。
 * 参数：after 为排他游标，空值重新筛选。
 * 返回值：加载 Promise，失败保留恢复入口。
 */
async function load(after = '') {
  if (busy.value || pending.value || loading.value) return;
  loading.value = true;
  loadError.value = '';
  try {
    const params = new URLSearchParams({ status: status.value, q: q.value, limit: '20' });
    if (after) params.set('after_id', after);
    const page = await request<components['schemas']['NotificationList']>(
      '/api/v1/admin/notifications?' + params,
    );
    notices.value = after ? [...notices.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
    if (selected.value)
      selected.value = notices.value.find((item) => item.id === selected.value?.id);
  } catch {
    loadError.value = '通知读取失败或没有权限。';
  } finally {
    loading.value = false;
  }
}
/**
 * act 用途：重试确定失败通知或依据证据核对 UNKNOWN。
 * 参数：action 为固定 retry 或核对结论。
 * 返回值：提交 Promise，网络不确定保留原命令。
 */
async function act(action: 'retry' | 'CONFIRMED_SENT' | 'CONFIRMED_NOT_SENT') {
  if (!selected.value) return;
  const body =
    action === 'retry'
      ? { version: selected.value.version, reason: reason.value }
      : {
          version: selected.value.version,
          reason: reason.value,
          evidence_reference: evidence.value,
          resolution: action,
        };
  const result = await send<Notice>(
    '/api/v1/admin/notifications/' +
      selected.value.id +
      (action === 'retry' ? '/retry' : '/resolve'),
    body,
  );
  if (result) {
    selected.value = result;
    reason.value = '';
    evidence.value = '';
    await load();
  }
}
/**
 * retryPending 用途：只重放原未确认处置。
 * 参数：无。
 * 返回值：恢复 Promise，成功后刷新列表。
 */
async function retryPending() {
  if (pending.value && (await send(pending.value.path, {}))) await load();
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>通知任务</h1>
    <p>MOCK 仅记录模拟发送。UNKNOWN 必须先核对供应商结果，禁止直接重发。</p>
    <RouterLink to="/admin/security">身份复核</RouterLink>
    <p v-if="error || loadError" role="alert">{{ error || loadError }}</p>
    <form @submit.prevent="load()">
      <fieldset :disabled="busy || loading || !!pending">
        <label for="notice-status">状态</label
        ><select id="notice-status" v-model="status">
          <option value="">全部</option>
          <option
            v-for="state in ['PENDING', 'SENT', 'FAILED', 'UNKNOWN', 'SUPPRESSED']"
            :key="state"
          >
            {{ state }}
          </option></select
        ><label for="notice-template">模板搜索</label
        ><input id="notice-template" v-model="q" maxlength="100" /><button>刷新通知</button>
      </fieldset>
    </form>
    <p v-if="!notices.length && !loading">暂无通知。</p>
    <ul>
      <li v-for="notice in notices" :key="notice.id">
        <button :disabled="busy || !!pending" @click="selected = notice">
          通知 {{ notice.id }} · {{ notice.template_key }} · {{ notice.channel }} ·
          {{ notice.state }}
        </button>
      </li>
    </ul>
    <button v-if="next" :disabled="busy || loading || !!pending" @click="load(next)">
      加载更多通知
    </button>
    <section v-if="selected">
      <h2>通知 {{ selected.id }} · {{ selected.state }}</h2>
      <p>版本 {{ selected.version }} · {{ selected.last_error_code || '无错误记录' }}</p>
      <RouterLink
        v-if="selected.order_id && permissions.includes('order.read')"
        :to="'/admin/orders/' + selected.order_id"
        >查看关联订单</RouterLink
      >
      <fieldset v-if="['FAILED', 'UNKNOWN'].includes(selected.state)" :disabled="busy || !!pending">
        <label for="notice-reason">操作说明</label
        ><textarea id="notice-reason" v-model="reason" maxlength="200" />
        <button
          v-if="selected.state === 'FAILED' && permissions.includes('notification.retry')"
          :disabled="!reason.trim()"
          @click="act('retry')"
        >
          重试已确认失败通知
        </button>
        <template v-if="selected.state === 'UNKNOWN' && permissions.includes('notification.manage')"
          ><label for="notice-evidence">供应商核对工单引用（不含联系人）</label
          ><input id="notice-evidence" v-model="evidence" maxlength="200" /><button
            :disabled="!evidence.trim()"
            @click="act('CONFIRMED_SENT')"
          >
            已核实发送成功</button
          ><button :disabled="!evidence.trim()" @click="act('CONFIRMED_NOT_SENT')">
            已核实未发送
          </button></template
        >
      </fieldset>
    </section>
    <button v-if="pending" :disabled="busy" @click="retryPending">重试原通知处置请求</button>
  </main>
</template>
<style scoped>
main {
  overflow-wrap: anywhere;
}
input,
select,
textarea {
  min-width: 0;
  max-width: 100%;
}
</style>
