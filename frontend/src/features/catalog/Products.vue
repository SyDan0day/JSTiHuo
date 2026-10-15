<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { request } from '../../api/client';
import type { components } from '../../api/generated/schema';
import PublicLayout from '../../layouts/PublicLayout.vue';
import ProductGrid from '../../components/ProductGrid.vue';

const products = ref<components['schemas']['ProductPublic'][]>([]);
const next = ref(''),
  busy = ref(false),
  error = ref('');

/**
 * load 用途：读取礼品分页，失败重试保留当前列表与游标。
 * 参数：after 上次成功返回的排他游标，空值读取首页。
 * 返回值：加载 Promise；失败显示可重试提示。
 */
async function load(after = '') {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const page = await request<components['schemas']['ProductPublicList']>(
      '/api/v1/products?limit=20' + (after ? '&after_id=' + after : ''),
    );
    products.value = after ? [...products.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch {
    error.value = '商品加载失败，请重试。';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>

<template>
  <PublicLayout>
    <p class="eyebrow">每一份礼物，都是心意</p>
    <h1>礼品目录</h1>
    <p>查看礼品详情与规格。可提货商品以您卡券的实际权益为准。</p>
    <div :aria-busy="busy">
      <p v-if="error" role="alert">
        {{ error }}
        <button :disabled="busy" @click="load(products.length ? next : '')">重试</button>
      </p>
      <p v-else-if="busy && !products.length" role="status">加载中…</p>
      <p v-else-if="!products.length" class="public-empty">暂无上架商品，请稍后再来看看。</p>
      <ProductGrid :products="products" />
      <button v-if="next" :disabled="busy" class="load-more" @click="load(next)">
        {{ busy ? '加载中…' : '加载更多' }}
      </button>
    </div>
  </PublicLayout>
</template>
