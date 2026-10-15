<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useSession } from '../../../stores/session';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
const session = useSession(),
  items = ref<components['schemas']['Carrier'][]>([]),
  next = ref(''),
  busy = ref(false),
  error = ref('');
const draft = ref<components['schemas']['CarrierInput']>(),
  editing = ref<components['schemas']['Carrier']>();
const pending = ref<{ path: string; method: string; body: string; key: string }>();
/**
 * load 用途：分页读取真实承运商目录。
 * 参数：more 表示加载下一页。
 * 返回值：完成操作的 Promise，失败显示错误。
 */
async function load(more = false) {
  busy.value = true;
  error.value = '';
  try {
    const page = await request<components['schemas']['CarrierList']>(
      '/api/v1/admin/carriers?limit=20' + (more ? '&after_id=' + next.value : ''),
    );
    items.value = more ? [...items.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败';
  } finally {
    busy.value = false;
  }
}
/**
 * edit 用途：准备新增或已有目录的完整编辑快照。
 * 参数：item 可选的已有承运商。
 * 返回值：无，不写入数据库。
 */
function edit(item?: components['schemas']['Carrier']) {
  editing.value = item;
  draft.value = item
    ? {
        code: item.code,
        name: item.name,
        alias: item.alias,
        provider: item.provider,
        kdniao_code: item.kdniao_code,
        kuaidi100_code: item.kuaidi100_code,
        wdt_code: item.wdt_code,
        enabled: item.enabled,
        secret_ref: item.secret_ref,
      }
    : {
        code: '',
        name: '',
        alias: '',
        provider: 'MANUAL',
        kdniao_code: '',
        kuaidi100_code: '',
        wdt_code: '',
        enabled: true,
        secret_ref: '',
      };
}
/**
 * save 用途：按版本和幂等键保存完整承运商目录。
 * 参数：无，使用当前编辑快照。
 * 返回值：完成操作的 Promise，未知结果保留原请求供重试。
 */
async function save() {
  pending.value ??= {
    path: '/api/v1/admin/carriers' + (editing.value ? '/' + editing.value.id : ''),
    method: editing.value ? 'PATCH' : 'POST',
    body: JSON.stringify({
      ...draft.value,
      ...(editing.value ? { version: editing.value.version } : {}),
    }),
    key: crypto.randomUUID(),
  };
  busy.value = true;
  error.value = '';
  try {
    await request(
      pending.value.path,
      { method: pending.value.method, body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
    draft.value = undefined;
    await load();
  } catch (e) {
    error.value =
      e instanceof RequestError ? e.detail.code : '网络结果未确认，请重试原请求';
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">管理工作台</RouterLink>
    <h1>承运商目录</h1>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy || !!pending" @click="load()">刷新</button>
    <button
      v-if="session.session?.permissions.includes('carrier.write')"
      :disabled="busy || !!pending"
      @click="edit()"
    >
      新增承运商
    </button>
    <table>
      <thead>
        <tr>
          <th>编码 / 名称</th>
          <th>模式</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.id">
          <td>{{ item.code }} / {{ item.name }}</td>
          <td>{{ item.provider }}</td>
          <td>{{ item.enabled ? '启用' : '停用' }}</td>
          <td>
            <button
              v-if="session.session?.permissions.includes('carrier.write')"
              :disabled="busy || !!pending"
              @click="edit(item)"
            >
              编辑
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <button v-if="next" :disabled="busy || !!pending" @click="load(true)">更多</button>
    <form v-if="draft" @submit.prevent="save">
      <h2>{{ editing ? '编辑' : '新增' }}承运商</h2>
      <fieldset :disabled="busy || !!pending">
        <label for="carrier-code">编码</label
        ><input
          id="carrier-code"
          v-model="draft.code"
          required
          maxlength="40"
          pattern="[A-Za-z0-9_-]+"
        />
        <label for="carrier-name">名称</label
        ><input id="carrier-name" v-model="draft.name" required maxlength="80" />
        <label for="carrier-alias">别名</label
        ><input id="carrier-alias" v-model="draft.alias" maxlength="80" />
        <label for="carrier-provider">模式</label
        ><select id="carrier-provider" v-model="draft.provider">
          <option>MANUAL</option>
          <option>MOCK</option>
          <option>KDNIAO</option>
          <option>KUAIDI100</option>
        </select>
        <p>MOCK 产生模拟轨迹。真实供应商目前只维护映射目录，尚未接入查询与面单。</p>
        <label for="carrier-kdniao">快递鸟编码</label
        ><input id="carrier-kdniao" v-model="draft.kdniao_code" maxlength="40" />
        <label for="carrier-kuaidi">快递100编码</label
        ><input id="carrier-kuaidi" v-model="draft.kuaidi100_code" maxlength="40" />
        <label for="carrier-wdt">旺店通编码</label
        ><input id="carrier-wdt" v-model="draft.wdt_code" maxlength="40" />
        <p>V1 尚无已配置的 LIVE 密钥引用，不能在后台录入供应商凭据；仅维护非敏感平台编码。</p>
        <label><input v-model="draft.enabled" type="checkbox" />启用</label
        ><button>保存</button>
      </fieldset>
      <button v-if="pending" type="button" :disabled="busy" @click="save">
        重试原请求
      </button>
    </form>
  </main>
</template>
