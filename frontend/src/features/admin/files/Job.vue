<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError, download } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
import { useCommand } from '../../../api/command';

const route = useRoute(),
  session = useSession();
const job = ref<components['schemas']['Job']>();
const error = ref(''),
  reason = ref('');
const busy = ref(false);
const pending = ref<{ path: string; body: string; key: string }>();
const { busy: resolving, error: resolveError, pending: resolvePending, send } = useCommand();
const evidence = ref(''),
  conclusion = ref<'CONFIRMED_VOID' | 'CONFIRMED_CREATED' | 'CONFIRMED_FAILED'>('CONFIRMED_VOID'),
  carrier = ref(''),
  tracking = ref('');
let generation = 0;
let timer: ReturnType<typeof setInterval>;
const canCommit = computed(
  () =>
    job.value?.kind.startsWith('IMPORT_') &&
    job.value.phase === 'PREVIEW' &&
    job.value.state === 'READY' &&
    job.value.failed === 0 &&
    session.session?.permissions.includes('file.import'),
);

/**
 * load 用途：读取任务权威状态，失败停止自动轮询并保留原结果。
 * 参数：无。
 * 返回值：读取 Promise，不把 RUNNING 或 PREVIEW 当作导入完成。
 */
async function load() {
  if (busy.value || resolving.value || resolvePending.value) return;
  const current = generation;
  busy.value = true;
  error.value = '';
  try {
    const result = await request<components['schemas']['Job']>(
      '/api/v1/admin/jobs/' + route.params.id,
    );
    if (current === generation) job.value = result;
  } catch {
    if (current === generation) error.value = '任务读取失败、已过期或没有权限，请重试。';
  } finally {
    if (current === generation) busy.value = false;
  }
}

/**
 * commit 用途：明确确认无错误预检，按版本进入实际导入阶段。
 * 参数：无，使用当前任务版本和说明。
 * 返回值：受理 Promise；不确定时复用原版本、正文和幂等键。
 */
