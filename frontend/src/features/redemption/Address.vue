<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError } from '../../api/client';
import { useRedemption } from '../../stores/redemption';
import type { components } from '../../api/generated/schema';
const store = useRedemption(),
  router = useRouter(),
  provinces = ref<components['schemas']['Region'][]>([]),
  cities = ref<components['schemas']['Region'][]>([]),
  districts = ref<components['schemas']['Region'][]>([]),
  days = ref<components['schemas']['CalendarDay'][]>([]),
  error = ref(''),
  busy = ref(true);

/**
 * regions 用途：读取可信父级下的地区选项。
 * 参数：parent 为可空六位父级代码。
 * 返回值：地区数组 Promise。
 * 约束：失败向调用者抛错，无业务写入。
 */
async function regions(parent = '') {
  return (
    await request<components['schemas']['RegionList']>(
      '/api/v1/regions' + (parent ? '?parent_code=' + parent : ''),
    )
  ).items;
}

/**
 * province 用途：响应省份选择并刷新城市。
 * 参数：无，使用省级输入。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：清空下级选择，加载失败显示错误。
 */
async function province() {
  store.address.city_code = '';
  store.address.district_code = '';
  districts.value = [];
  try {
    cities.value = store.address.province_code
      ? await regions(store.address.province_code)
      : [];
  } catch {
    error.value = '地区读取失败';
  }
}

/**
 * city 用途：响应城市选择并刷新区县。
 * 参数：无，使用市级输入。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：清空区县，加载失败显示错误。
 */
async function city() {
  store.address.district_code = '';
  try {
    districts.value = store.address.city_code
      ? await regions(store.address.city_code)
      : [];
  } catch {
    error.value = '地区读取失败';
  }
}

/**
 * load 用途：恢复选品并读取地区和预约建议。
 * 参数：无，使用当前会话及兑换 store。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：选择不完整跳回选品，查询失败显示重试入口。
 * 约束：不占日额度。
 */
async function load() {
  busy.value = true;
  error.value = '';
  try {
    if (!store.options) {
      store.options = await request('/api/v1/redemption/options');
    }
    if (store.selected.length !== store.options?.selection_count) {
      await router.replace('/redeem/select');
      return;
    }
    provinces.value = await regions();
    if (store.address.province_code)
      cities.value = await regions(store.address.province_code);
    if (store.address.city_code) districts.value = await regions(store.address.city_code);
    days.value = (
      await request<components['schemas']['Calendar']>('/api/v1/redemption/calendar')
    ).items;
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}
onMounted(load);
</script>
<template>
  <main class="auth">
    <RouterLink to="/redeem/select">返回选品</RouterLink>
    <h1>收货与预约</h1>
    <p role="alert">{{ error }}</p>
    <button v-if="error" @click="load">重新读取</button>
    <form @submit.prevent="router.push('/redeem/confirm')">
      <fieldset :disabled="busy || !!store.pending">
        <legend>收货信息</legend>
        <label for="recipient">姓名</label
        ><input
          id="recipient"
          v-model="store.address.name"
          required
          maxlength="40"
          autocomplete="shipping name"
        /><label for="phone">手机</label
        ><input
          id="phone"
          v-model="store.address.phone"
          type="tel"
          required
          pattern="1[3-9][0-9]{9}"
          maxlength="11"
          autocomplete="shipping tel"
        /><label for="province">省份</label
        ><select
          id="province"
          v-model="store.address.province_code"
          required
          @change="province"
        >
          <option disabled value="">请选择省份</option>
          <option v-for="r in provinces" :key="r.code" :value="r.code">
            {{ r.name }}
          </option></select
        ><label for="city">城市</label
        ><select id="city" v-model="store.address.city_code" required @change="city">
          <option disabled value="">请选择城市</option>
          <option v-for="r in cities" :key="r.code" :value="r.code">
            {{ r.name }}
          </option></select
        ><label for="district">区县</label
        ><select id="district" v-model="store.address.district_code" required>
          <option disabled value="">请选择区县</option>
          <option v-for="r in districts" :key="r.code" :value="r.code">
            {{ r.name }}
          </option></select
        ><label for="detail">详细地址</label
        ><textarea
          id="detail"
          v-model="store.address.detail"
          required
          minlength="5"
          maxlength="200"
          autocomplete="shipping street-address"
        /><label for="ship-date">预约发货日</label
        ><select id="ship-date" v-model="store.date" required>
          <option disabled value="">请选择发货日期</option>
          <option
            v-for="d in days"
            :key="d.date"
            :value="d.date"
            :disabled="!d.available"
          >
            {{ d.date }} · {{ d.available ? '余量 ' + d.remaining : '不可预约' }}
          </option></select
        ><label for="customer-note">备注（可选）</label
        ><textarea id="customer-note" v-model="store.note" maxlength="200" />
      </fieldset>
      <button :disabled="busy || !!store.pending">核对并继续</button>
    </form>
  </main>
</template>
