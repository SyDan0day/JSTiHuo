<script setup lang="ts">
import { ref, watch } from 'vue';
import { request } from '../api/client';
import type { components } from '../api/generated/schema';
type Address = components['schemas']['Address'];
type Region = components['schemas']['Region'];
const props = defineProps<{ modelValue: Address; disabled?: boolean }>(),
  emit = defineEmits<{ (e: 'update:modelValue', value: Address): void }>(),
  provinces = ref<Region[]>([]),
  cities = ref<Region[]>([]),
  districts = ref<Region[]>([]),
  error = ref('');
/**
 * update 用途：发送完整地址字段变化。
 * 参数：key 为地址字段，value 为新文本。
 * 返回值：无。
 * 约束：选择父级时清空下级，不直接修改父组件 props。
 */
function update(key: keyof Address, value: string) {
  const next = { ...props.modelValue, [key]: value };
  if (key === 'province_code') {
    next.city_code = '';
    next.district_code = '';
  }
  if (key === 'city_code') next.district_code = '';
  emit('update:modelValue', next);
}
/**
 * regions 用途：读取部署版本地区。
 * 参数：parent 为可空六位父级代码。
 * 返回值：地区数组 Promise。
 * 错误：网络或 API 错误向调用方传播。
 */
async function regions(parent = '') {
  return (
    await request<components['schemas']['RegionList']>(
      '/api/v1/regions' + (parent ? '?parent_code=' + parent : ''),
    )
  ).items;
}
regions()
  .then((v) => (provinces.value = v))
  .catch(() => (error.value = '省份读取失败，请刷新'));
watch(
  () => props.modelValue.province_code,
  async (parent, _, cleanup) => {
    let canceled = false;
    cleanup(() => (canceled = true));
    cities.value = [];
    districts.value = [];
    if (!parent) return;
    try {
      const v = await regions(parent);
      if (!canceled) cities.value = v;
    } catch {
      if (!canceled) error.value = '城市读取失败';
    }
  },
  { immediate: true },
);
watch(
  () => props.modelValue.city_code,
  async (parent, _, cleanup) => {
    let canceled = false;
    cleanup(() => (canceled = true));
    districts.value = [];
    if (!parent) return;
    try {
      const v = await regions(parent);
      if (!canceled) districts.value = v;
    } catch {
      if (!canceled) error.value = '区县读取失败';
    }
  },
  { immediate: true },
);
</script>
<template>
  <fieldset :disabled="disabled">
    <legend>完整替换收货地址</legend>
    <label for="address-name">姓名</label
    ><input
      id="address-name"
      :value="modelValue.name"
      @input="update('name', ($event.target as HTMLInputElement).value)"
      required
      maxlength="40"
      autocomplete="shipping name"
    /><label for="address-phone">手机</label
    ><input
      id="address-phone"
      :value="modelValue.phone"
      @input="update('phone', ($event.target as HTMLInputElement).value)"
      required
      type="tel"
      pattern="1[3-9][0-9]{9}"
      maxlength="11"
      autocomplete="shipping tel"
    /><label for="address-province">省份</label
    ><select
      id="address-province"
      :value="modelValue.province_code"
      @change="update('province_code', ($event.target as HTMLSelectElement).value)"
      required
    >
      <option disabled value="">请选择省份</option>
      <option v-for="r in provinces" :key="r.code" :value="r.code">
        {{ r.name }}
      </option></select
    ><label for="address-city">城市</label
    ><select
      id="address-city"
      :value="modelValue.city_code"
      @change="update('city_code', ($event.target as HTMLSelectElement).value)"
      required
    >
      <option disabled value="">请选择城市</option>
      <option v-for="r in cities" :key="r.code" :value="r.code">
        {{ r.name }}
      </option></select
    ><label for="address-district">区县</label
    ><select
      id="address-district"
      :value="modelValue.district_code"
      @change="update('district_code', ($event.target as HTMLSelectElement).value)"
      required
    >
      <option disabled value="">请选择区县</option>
      <option v-for="r in districts" :key="r.code" :value="r.code">
        {{ r.name }}
      </option></select
    ><label for="address-detail">详细地址</label
    ><textarea
      id="address-detail"
      :value="modelValue.detail"
      @input="update('detail', ($event.target as HTMLTextAreaElement).value)"
      required
      minlength="5"
      maxlength="200"
      autocomplete="shipping street-address"
    />
    <p v-if="error" role="alert">{{ error }}</p>
  </fieldset>
</template>
<style scoped>
select {
  font: inherit;
  min-height: 44px;
  border: 1px solid #aaa;
  border-radius: 4px;
  padding: 8px;
}
select:focus-visible {
  outline: 3px solid #2563eb;
}
</style>
