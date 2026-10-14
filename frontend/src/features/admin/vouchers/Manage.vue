<script setup lang="ts">
import './vouchers.css';
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError } from '../../../api/client';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
type Type = components['schemas']['VoucherType'];
type Batch = components['schemas']['Batch'];
type Product = components['schemas']['Product'];
const router = useRouter(),
  store = useSession(),
  types = ref<Type[]>([]),
  batches = ref<Batch[]>([]),
  products = ref<Product[]>([]),
  next = ref(''),
  typeNext = ref(''),
  productNext = ref(''),
  busy = ref(false),
  error = ref(''),
  q = ref(''),
  status = ref('');
const canGenerate = computed(() =>
  store.session?.permissions.includes('voucher.generate'),
);
const typeForm = ref<components['schemas']['TypeInput']>({
  name: '',
  selection_mode: 'ONE_OF',
  selection_count: 1,
});
const form = ref<components['schemas']['BatchInput']>({
  type_id: '',
  label: '',
  product_ids: [],
  quantity: 1,
  code_prefix: 'JT',
  code_length: 16,
  code_mode: 'RANDOM',
  secret_length: 12,
  secret_format: 'MIXED',
  valid_from: '',
  valid_until: '',
  activate_on_seal: false,
});
const from = ref(''),
  until = ref(''),
  selectedType = computed(() => types.value.find((t) => t.id === form.value.type_id));
const pending = ref<{ path: string; body: string; key: string }>();

/**
 * message 用途：转换卡型与建批次错误提示。
 * 参数：e 为异常。
 * 返回值：提示字符串。
 * 约束：不输出输入凭证。
 */
function message(e: unknown) {
  return e instanceof RequestError
    ? ({
        SELECTION_INVALID: '商品集合必须满足卡型的可选数量',
        VALIDATION_FAILED: '请检查生成规则和有效期',
        VERSION_CONFLICT: '数据已变化，请刷新',
        FORBIDDEN: '没有操作权限',
        IDEMPOTENCY_CONFLICT: '请求标识冲突',
      }[e.detail.code] ?? e.detail.code)
    : '网络异常，可重试原请求';
}

/**
 * load 用途：分页查询批次。
 * 参数：after 为可空游标。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：更新列表或错误状态，不启动生成。
 */
