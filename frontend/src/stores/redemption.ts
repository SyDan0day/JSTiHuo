import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { components } from '../api/generated/schema';
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
    date = ref(''),
    note = ref(''),
    pending = ref<{ body: string; key: string }>();
  /**
   * clear 用途：清理当前兑换流程的内存信息。
   * 参数：无，使用 store 内部状态。
   * 返回值：无。
   * 约束：清除选品、地址、备注、鉴真和待确认请求，不修改服务端订单。
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
    date.value = '';
    note.value = '';
    pending.value = undefined;
  }
  return { verification, options, selected, address, date, note, pending, clear };
});
