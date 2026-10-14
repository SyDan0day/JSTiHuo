<script setup lang="ts">
import './vouchers.css';
import { onMounted, ref, computed } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError, download } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
type Voucher = components['schemas']['Voucher'];
const route = useRoute(),
  store = useSession(),
  items = ref<Voucher[]>([]),
  selected = ref<Voucher[]>([]),
  q = ref(''),
  batch = ref(String(route.query.batch_id ?? '')),
  active = ref(''),
  redeemed = ref(''),
  next = ref(''),
  error = ref(''),
  busy = ref(false),
  reason = ref(''),
  action = ref('ACTIVATE'),
  from = ref(''),
  until = ref(''),
  resetTarget = ref<Voucher>(),
  length = ref(12),
  format = ref('MIXED'),
  delivery = ref<components['schemas']['CredentialDelivery']>();
const pending = ref<{ path: string; body: string; key: string }>(),
  canWrite = computed(() => store.session?.permissions.includes('voucher.write')),
  canReset = computed(() => store.session?.permissions.includes('voucher.secret.reset')),
  canSeal = computed(() => store.session?.permissions.includes('voucher.seal')),
  canExport = computed(() =>
    ['file.read', 'voucher.export', 'voucher.generate'].every((p) =>
      store.session?.permissions.includes(p),
    ),
  );

/**
 * message 用途：将发行请求错误转换为界面提示。
 * 参数：e 为异常。
 * 返回值：提示字符串。
 * 约束：不显示凭证输入。
 */
function message(e: unknown) {
  return e instanceof RequestError
    ? ({
        REAUTH_REQUIRED: '请先完成身份复核',
        VERSION_CONFLICT: '卡券已变化，请刷新后重新操作',
        ILLEGAL_TRANSITION: '当前状态不允许该操作',
        EXPORT_EXPIRED: '凭证已过期或封存，不能下载',
      }[e.detail.code] ?? e.detail.code)
    : '网络异常，可重试原请求';
}

/**
 * load 用途：分页查询卡券及状态。
 * 参数：after 为可空排他游标。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功刷新列表与勾选，失败显示提示。
 */
