import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { components } from '../api/generated/schema';
/**
 * useRedemption 用途：保存单次兑换的内存状态，供选品和收货页面共享。
 * 参数：由 Pinia 提供当前应用实例。
 * 返回值：兑换状态及清理方法，不持久化收货信息。
 */
export const useRedemption = defineStore('redemption', () => {
  const verification = ref<components['schemas']['Verification']>(),
    options = ref<components['schemas']['Options']>(),
    selected = ref<string[]>([]),
    address = ref<components['schemas']['Address']>({
      name: '',
      phone: '',
      province_code: '',
      city_code: '',
      district_code: '',
      detail: '',
    }),
    regionLabel = ref(''),
    date = ref(''),
    note = ref(''),
    pending = ref<{ body: string; key: string }>();
  /**
   * clear 用途：清理当前兑换流程的内存信息。
   * 参数：无，使用 store 内部状态。
   * 返回值：无。
   * 约束：清除选品、地址及地区名称、备注、鉴真和待确认请求。
   */
  function clear() {
    verification.value = undefined;
    options.value = undefined;
    selected.value = [];
    address.value = {
      name: '',
      phone: '',
      province_code: '',
      city_code: '',
      district_code: '',
      detail: '',
    };
    regionLabel.value = '';
    date.value = '';
    note.value = '';
    pending.value = undefined;
  }
  return { verification, options, selected, address, regionLabel, date, note, pending, clear };
});
