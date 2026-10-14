<script setup lang="ts">
import './vouchers.css';
import { onMounted, onUnmounted, ref, computed } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError, download } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
const route = useRoute(),
  store = useSession(),
  batch = ref<components['schemas']['Batch']>(),
  busy = ref(false),
  error = ref(''),
  reason = ref(''),
  pending = ref<{ body: string; key: string; action: string }>();
const canGenerate = computed(() =>
    store.session?.permissions.includes('voucher.generate'),
  ),
  canSeal = computed(() => store.session?.permissions.includes('voucher.seal')),
  canExport = computed(() =>
    ['file.read', 'voucher.export', 'voucher.generate'].every((p) =>
      store.session?.permissions.includes(p),
    ),
  );
let timer: ReturnType<typeof setTimeout> | undefined,
  disposed = false;
const path = '/api/v1/admin/voucher-batches/' + route.params.id;

/**
 * load 用途：读取批次并在生成时轮询。
 * 参数：无，使用路由批次标识。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：读取失败显示错误，卸载后的返回不修改组件，定时器由卸载清理。
 */
async function load() {
  try {
    const b = await request<components['schemas']['Batch']>(path);
    if (disposed) return;
    batch.value = b;
    if (b.state === 'GENERATING') timer = setTimeout(load, 3000);
  } catch (e) {
    if (!disposed)
      error.value = e instanceof RequestError ? e.detail.code : '读取失败，请刷新';
  }
}

/**
 * mutate 用途：提交生成或不可逆封存动作。
 * 参数：action 为 generate/seal。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示服务端业务码，结果不确定保留同键同正文。
 * 约束：成功更新权威批次。
 */
async function mutate(action: string) {
  if (!batch.value) return;
  pending.value ??= {
    body: JSON.stringify(
      action === 'seal'
        ? { version: batch.value.version, reason: reason.value }
        : { version: batch.value.version },
    ),
    key: crypto.randomUUID(),
    action,
  };
  busy.value = true;
  error.value = '';
  try {
    batch.value = await request(
      path + '/' + pending.value.action,
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
    if (timer) clearTimeout(timer);
    await load();
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ({
            REAUTH_REQUIRED: '请先完成身份复核，再重试',
            VERSION_CONFLICT: '批次版本已变化，请刷新后重试',
            FORBIDDEN: '没有操作权限',
          }[e.detail.code] ?? e.detail.code)
        : '网络异常，请重试原请求';
    if (e instanceof RequestError && e.status < 500) {
      pending.value = undefined;
      await load();
    }
  } finally {
    busy.value = false;
  }
}

/**
 * exportFile 用途：经权限复核下载本批凭证。
 * 参数：无，使用当前批次文件标识。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，凭证不进入组件 JSON 状态。
 */
async function exportFile() {
  if (!batch.value?.export_file_id) return;
  busy.value = true;
  error.value = '';
  try {
    await download('/api/v1/admin/files/' + batch.value.export_file_id + '/download');
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '下载失败，请重试';
  } finally {
    busy.value = false;
  }
}

/**
 * refresh 用途：取消旧轮询并重新读取批次。
 * 参数：无。
 * 返回值：无。
 * 约束：启动读取，读取函数负责反馈错误。
 */
function refresh() {
  if (timer) clearTimeout(timer);
  load();
}
onMounted(load);
onUnmounted(() => {
  disposed = true;
  if (timer) clearTimeout(timer);
});
</script>
<template>
  <main class="admin-page voucher-page">
    <div class="page-title">
      <h1>批次详情</h1>
      <RouterLink to="/admin/voucher-batches">批次列表</RouterLink
      ><RouterLink to="/admin/security">身份复核</RouterLink>
    </div>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="batch"
      ><h2>{{ batch.label }}</h2>
      <p>{{ batch.batch_no }} · {{ batch.state }}</p>
      <p>
        已生成 {{ batch.generated_count }} / {{ batch.quantity }} 张；生成期间自动刷新。
      </p>
      <p>权益商品 ID：{{ batch.product_ids.join('、') }}</p>
      <p>
        有效期：{{ new Date(batch.valid_from).toLocaleString() }} 至
        {{ new Date(batch.valid_until).toLocaleString() }}
      </p>
      <p>
        卡号 {{ batch.code_length }} 位，{{
          batch.code_mode === 'RANDOM' ? '随机' : '顺序'
        }}；密码 {{ batch.secret_length }} 位。
      </p>
      <button
        v-if="canGenerate && (batch.state === 'DRAFT' || batch.state === 'FAILED')"
        :disabled="busy || !!pending"
        @click="mutate('generate')"
      >
        {{ batch.state === 'FAILED' ? '重新生成' : '提交生成任务' }}
      </button>
      <p v-if="batch.state === 'FAILED'">
        生成失败，整批未发布。顺序编号请检查是否与已有卡号重叠；可重试或建立新批次。
      </p>
      <p v-if="batch.state === 'GENERATED'">
        请在 24 小时内导出凭证，随后封存；封存后不能再次下载。
      </p>
      <button
        v-if="canExport && batch.state === 'GENERATED' && batch.export_file_id"
        :disabled="busy || !!pending"
        @click="exportFile"
      >
        下载卡密文件
      </button>
      <form
        v-if="canSeal && batch.state === 'GENERATED'"
        @submit.prevent="mutate('seal')"
      >
        <label
          >封存原因 <input v-model="reason" maxlength="200" :disabled="!!pending"
        /></label>
        <p>
          封存会永久关闭本批导出。{{
            batch.activate_on_seal
              ? '本批将在封存时自动激活。'
              : '本批封存后仍停用，可在卡券页激活。'
          }}
        </p>
        <button :disabled="busy || !!pending">确认封存</button>
      </form>
      <button v-if="pending" :disabled="busy" @click="mutate(pending.action)">
        重试原请求</button
      ><RouterLink :to="'/admin/vouchers?batch_id=' + batch.id"
        >查看本批卡券</RouterLink
      ></template
    ><button :disabled="busy" @click="refresh">刷新</button>
  </main>
</template>
