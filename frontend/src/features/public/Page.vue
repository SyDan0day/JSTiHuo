<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';
import PublicLayout from '../../layouts/PublicLayout.vue';

const route = useRoute();
const page = ref<components['schemas']['Content']>();
const error = ref(''),
  busy = ref(false);
let generation = 0;

/**
 * load 用途：按当前 slug 读取已发布内容，防止快速导航展示旧响应。
 * 参数：无。
 * 返回值：加载 Promise；未发布与不存在统一显示内容不可用。
 */
async function load() {
  const current = ++generation;
  busy.value = true;
  error.value = '';
  page.value = undefined;
  try {
    const result = await request<components['schemas']['Content']>(
      '/api/v1/content/' + encodeURIComponent(String(route.params.slug)),
    );
    if (current === generation) page.value = result;
  } catch (e) {
    if (current === generation)
      error.value =
        e instanceof RequestError && e.status === 404
          ? '此内容尚未发布或已撤下。'
          : '内容加载失败，请重试。';
  } finally {
    if (current === generation) busy.value = false;
  }
}
watch(() => route.params.slug, load, { immediate: true });
</script>

<template>
  <PublicLayout>
    <RouterLink to="/">← 返回首页</RouterLink>
    <article class="public-article" :aria-busy="busy">
      <template v-if="page">
        <p class="eyebrow">服务信息</p>
        <h1>{{ page.title }}</h1>
        <div class="public-prose" v-html="page.body_html" />
      </template>
      <template v-else-if="error"
        ><h1>服务信息</h1>
        <p role="alert">{{ error }}</p>
        <button @click="load">重新加载</button></template
      >
      <p v-else role="status">正在加载内容…</p>
    </article>
  </PublicLayout>
</template>
