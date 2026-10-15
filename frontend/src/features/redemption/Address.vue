<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import AddressFields from '../../components/AddressFields.vue';
import { request } from '../../api/client';
import { flowError } from '../../api/flow-error';
import { useRedemption } from '../../stores/redemption';
import type { components } from '../../api/generated/schema';
const store = useRedemption(),
  router = useRouter();
const days = ref<components['schemas']['CalendarDay'][]>([]);
const error = ref(''),
  busy = ref(true),
  ready = ref(false);

/**
 * load 用途：恢复授权选品并读取可预约日期。
 * 参数：无，使用当前会话及内存选品。
 * 返回值：加载 Promise；失败不开放提交，日期建议不预占库存或名额。
 */
async function load() {
  busy.value = true;
  ready.value = false;
  error.value = '';
  try {
    if (!store.options) store.options = await request('/api/v1/redemption/options');
    if (store.selected.length !== store.options?.selection_count) {
      await router.replace('/redeem/select');
      return;
    }
    days.value = (
      await request<components['schemas']['Calendar']>('/api/v1/redemption/calendar')
    ).items;
    if (!days.value.some((day) => day.date === store.date && day.available)) store.date = '';
    ready.value = true;
  } catch (e) {
    error.value = flowError(e);
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>

<template>
  <section class="flow-page auth">
    <RouterLink to="/redeem/select">返回选品</RouterLink>
    <h1>收货与预约</h1>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="busy" role="status">正在读取预约日期…</p>
    <button v-if="error" :disabled="busy || !!store.pending" @click="load">重新读取</button>
    <form @submit.prevent="router.push('/redeem/confirm')">
      <AddressFields
        v-model="store.address"
        :disabled="busy || !ready || !!store.pending"
        @region-label="store.regionLabel = $event"
      />
      <fieldset :disabled="busy || !ready || !!store.pending">
        <legend>预约与备注</legend>
        <label for="ship-date">预约发货日</label
        ><select id="ship-date" v-model="store.date" required>
          <option disabled value="">请选择发货日期</option>
          <option v-for="day in days" :key="day.date" :value="day.date" :disabled="!day.available">
            {{ day.date }} · {{ day.available ? '余量 ' + day.remaining : '不可预约' }}
          </option>
        </select>
        <label for="customer-note">备注（可选）</label
        ><textarea id="customer-note" v-model="store.note" maxlength="200" />
      </fieldset>
      <button :disabled="busy || !ready || !store.regionLabel || !!store.pending">
        核对并继续
      </button>
    </form>
    <RouterLink v-if="store.pending" to="/redeem/confirm">返回确认原请求结果</RouterLink>
  </section>
</template>