async function load(after = '') {
  busy.value = true;
  try {
    const p = new URLSearchParams({ limit: '20', q: q.value, status: status.value });
    if (after) p.set('after_id', after);
    const r = await request<components['schemas']['BatchList']>(
      '/api/v1/admin/voucher-batches?' + p,
    );
    batches.value = r.items;
    next.value = r.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}

/**
 * loadTypes 用途：读取卡型版本并按游标追加。
 * 参数：after 为可空游标。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：更新可选规则或错误提示。
 */
async function loadTypes(after = '') {
  try {
    const r = await request<components['schemas']['VoucherTypeList']>(
      '/api/v1/admin/voucher-types?limit=100' + (after ? '&after_id=' + after : ''),
    );
    types.value = after ? [...types.value, ...r.items] : r.items;
    typeNext.value = r.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  }
}

/**
 * loadProducts 用途：在具备权限时读取权益商品。
 * 参数：after 为可空游标。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：更新候选集合或错误提示，无商品写入。
 */
async function loadProducts(after = '') {
  if (!store.session?.permissions.includes('product.read')) return;
  try {
    const r = await request<components['schemas']['ProductList']>(
      '/api/v1/admin/products?limit=100' + (after ? '&after_id=' + after : ''),
    );
    products.value = after ? [...products.value, ...r.items] : r.items;
    productNext.value = r.next_after_id ?? '';
  } catch (e) {
    error.value = message(e);
  }
}

/**
 * submit 用途：保存卡型版本或冻结批次草稿。
 * 参数：kind 为 type/batch。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：结果不确定保留固定请求，成功导航至批次或刷新卡型，校验失败显示提示。
 */
async function submit(kind: 'type' | 'batch') {
  if (!pending.value) {
    try {
      const body =
        kind === 'type'
          ? {
              ...typeForm.value,
              selection_count:
                typeForm.value.selection_mode === 'ONE_OF'
                  ? 1
                  : typeForm.value.selection_count,
            }
          : {
              ...form.value,
              valid_from: new Date(from.value).toISOString(),
              valid_until: new Date(until.value).toISOString(),
            };
      if (kind === 'batch' && form.value.code_mode === 'RANDOM')
        delete (body as components['schemas']['BatchInput']).start_serial;
      pending.value = {
        path: '/api/v1/admin/' + (kind === 'type' ? 'voucher-types' : 'voucher-batches'),
        body: JSON.stringify(body),
        key: crypto.randomUUID(),
      };
    } catch {
      error.value = '请填写完整有效期';
      return;
    }
  }
  busy.value = true;
  error.value = '';
  try {
    const v = await request<Type | Batch>(
      pending.value.path,
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    const batch = pending.value.path.endsWith('voucher-batches');
    pending.value = undefined;
    if (batch) {
      await router.push('/admin/voucher-batches/' + v.id);
    } else {
      typeForm.value.name = '';
      await loadTypes();
    }
  } catch (e) {
    error.value = message(e);
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}
onMounted(async () => {
  await Promise.all([load(), loadTypes(), loadProducts()]);
});
</script>
<template>
  <main class="admin-page voucher-page">
    <div class="page-title">
      <h1>卡型与批次</h1>
      <RouterLink to="/admin">工作台</RouterLink
      ><RouterLink to="/admin/vouchers">卡券查询</RouterLink>
    </div>
    <p v-if="error" role="alert">{{ error }}</p>
    <p>卡型保存后不可修改。相同名称再次创建会形成新版本；已有批次权益保持不变。</p>
    <form v-if="canGenerate" @submit.prevent="submit('type')">
      <fieldset :disabled="busy || !!pending">
        <legend>新增卡型版本</legend>
        <label>名称 <input v-model="typeForm.name" required maxlength="80" /></label
        ><label
          >选品规则
          <select v-model="typeForm.selection_mode">
            <option value="ONE_OF">任选一种</option>
            <option value="N_OF">任选 N 种</option>
          </select></label
        ><label v-if="typeForm.selection_mode === 'N_OF'"
          >种数
          <input
            v-model.number="typeForm.selection_count"
            type="number"
            min="2"
            max="99"
            required
        /></label>
      </fieldset>
      <button :disabled="busy || !!pending">保存卡型</button>
    </form>
    <table>
      <thead>
        <tr>
          <th>卡型</th>
          <th>版本</th>
          <th>规则</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="t in types" :key="t.id">
          <td>{{ t.name }}</td>
          <td>{{ t.revision }}</td>
          <td>任选 {{ t.selection_count }} 种</td>
        </tr>
      </tbody>
    </table>
    <button v-if="typeNext" @click="loadTypes(typeNext)">加载更多卡型</button>
    <form v-if="canGenerate" @submit.prevent="submit('batch')">
      <fieldset :disabled="busy || !!pending">
        <legend>创建卡券批次</legend>
        <label
          >卡型
          <select v-model="form.type_id" required>
            <option value="" disabled>请选择</option>
            <option v-for="t in types.filter((t) => t.enabled)" :key="t.id" :value="t.id">
              {{ t.name }} · v{{ t.revision }} · 选{{ t.selection_count }}种
            </option>
          </select></label
        ><label>批次名称 <input v-model="form.label" required maxlength="100" /></label>
        <fieldset>
          <legend>
            权益商品（已选 {{ form.product_ids.length }}，至少
            {{ selectedType?.selection_count ?? 1 }} 种）
          </legend>
          <label v-for="p in products" :key="p.id"
            ><input v-model="form.product_ids" type="checkbox" :value="p.id" />{{
              p.name
            }}
            · {{ p.code }} · {{ p.status }}</label
          ><button v-if="productNext" type="button" @click="loadProducts(productNext)">
            加载更多商品
          </button>
          <p v-if="!store.session?.permissions.includes('product.read')">
            需要商品读取权限才能选择商品。
          </p>
        </fieldset>
        <label
          >张数
          <input
            v-model.number="form.quantity"
            type="number"
            min="1"
            max="1000"
            required /></label
        ><label
          >卡号前缀
          <input v-model="form.code_prefix" maxlength="4" pattern="[A-Za-z0-9]*" /></label
        ><label
          >卡号总长
          <input
            v-model.number="form.code_length"
            type="number"
            min="12"
            max="24"
            required /></label
        ><label
          >卡号模式
          <select v-model="form.code_mode">
            <option value="RANDOM">安全随机</option>
            <option value="SEQUENTIAL">顺序编号</option>
          </select></label
        ><label v-if="form.code_mode === 'SEQUENTIAL'"
          >起始序号
          <input
            v-model="form.start_serial"
            required
            pattern="[0-9]+"
            maxlength="24" /></label
        ><label
          >密码长度
          <select v-model.number="form.secret_length">
            <option v-for="n in [6, 8, 9, 10, 11, 12, 16, 24, 32]" :key="n" :value="n">
              {{ n }}
            </option>
          </select></label
        ><label
          >密码格式
          <select v-model="form.secret_format">
            <option value="MIXED">字母数字</option>
            <option value="LETTERS">字母</option>
            <option value="DIGITS">数字</option>
          </select></label
        ><label>生效时间 <input v-model="from" type="datetime-local" required /></label
        ><label>到期时间 <input v-model="until" type="datetime-local" required /></label
        ><label
          ><input v-model="form.activate_on_seal" type="checkbox" />封存时自动激活</label
        >
      </fieldset>
      <button :disabled="busy || !!pending">保存草稿</button>
    </form>
    <button v-if="pending" :disabled="busy" @click="submit('batch')">重试原请求</button>
    <h2>批次列表</h2>
    <form class="filters" @submit.prevent="load()">
      <input v-model="q" placeholder="批次号或名称" maxlength="100" /><select
        v-model="status"
      >
        <option value="">全部状态</option>
        <option
          v-for="s in ['DRAFT', 'GENERATING', 'GENERATED', 'FAILED', 'SEALED']"
          :key="s"
        >
          {{ s }}
        </option></select
      ><button :disabled="busy">查询</button>
    </form>
    <table>
      <thead>
        <tr>
          <th>批次</th>
          <th>名称</th>
          <th>状态</th>
          <th>生成数量</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="b in batches" :key="b.id">
          <td>
            <RouterLink :to="'/admin/voucher-batches/' + b.id">{{
              b.batch_no
            }}</RouterLink>
          </td>
          <td>{{ b.label }}</td>
          <td>{{ b.state }}</td>
          <td>{{ b.generated_count }} / {{ b.quantity }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="!batches.length">暂无批次</p>
    <button v-if="next" :disabled="busy" @click="load(next)">下一页</button>
  </main>
</template>
