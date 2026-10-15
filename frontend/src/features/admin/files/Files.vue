<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError, download } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';

const router = useRouter(),
  session = useSession();
const selected = ref<File>(),
  fileInput = ref<HTMLInputElement>();
const uploaded = ref<components['schemas']['File']>();
const kind = ref<'IMPORT_VOUCHERS' | 'IMPORT_SHIPMENTS'>('IMPORT_VOUCHERS');
const batchID = ref(''),
  carrierID = ref('');
const exportKind = ref<'VOUCHER_METADATA' | 'ORDERS'>('VOUCHER_METADATA');
const exportBatch = ref(''),
  exportStatus = ref('');
const busy = ref(false),
  error = ref('');
const uploadAction = ref<{ file: File; key: string }>();
const action = ref<{ path: string; body: string; key: string }>();
const locked = computed(() => busy.value || !!uploadAction.value || !!action.value);
const canImport = computed(() => session.session?.permissions.includes('file.import'));
const canExport = computed(() => session.session?.permissions.includes('file.export'));
const files = ref<components['schemas']['File'][]>([]),
  next = ref(''),
  fileState = ref(''),
  fileKind = ref(''),
  listing = ref(false),
  listError = ref('');
/**
 * loadFiles 用途：分页读取授权可见文件，不展示存储路径或原文件内容。
 * 参数：after 为排他游标，空值重新筛选。
 * 返回值：加载 Promise，失败保留刷新入口。
 */
