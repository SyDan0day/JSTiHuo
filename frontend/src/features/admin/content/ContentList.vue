<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
import { useSession } from '../../../stores/session';

const session = useSession();
const items = ref<components['schemas']['Content'][]>([]);
const next = ref(''),
  query = ref(''),
  status = ref('');
const busy = ref(false),
  error = ref('');
let applied = '';

/**
 * load 用途：按标题或 slug 和发布状态读取后台内容。
 * 参数：after 成功响应返回的游标；翻页复用已应用筛选。
 * 返回值：加载 Promise，失败保持当前列表并允许重试。
 */
async function load(after = '') {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  const filter = after
    ? applied
    : new URLSearchParams({ q: query.value, status: status.value }).toString();
  try {
    const page = await request<components['schemas']['ContentList']>(
      '/api/v1/admin/content?limit=20&' + filter + (after ? '&after_id=' + after : ''),
    );
    items.value = after ? [...items.value, ...page.items] : page.items;
    applied = filter;
    next.value = page.next_after_id ?? '';
  } catch (e) {
    error.value =
      e instanceof RequestError && e.status === 403
        ? '没有内容读取权限。'
        : '内容读取失败，请重试。';
  } finally {
    busy.value = false;
  }
}
onMounted(() => load());
</script>

<template>
  <main class="admin-page">
    <RouterLink to="/admin">← 管理工作台</RouterLink>
    <div class="page-title">
      <h1>内容管理</h1>
      <RouterLink
        v-if="session.session?.permissions.includes('content.write')"
        to="/admin/content/new"
        >新增内容</RouterLink
      >
    </div>
    <p>
      维护公告、首页推荐与信息页。须知、关于、联系和公告入口分别使用 guide、about、contact、notice。
    </p>
    <form class="filters" @submit.prevent="load()">
      <label for="content-search">标题或 slug</label
      ><input id="content-search" v-model="query" maxlength="100" />
      <label for="content-status">发布状态</label
      ><select id="content-status" v-model="status">
        <option value="">全部</option>
        <option value="DRAFT">草稿</option>
        <option value="PUBLISHED">已发布</option>
      </select>
      <button type="submit" :disabled="busy">查询</button>
    </form>
    <p v-if="error" role="alert">
      {{ error }} <button :disabled="busy" @click="load()">重试</button>
    </p>
    <p v-else-if="busy && !items.length" role="status">加载中…</p>
    <p v-else-if="!items.length">暂无匹配内容。请先创建并发布自有内容。</p>
    <ul class="content-admin-list" :aria-busy="busy">
      <li v-for="item in items" :key="item.id">
        <div>
          <strong>{{ item.title }}</strong>
          <p>
            {{ item.slug }} · {{ item.kind }} · {{ item.published ? '已发布' : '草稿' }} · 版本
            {{ item.version }}
          </p>
        </div>
        <RouterLink :to="'/admin/content/' + item.id">查看与编辑</RouterLink>
        <RouterLink v-if="item.published" :to="'/pages/' + item.slug">公开页面</RouterLink>
      </li>
    </ul>
    <button v-if="next" :disabled="busy" @click="load(next)">加载更多</button>
  </main>
</template>

<style scoped>
select {
  min-height: 44px;
  font: inherit;
  padding: 6px 12px;
}
.filters {
  flex-wrap: wrap;
}
.content-admin-list {
  list-style: none;
  padding: 0;
}
.content-admin-list li {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 20px;
  border-bottom: 1px solid #ddd;
  padding: 20px 0;
  overflow-wrap: anywhere;
}
.content-admin-list li > div {
  flex: 1;
  min-width: 180px;
}
.content-admin-list p {
  margin: 4px 0;
  color: #78716c;
}
</style>
