<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';
import PublicLayout from '../../layouts/PublicLayout.vue';

const route = useRoute(),
  product = ref<components['schemas']['ProductPublic']>();
const error = ref('');
let generation = 0;

/**
 * load 用途：读取当前礼品详情，避免快速切换时旧响应覆盖新商品。
 * 参数：无。
 * 返回值：加载 Promise；下架与不存在统一提示，其他错误允许重试。
 */
async function load() {
  const current = ++generation;
  error.value = '';
  product.value = undefined;
  try {
    const result = await request<components['schemas']['ProductPublic']>(
      '/api/v1/products/' + encodeURIComponent(String(route.params.id)),
    );
    if (current === generation) product.value = result;
  } catch (e) {
    if (current === generation)
      error.value =
        e instanceof RequestError && e.status === 404
          ? '商品不存在或已下架。'
          : '商品读取失败，请重试。';
  }
}
watch(() => route.params.id, load, { immediate: true });
</script>

<template>
  <PublicLayout>
    <RouterLink to="/products">← 返回礼品目录</RouterLink>
    <p v-if="error" role="alert">{{ error }} <button @click="load">重试</button></p>
    <article v-else-if="product" class="public-product-detail">
      <div class="detail-images">
        <img
          v-for="(src, index) in product.image_urls"
          :key="src"
          :src="src"
          :alt="product.name + '，图片 ' + (index + 1)"
          :loading="index ? 'lazy' : 'eager'"
        />
      </div>
      <div>
        <p class="eyebrow">礼品详情</p>
        <h1>{{ product.name }}</h1>
        <dl>
          <template v-for="(attribute, index) in product.attributes" :key="index">
            <dt>{{ attribute.name }}</dt>
            <dd>{{ attribute.value }}</dd>
          </template>
        </dl>
        <p>此页用于浏览礼品。提货时请先鉴真卡券，确认可选权益。</p>
        <RouterLink class="public-action" to="/verify">鉴真卡券与查看权益 →</RouterLink>
      </div>
      <section class="public-product-description">
        <h2>礼品介绍</h2>
        <div class="public-prose" v-html="product.description_html" />
      </section>
    </article>
    <p v-else role="status">加载中…</p>
  </PublicLayout>
</template>
