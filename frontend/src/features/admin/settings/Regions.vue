<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { request } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
type Region = components['schemas']['Region'];
const items = ref<Region[]>([]),
  parent = ref('000000'),
  error = ref(''),
  busy = ref(false);
const trail = ref<{ code: string; name: string }[]>([{ code: '000000', name: '全部省份' }]);
/**
 * load 用途：读取锁定版本地区数据的直接子节点。
 * 参数：code 为父级代码；name 为面包屑名称。
 * 返回值：加载 Promise，无在线写入能力。
 */
async function load(code = '000000', name = '全部省份') {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const page = await request<components['schemas']['RegionList']>(
      '/api/v1/regions?parent_code=' + code,
    );
    items.value = page.items;
    parent.value = code;
    const index = trail.value.findIndex((item) => item.code === code);
    trail.value = index >= 0 ? trail.value.slice(0, index + 1) : [...trail.value, { code, name }];
  } catch {
    error.value = '地区读取失败，请重试。';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>行政地区只读目录</h1>
    <p>使用随版本发布的地区数据。在线编辑暂不提供；镇街信息应填写在详细地址。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <nav aria-label="地区层级">
      <button
        v-for="item in trail"
        :key="item.code"
        :disabled="busy"
        @click="load(item.code, item.name)"
      >
        {{ item.name }}
      </button>
    </nav>
    <p>父级 {{ parent }}</p>
    <p v-if="!items.length && !busy">此地区没有下一级。</p>
    <ul>
      <li v-for="item in items" :key="item.code">
        <button
          :disabled="busy || item.code === parent || trail.length >= 3"
          @click="load(item.code, item.name)"
        >
          {{ item.name }} · {{ item.code }}
        </button>
      </li>
    </ul>
  </main>
</template>
<style scoped>
nav {
  flex-wrap: wrap;
}
main {
  overflow-wrap: anywhere;
}
</style>
