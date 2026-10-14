<script setup lang="ts">
import { ref, onUnmounted } from 'vue';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
const code = ref(''),
  secret = ref(''),
  productIds = ref(''),
  reason = ref(''),
  error = ref(''),
  busy = ref(false);
const result = ref<components['schemas']['Order']>();
const pending = ref<{ body: string; key: string }>();
/**
 * submit 用途：提交现场卡密与商品集合完成到店核销。
 * 参数：无，读取当前输入；各商品固定一件。
 * 返回值：完成操作的 Promise，失败显示错误，未知结果保留原请求仅供重试。
 * 约束：输入卡密立即清除，请求仅在内存保留，离开页面销毁。
 */
async function submit() {
  pending.value ??= {
    key: crypto.randomUUID(),
    body: JSON.stringify({
      code: code.value,
      secret: secret.value,
      items: productIds.value
        .split(/[\s,，]+/)
        .filter(Boolean)
        .map((product_id) => ({ product_id, quantity: 1 })),
      reason: reason.value,
    }),
  };
  secret.value = '';
  busy.value = true;
  error.value = '';
  try {
    result.value = await request(
      '/api/v1/admin/pickups',
      { method: 'POST', body: pending.value.body },
      pending.value.key,
    );
    pending.value = undefined;
  } catch (e) {
    error.value =
      e instanceof RequestError ? e.detail.code : '网络结果未确认，请重试原请求';
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}
onUnmounted(() => {
  secret.value = '';
  pending.value = undefined;
});
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">管理工作台</RouterLink>
    <h1>到店核销</h1>
    <p>现场确认卡券和商品，每个商品一件。成功后订单直接完成，不占配送名额。</p>
    <p role="alert">{{ error }}</p>
    <p v-if="result" role="status">
      到店核销成功：<RouterLink :to="'/admin/orders/' + result.id">{{
        result.order_no
      }}</RouterLink>
    </p>
    <form v-else @submit.prevent="submit">
      <fieldset :disabled="busy || !!pending">
        <label for="pickup-code">卡号</label
        ><input id="pickup-code" v-model="code" required minlength="6" maxlength="32" />
        <label for="pickup-secret">原卡密</label
        ><input
          id="pickup-secret"
          v-model="secret"
          type="password"
          autocomplete="off"
          required
          minlength="6"
          maxlength="32"
        />
        <label for="pickup-items">商品 ID（逗号分隔）</label
        ><input id="pickup-items" v-model="productIds" required />
        <p>商品 ID 可在商品管理查看；商品集合必须符合该卡权益，服务端会再次校验。</p>
        <label for="pickup-reason">核销原因</label
        ><textarea id="pickup-reason" v-model="reason" required maxlength="200" />
        <button>确认到店核销</button>
      </fieldset>
      <button v-if="pending" type="button" :disabled="busy" @click="submit">
        重试原请求
      </button>
    </form>
  </main>
</template>
