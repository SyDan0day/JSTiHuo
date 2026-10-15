<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { request } from '../../../api/client';
import { useCommand } from '../../../api/command';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
const session = useSession(),
  { busy, error, pending, send } = useCommand();
const config = ref<components['schemas']['Config']>(),
  blackouts = ref(''),
  loadError = ref(''),
  loading = ref(false),
  saved = ref(false);
/**
 * load 用途：读取固定配置与版本，不读取供应商凭据。
 * 参数：无。
 * 返回值：加载 Promise，失败保留刷新入口。
 */
async function load() {
  if (busy.value || pending.value || loading.value) return;
  loading.value = true;
  loadError.value = '';
  saved.value = false;
  try {
    config.value = await request('/api/v1/admin/config');
    blackouts.value = config.value?.blackout_dates.join('\n') ?? '';
  } catch {
    loadError.value = '配置读取失败或没有权限。';
  } finally {
    loading.value = false;
  }
}
/**
 * save 用途：提交完整版本配置，容量不足时由服务端整笔回滚。
 * 参数：无，使用固定表单与逐行禁配日期。
 * 返回值：保存 Promise，不允许将未接入的 LIVE 配置为成功。
 */
async function save() {
  if (!config.value) return;
  const body = { ...config.value, blackout_dates: blackouts.value.split(/\s+/).filter(Boolean) };
  if (!body.default_carrier_id) delete body.default_carrier_id;
  const result = await send<components['schemas']['Config']>('/api/v1/admin/config', body, 'PUT');
  if (result) {
    config.value = result;
    blackouts.value = result.blackout_dates.join('\n');
    saved.value = true;
  }
}
onMounted(load);
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>预约与通知设置</h1>
    <p>容量变更只调整未来日历；不能低于已有占用，不改变既有订单日期。黑名单阻止新预约。</p>
    <p v-if="error || loadError" role="alert">{{ error || loadError }}</p>
    <p v-if="saved" role="status">配置已保存，未来日历已同步。</p>
    <button :disabled="busy || loading || !!pending" @click="load">重新读取配置</button>
    <form v-if="config" @submit.prevent="save">
      <fieldset
        :disabled="busy || !!pending || !session.session?.permissions.includes('config.write')"
      >
        <legend>当前版本 {{ config.version }}</legend>
        <label for="settings-lead">最少提前天数</label
        ><input
          id="settings-lead"
          v-model.number="config.min_lead_days"
          type="number"
          min="0"
          max="30"
          required
        />
        <label for="settings-horizon">可预约最远天数</label
        ><input
          id="settings-horizon"
          v-model.number="config.horizon_days"
          type="number"
          min="1"
          max="30"
          required
        />
        <label for="settings-cutoff">上海时间截止小时</label
        ><input
          id="settings-cutoff"
          v-model.number="config.cutoff_hour"
          type="number"
          min="0"
          max="23"
          required
        />
        <label for="settings-capacity">每日容量</label
        ><input
          id="settings-capacity"
          v-model.number="config.daily_capacity"
          type="number"
          min="0"
          max="100000"
          required
        />
        <label for="settings-blackouts">暂停预约日期（每行 YYYY-MM-DD，最多 366 天）</label
        ><textarea id="settings-blackouts" v-model="blackouts" rows="6" />
        <label for="settings-notification">业务通知</label
        ><select id="settings-notification" v-model="config.notification_mode">
          <option value="DISABLED">关闭，记录 SUPPRESSED</option>
          <option value="MOCK">MOCK 模拟，不发送真实消息</option>
        </select>
        <p>LIVE 供应商尚未接入。密钥由部署文件管理，不能在此录入。</p>
        <label for="settings-carrier">默认承运商 ID（可留空）</label
        ><input id="settings-carrier" v-model="config.default_carrier_id" pattern="[1-9][0-9]*" />
        <RouterLink
          v-if="session.session?.permissions.includes('carrier.read')"
          to="/admin/carriers"
          >查看承运商</RouterLink
        >
        <button>保存固定配置</button>
      </fieldset>
      <button v-if="pending" type="button" :disabled="busy" @click="save">重试原配置请求</button>
    </form>
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
