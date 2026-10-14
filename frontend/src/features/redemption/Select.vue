<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError } from '../../api/client';
import { useRedemption } from '../../stores/redemption';
import type { components } from '../../api/generated/schema';
const store = useRedemption(),
  router = useRouter(),
  error = ref(''),
  busy = ref(true);

/**
 * load 用途：读取授权权益选品集合。
 * 参数：无，使用当前 Cookie 和 store。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，成功移除已不在集合中的选择。
 */
async function load() {
  busy.value = true;
  try {
    store.options = await request<components['schemas']['Options']>(
      '/api/v1/redemption/options',
    );
    store.selected = store.selected.filter((id) =>
      store.options?.products.some((p) => p.id === id),
    );
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}

/**
 * choose 用途：更新本地商品选择。
 * 参数：id 为授权商品标识。
 * 返回值：无。
 * 约束：ONE_OF 替换选择，N_OF 切换勾选，不预占库存。
 */
function choose(id: string) {
  if (store.options?.selection_mode === 'ONE_OF') store.selected = [id];
  else if (store.selected.includes(id))
    store.selected = store.selected.filter((v) => v !== id);
  else store.selected.push(id);
}
onMounted(load);
</script>
<template>
  <main>
    <RouterLink to="/verify">重新鉴真</RouterLink>
    <h1>选择提货商品</h1>
    <p role="alert">{{ error }}</p>
    <button v-if="error" @click="load">重新读取</button
    ><template v-if="store.options"
      ><p>
        请恰选 {{ store.options.selection_count }} 种，每种一件；已选
        {{ store.selected.length }} 种。
      </p>
      <div class="product-grid">
        <article v-for="p in store.options.products" :key="p.id" class="product-card">
          <img
            v-if="p.image_urls[0]"
            :src="p.image_urls[0]"
            :alt="p.name"
            loading="lazy"
          />
          <h2>{{ p.name }}</h2>
          <p v-for="a in p.attributes" :key="a.name">{{ a.name }}：{{ a.value }}</p>
          <button
            :aria-pressed="store.selected.includes(p.id)"
            :disabled="busy"
            @click="choose(p.id)"
          >
            {{ store.selected.includes(p.id) ? '已选择' : '选择商品' }}
          </button>
        </article>
      </div>
      <button
        :disabled="busy || store.selected.length !== store.options.selection_count"
        @click="router.push('/redeem/address')"
      >
        继续填写收货信息
      </button></template
    >
  </main>
</template>