async function commit() {
  if (busy.value || !job.value) return;
  pending.value ??= {
    path: '/api/v1/admin/import-jobs/' + job.value.id + '/commit',
    body: JSON.stringify({ version: job.value.version, reason: reason.value }),
    key: crypto.randomUUID(),
  };
  busy.value = true;
  const current = generation;
  error.value = '';
  try {
    await request(
      pending.value.path,
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    if (current !== generation) return;
    pending.value = undefined;
    busy.value = false;
    await load();
  } catch (e) {
    if (current !== generation) return;
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
    error.value =
      e instanceof RequestError && e.status < 500
        ? '确认被拒绝，请刷新核对预检、文件期限与版本。'
        : '确认结果尚未确定，请重试原请求。';
  } finally {
    if (current === generation) busy.value = false;
  }
}

/**
 * downloadResult 用途：以当前管理员权限下载短期脱敏结果文件。
 * 参数：无。
 * 返回值：下载 Promise，过期或权限错误明确提示。
 */
async function downloadResult() {
  if (!job.value?.result_file_id) return;
  try {
    await download('/api/v1/admin/files/' + job.value.result_file_id + '/download');
  } catch (e) {
    error.value =
      e instanceof RequestError && e.status === 410
        ? '结果文件已过期。'
        : '文件下载失败或没有权限。';
  }
}
/**
 * resolveLabel 用途：带近期复核和供应商工单引用核对面单。
 * 参数：无，使用任务版本及用户明确的核对结论。
 * 返回值：提交 Promise，UNKNOWN 不通过自动重试解除。
 */
async function resolveLabel() {
  if (!job.value) return;
  const body: components['schemas']['ResolveJob'] = {
    version: job.value.version,
    resolution: job.value.state === 'READY' ? 'CONFIRMED_VOID' : conclusion.value,
    evidence_reference: evidence.value,
    reason: reason.value,
  };
  if (body.resolution === 'CONFIRMED_CREATED') {
    body.carrier_id = carrier.value;
    body.tracking_no = tracking.value;
  }
  const result = await send<components['schemas']['Job']>(
    '/api/v1/admin/jobs/' + job.value.id + '/resolve',
    body,
  );
  if (result) {
    job.value = result;
    evidence.value = '';
    reason.value = '';
  }
}
onMounted(() => {
  timer = setInterval(() => {
    if (
      !error.value &&
      !pending.value &&
      !resolvePending.value &&
      (job.value?.state === 'PENDING' || job.value?.state === 'RUNNING')
    )
      load();
  }, 2000);
});
watch(
  () => route.params.id,
  () => {
    generation++;
    job.value = undefined;
    pending.value = undefined;
    busy.value = false;
    load();
  },
  { immediate: true },
);
onUnmounted(() => clearInterval(timer));
</script>

<template>
  <main class="admin-page editor">
    <RouterLink to="/admin/files">返回导入导出</RouterLink>
    <h1>任务状态</h1>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="resolveError" role="alert">{{ resolveError }}</p>
    <button :disabled="busy || !!pending" @click="load">刷新任务</button>
    <section v-if="job">
      <h2>任务 {{ job.id }} · {{ job.kind }}</h2>
      <p>阶段 {{ job.phase }} · 状态 {{ job.state }} · 版本 {{ job.version }}</p>
      <p v-if="job.kind !== 'LABEL'">已检查 {{ job.processed }} 行，失败 {{ job.failed }} 行。</p>
      <section v-if="job.kind === 'LABEL'">
        <p>面单生成不会自动发货。MOCK 为测试面单，不会真实寄送。</p>
        <p v-if="job.carrier_id && job.tracking_no">
          承运商 {{ job.carrier_id }} · 单号 {{ job.tracking_no }}
        </p>
        <RouterLink
          v-if="job.state === 'READY' && job.order_id"
          :to="'/admin/orders/' + job.order_id + '/ship?label_job=' + job.id"
          >使用此面单确认发货</RouterLink
        >
        <RouterLink v-if="job.order_id" :to="'/admin/orders/' + job.order_id"
          >返回原订单</RouterLink
        >
        <p v-if="job.state === 'UNKNOWN'" role="alert">
          供应商结果未知，禁止重新创建。请查单核实后填写工单引用。
        </p>
        <form
          v-if="
            ['UNKNOWN', 'READY'].includes(job.state) &&
            session.session?.permissions.includes('integration.manage')
          "
          @submit.prevent="resolveLabel"
        >
          <RouterLink to="/admin/security">先完成身份复核</RouterLink>
          <fieldset :disabled="resolving || !!resolvePending">
            <legend>供应商结果核对</legend>
            <p v-if="job.state === 'READY'">仅当供应商明确确认作废，且尚未发货时可解除面单锁。</p>
            <template v-if="job.state === 'UNKNOWN'"
              ><label for="label-conclusion">已核实结论</label
              ><select id="label-conclusion" v-model="conclusion">
                <option value="CONFIRMED_VOID">确认已作废</option>
                <option value="CONFIRMED_CREATED">确认已创建</option>
                <option value="CONFIRMED_FAILED">确认创建失败</option>
              </select></template
            >
            <template v-if="job.state === 'UNKNOWN' && conclusion === 'CONFIRMED_CREATED'"
              ><label for="label-carrier">原承运商 ID</label
              ><input id="label-carrier" v-model="carrier" required pattern="[1-9][0-9]*" /><label
                for="label-tracking"
                >核实后的单号</label
              ><input
                id="label-tracking"
                v-model="tracking"
                required
                minlength="3"
                maxlength="80"
                pattern="[A-Za-z0-9-]+"
            /></template>
            <label for="label-evidence">供应商工单引用（不含客户信息）</label
            ><input id="label-evidence" v-model="evidence" required maxlength="200" />
            <label for="label-reason">核对说明</label
            ><textarea id="label-reason" v-model="reason" maxlength="200" />
            <button>提交已核实结论</button>
          </fieldset>
          <button v-if="resolvePending" type="button" :disabled="resolving" @click="resolveLabel">
            重试原核对请求
          </button>
        </form>
      </section>
      <p v-if="job.state === 'PENDING' || job.state === 'RUNNING'" role="status">
        任务已受理，等待 worker 处理；尚未完成。
      </p>
      <p v-else-if="job.phase === 'PREVIEW'">
        当前只是预检，未创建卡券或发货。只有预检无错误才可确认。
      </p>
      <p v-else-if="job.state === 'READY'" role="status">
        实际处理已结束{{ job.failed ? '，存在失败行，请下载结果核对。' : '。' }}
      </p>
      <p v-if="job.state === 'FAILED'" role="alert">
        任务失败。卡券导入失败不会留下部分卡；运单请核对每行结果。
      </p>
      <button v-if="job.result_file_id" @click="downloadResult">下载脱敏结果文件</button>
      <form v-if="canCommit || pending" @submit.prevent="commit">
        <label for="commit-reason">确认说明</label
        ><input id="commit-reason" v-model="reason" maxlength="200" :disabled="busy || !!pending" />
        <p>确认后卡券按整批导入；运单按行执行真实发货并扣减库存。此操作不能通过预检页面撤销。</p>
        <button :disabled="busy">{{ pending ? '重试原确认请求' : '确认执行实际导入' }}</button>
      </form>
    </section>
  </main>
</template>

<style scoped>
main {
  overflow-wrap: anywhere;
}
input {
  min-width: 0;
  max-width: 100%;
}
</style>