async function loadFiles(after = '') {
  if (listing.value || locked.value) return;
  listing.value = true;
  listError.value = '';
  try {
    const query = new URLSearchParams({ q: fileKind.value, status: fileState.value, limit: '20' });
    if (after) query.set('after_id', after);
    const page = await request<components['schemas']['FileList']>('/api/v1/admin/files?' + query);
    files.value = after ? [...files.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch {
    listError.value = '文件目录读取失败或没有权限。';
  } finally {
    listing.value = false;
  }
}
/**
 * downloadFile 用途：显式授权下载报表或近期复核后的短期凭证。
 * 参数：id 为当前可见文件引用。
 * 返回值：下载 Promise，过期、权限和复核失败明确提示。
 */
async function downloadFile(id: string) {
  try {
    await download('/api/v1/admin/files/' + id + '/download');
  } catch (e) {
    listError.value =
      e instanceof RequestError && e.status === 410
        ? '文件已过期。'
        : '下载失败，请核对归属、权限和身份复核。';
  }
}
onMounted(() => loadFiles());

/**
 * choose 用途：选择 CSV，已有不确定上传时禁止替换原文件。
 * 参数：event 原生文件选择事件。
 * 返回值：无，文件只保留在浏览器内存。
 */
function choose(event: Event) {
  if (locked.value) return;
  selected.value = (event.target as HTMLInputElement).files?.[0];
  uploaded.value = undefined;
}

/**
 * upload 用途：把用户选择的 CSV 发送到服务端加密暂存。
 * 参数：无。
 * 返回值：上传 Promise，结果不确定时保留原文件与键，成功清空文件选择。
 */
async function upload() {
  if (busy.value || (!selected.value && !uploadAction.value)) return;
  uploadAction.value ??= { file: selected.value!, key: crypto.randomUUID() };
  busy.value = true;
  error.value = '';
  const form = new FormData();
  form.set('kind', 'IMPORT');
  form.set('file', uploadAction.value.file);
  try {
    uploaded.value = await request(
      '/api/v1/admin/files',
      { method: 'POST', body: form },
      uploadAction.value.key,
    );
    uploadAction.value = undefined;
    selected.value = undefined;
    if (fileInput.value) fileInput.value.value = '';
  } catch (e) {
    if (e instanceof RequestError && e.status < 500) uploadAction.value = undefined;
    error.value =
      e instanceof RequestError && e.status < 500
        ? '上传被拒绝，请检查权限、UTF-8 CSV 格式和 5 MiB 限制。'
        : '上传结果不确定，请重试原文件。';
  } finally {
    busy.value = false;
  }
}

/**
 * create 用途：创建预检或脱敏导出任务，原键确认不确定结果。
 * 参数：type 为 import 或 export；已有 pending 时仅重试原动作。
 * 返回值：提交 Promise，成功进入可轮询任务页，不显示业务已完成。
 */
async function create(type: 'import' | 'export') {
  if (busy.value || uploadAction.value) return;
  const body =
    type === 'import'
      ? {
          file_id: uploaded.value?.id,
          kind: kind.value,
          ...(kind.value === 'IMPORT_VOUCHERS'
            ? { batch_id: batchID.value }
            : carrierID.value
              ? { carrier_id: carrierID.value }
              : {}),
        }
      : {
          kind: exportKind.value,
          ...(exportKind.value === 'VOUCHER_METADATA' && exportBatch.value
            ? { batch_id: exportBatch.value }
            : {}),
          ...(exportStatus.value ? { status: exportStatus.value } : {}),
        };
  action.value ??= {
    path: '/api/v1/admin/' + (type === 'import' ? 'import-jobs' : 'export-jobs'),
    body: JSON.stringify(body),
    key: crypto.randomUUID(),
  };
  busy.value = true;
  error.value = '';
  try {
    const job = await request<components['schemas']['Job']>(
      action.value.path,
      { method: 'POST', body: action.value.body },
      action.value.key,
    );
    action.value = undefined;
    await router.push('/admin/jobs/' + job.id);
  } catch (e) {
    if (e instanceof RequestError && e.status < 500) action.value = undefined;
    error.value =
      e instanceof RequestError && e.status < 500
        ? '任务未受理，请核对权限、文件归属、草稿批次及筛选。'
        : '受理结果未确认，请重试原请求。';
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <main class="admin-page editor">
    <RouterLink to="/admin">管理工作台</RouterLink>
    <h1>导入与导出</h1>
    <p v-if="error" role="alert">{{ error }}</p>
    <section v-if="canImport">
      <h2>CSV 导入</h2>
      <p>UTF-8，最多 1000 行、5 MiB。上传只暂存密文；预检完成后还需确认导入。</p>
      <p>
        卡券列：code,secret,valid_from,valid_until。运单列：order_no,carrier_code,tracking_no,expected_version。
      </p>
      <form @submit.prevent="upload">
        <label for="import-file">CSV 文件</label
        ><input
          id="import-file"
          ref="fileInput"
          type="file"
          accept=".csv,text/csv"
          :disabled="locked"
          @change="choose"
        /><button :disabled="busy || (!selected && !uploadAction)">
          {{ uploadAction ? '重试原文件上传' : '加密上传' }}
        </button>
      </form>
      <p v-if="uploaded" role="status">已暂存文件 {{ uploaded.id }}，尚未导入业务数据。</p>
      <form v-if="uploaded" @submit.prevent="create('import')">
        <fieldset :disabled="locked">
          <legend>预检参数</legend>
          <label for="import-kind">导入种类</label
          ><select id="import-kind" v-model="kind">
            <option value="IMPORT_VOUCHERS">卡券（整批原子）</option>
            <option value="IMPORT_SHIPMENTS">运单（逐行发货）</option>
          </select>
          <template v-if="kind === 'IMPORT_VOUCHERS'"
            ><label for="import-batch">草稿批次 ID</label
            ><input id="import-batch" v-model="batchID" required pattern="[1-9][0-9]*" /><small
              >CSV 行数必须等于批次数量；已生成或封存批次不能导入。</small
            ></template
          >
          <template v-else
            ><label for="import-carrier">限定承运商 ID（可选）</label
            ><input id="import-carrier" v-model="carrierID" pattern="[1-9][0-9]*" /><small
              >填写时，每行 carrier_code 必须对应此承运商；发货还需 order.ship 权限。</small
            ></template
          >
          <button>创建预检任务</button>
        </fieldset>
      </form>
    </section>
    <section v-if="canExport">
      <h2>脱敏报表</h2>
      <p>最多十万行、10 MiB；不导出卡密、完整电话或详细地址。结果二十四小时过期。</p>
      <form @submit.prevent="create('export')">
        <fieldset :disabled="locked">
          <legend>导出筛选</legend>
          <label for="export-kind">报表种类</label
          ><select id="export-kind" v-model="exportKind" @change="exportStatus = ''">
            <option value="VOUCHER_METADATA">卡券元数据</option>
            <option value="ORDERS">订单</option>
          </select>
          <template v-if="exportKind === 'VOUCHER_METADATA'"
            ><label for="export-batch">批次 ID（可选）</label
            ><input id="export-batch" v-model="exportBatch" pattern="[1-9][0-9]*"
          /></template>
          <label for="export-status">状态</label
          ><select id="export-status" v-model="exportStatus">
            <option value="">全部</option>
            <template v-if="exportKind === 'ORDERS'"
              ><option value="PENDING">待发</option>
              <option value="SHIPPED">已发</option>
              <option value="COMPLETED">完成</option>
              <option value="CANCELLED">取消</option></template
            ><template v-else
              ><option value="SEALED">已封存</option>
              <option value="EXPORTABLE">可导出凭证</option></template
            >
          </select>
          <button>创建脱敏导出任务</button>
        </fieldset>
      </form>
    </section>
    <button v-if="action" :disabled="busy" @click="create('import')">确认原任务请求结果</button>
    <p>已有任务可通过已保存的 /admin/jobs/任务ID 地址查看。系统没有公开文件链接。</p>
    <section v-if="session.session?.permissions.includes('file.read')">
      <h2>受控文件目录</h2>
      <p>导入原文件不可下载。凭证导出需额外权限与近期复核，封存后不可恢复下载。</p>
      <RouterLink to="/admin/security">身份复核</RouterLink>
      <p v-if="listError" role="alert">{{ listError }}</p>
      <form @submit.prevent="loadFiles()">
        <fieldset :disabled="listing || locked">
          <label for="file-kind">文件用途</label
          ><select id="file-kind" v-model="fileKind">
            <option value="">全部</option>
            <option
              v-for="value in ['IMAGE', 'IMPORT', 'CREDENTIAL_EXPORT', 'REPORT']"
              :key="value"
            >
              {{ value }}
            </option></select
          ><label for="file-state">文件状态</label
          ><select id="file-state" v-model="fileState">
            <option value="">全部</option>
            <option v-for="value in ['STAGED', 'READY', 'EXPIRED', 'DELETED']" :key="value">
              {{ value }}
            </option></select
          ><button>刷新文件目录</button>
        </fieldset>
      </form>
      <p v-if="!files.length && !listing">暂无文件。</p>
      <ul>
        <li v-for="file in files" :key="file.id">
          <p>
            文件 {{ file.id }} · {{ file.kind }} · {{ file.state }} · {{ file.size_bytes }} 字节
          </p>
          <p v-if="file.expires_at">
            有效期至 {{ new Date(file.expires_at).toLocaleString('zh-CN') }}
          </p>
          <button
            v-if="['REPORT', 'CREDENTIAL_EXPORT'].includes(file.kind) && file.state === 'READY'"
            :disabled="listing || locked"
            @click="downloadFile(file.id)"
          >
            授权下载文件 {{ file.id }}
          </button>
        </li>
      </ul>
      <button v-if="next" :disabled="listing || locked" @click="loadFiles(next)">
        加载更多文件
      </button>
    </section>
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

select {
  font: inherit;
  min-height: 44px;
  padding: 8px;
}
small {
  color: #78716c;
}
</style>