async function load(after = '') {
  busy.value = true;
  try {
    const p = new URLSearchParams({ limit: '20', q: q.value });
    if (batch.value) p.set('batch_id', batch.value);
    if (active.value) p.set('active', active.value);
    if (redeemed.value) p.set('redeemed', redeemed.value);
    if (after) p.set('after_id', after);
    const r = await request<components['schemas']['VoucherList']>(
      '/api/v1/admin/vouchers?' + p,
    );
    items.value = r.items;
    selected.value = [];
    next.value = r.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}

/**
 * send 用途：提交发行写操作并维护固定幂等键。
 * 参数：path 当前动作路径；body 当前动作命令。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：已有待确认请求优先，成功读取权威结果，确定业务失败解除待确认状态。
 */
async function send(path: string, body: unknown) {
  pending.value ??= { path, body: JSON.stringify(body), key: crypto.randomUUID() };
  busy.value = true;
  error.value = '';
  try {
    const result = await request<components['schemas']['CredentialDelivery'] | unknown>(
      pending.value.path,
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    if (pending.value.path.endsWith('/reset-secret'))
      delivery.value = result as components['schemas']['CredentialDelivery'];
    pending.value = undefined;
    resetTarget.value = undefined;
    await load();
  } catch (e) {
    error.value = message(e);
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}

/**
 * bulk 用途：构造显式选中卡券和版本的批量命令。
 * 参数：无，使用勾选、动作和日期输入。
 * 返回值：无。
 * 约束：日期解析失败显示提示，合法命令交由 send 提交。
 */
function bulk() {
  if (!selected.value.length) return;
  try {
    const body: components['schemas']['BulkVouchers'] = {
      ids: selected.value.map((v) => v.id),
      versions: selected.value.map((v) => v.version),
      action: action.value as components['schemas']['BulkVouchers']['action'],
      reason: reason.value,
    };
    if (action.value === 'UPDATE_VALIDITY') {
      body.valid_from = new Date(from.value).toISOString();
      body.valid_until = new Date(until.value).toISOString();
    }
    send('/api/v1/admin/vouchers/actions', body);
  } catch {
    error.value = '请填写完整有效期';
  }
}

/**
 * reset 用途：提交停用未消费卡的新随机凭证请求。
 * 参数：无，使用重置对象及格式输入。
 * 返回值：无。
 * 约束：交由 send 处理错误与幂等，返回值不含密码。
 */
function reset() {
  if (resetTarget.value)
    send('/api/v1/admin/vouchers/' + resetTarget.value.id + '/reset-secret', {
      version: resetTarget.value.version,
      reason: reason.value,
      secret_length: length.value,
      secret_format: format.value,
    });
}

/**
 * exportFile 用途：下载重置后短期凭证。
 * 参数：无，使用交付文件引用。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：下载失败显示提示，不在页面回显卡密。
 */
async function exportFile() {
  if (!delivery.value) return;
  busy.value = true;
  try {
    await download('/api/v1/admin/files/' + delivery.value.export_file_id + '/download');
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page voucher-page">
    <div class="page-title">
      <h1>卡券查询与发行</h1>
      <RouterLink to="/admin/voucher-batches">卡型与批次</RouterLink
      ><RouterLink to="/admin">工作台</RouterLink
      ><RouterLink to="/admin/security">身份复核</RouterLink>
    </div>
    <form class="filters" @submit.prevent="load()">
      <input v-model="q" placeholder="卡号" maxlength="100" /><input
        v-model="batch"
        placeholder="批次 ID"
        pattern="[1-9][0-9]*"
      /><select v-model="active">
        <option value="">全部激活状态</option>
        <option value="true">已激活</option>
        <option value="false">未激活</option></select
      ><select v-model="redeemed">
        <option value="">全部消费状态</option>
        <option value="true">已消费</option>
        <option value="false">未消费</option></select
      ><button :disabled="busy || !!pending">查询</button>
    </form>
    <p role="alert">{{ error }}</p>
    <form v-if="canWrite" @submit.prevent="bulk">
      <fieldset :disabled="busy || !!pending">
        <legend>对已选 {{ selected.length }} 张卡执行操作</legend>
        <select v-model="action">
          <option value="ACTIVATE">激活</option>
          <option value="DEACTIVATE">停用</option>
          <option value="SOFT_DELETE">软删除</option>
          <option value="UPDATE_VALIDITY">修改有效期</option></select
        ><label>原因 <input v-model="reason" maxlength="200" /></label
        ><template v-if="action === 'UPDATE_VALIDITY'"
          ><label>生效时间 <input v-model="from" type="datetime-local" required /></label
          ><label>到期时间 <input v-model="until" type="datetime-local" required /></label
        ></template>
        <p>激活需要已封存；删除需要未消费且停用。修改有效期会撤销相关会话。</p>
      </fieldset>
      <button :disabled="busy || !!pending || !selected.length">确认批量操作</button>
    </form>
    <button v-if="pending" :disabled="busy" @click="send('', {})">重试原请求</button>
    <form v-if="resetTarget" @submit.prevent="reset">
      <fieldset :disabled="busy || !!pending">
        <legend>重置 {{ resetTarget.code }} 的凭证</legend>
        <p>仅支持已封存批次内未消费的停用卡；旧凭证与会话立即失效。</p>
        <label>原因 <input v-model="reason" maxlength="200" /></label
        ><label
          >密码长度
          <select v-model.number="length">
            <option v-for="n in [6, 8, 9, 10, 11, 12, 16, 24, 32]" :key="n">
              {{ n }}
            </option>
          </select></label
        ><label
          >格式
          <select v-model="format">
            <option value="MIXED">字母数字</option>
            <option value="LETTERS">字母</option>
            <option value="DIGITS">数字</option>
          </select></label
        >
      </fieldset>
      <button :disabled="busy || !!pending">确认重置</button
      ><button type="button" :disabled="!!pending" @click="resetTarget = undefined">
        取消
      </button>
    </form>
    <p v-if="delivery">
      新凭证导出有效至 {{ new Date(delivery.expires_at).toLocaleString() }}。<button
        v-if="canExport"
        :disabled="busy"
        @click="exportFile"
      >
        下载新凭证
      </button>
    </p>
    <table>
      <thead>
        <tr>
          <th>选择</th>
          <th>卡号</th>
          <th>批次</th>
          <th>凭证状态</th>
          <th>激活 / 消费</th>
          <th>有效期</th>
          <th>商品</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="v in items" :key="v.id">
          <td>
            <input
              v-if="canWrite"
              v-model="selected"
              type="checkbox"
              :value="v"
              :disabled="busy || !!pending"
              :aria-label="'选择卡券 ' + v.code"
            />
          </td>
          <td>{{ v.code }}</td>
          <td>
            <RouterLink :to="'/admin/voucher-batches/' + v.batch_id">{{
              v.batch_id
            }}</RouterLink>
          </td>
          <td>{{ v.credential_state }}</td>
          <td>
            {{ v.active ? '已激活' : '未激活' }} / {{ v.redeemed ? '已消费' : '未消费' }}
          </td>
          <td>{{ new Date(v.valid_until).toLocaleString() }}</td>
          <td>{{ v.product_ids.join('、') }}</td>
          <td>
            <button
              v-if="canReset && !v.active && !v.redeemed"
              :disabled="busy || !!pending"
              @click="resetTarget = v"
            >
              重置密码</button
            ><button
              v-if="canSeal && v.credential_state === 'EXPORTABLE' && !v.active"
              :disabled="busy || !!pending"
              @click="
                send('/api/v1/admin/vouchers/' + v.id + '/seal', {
                  version: v.version,
                  reason,
                })
              "
            >
              封存单卡
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="!items.length">暂无卡券</p>
    <button v-if="next" :disabled="busy || !!pending" @click="load(next)">下一页</button>
  </main>
</template>
