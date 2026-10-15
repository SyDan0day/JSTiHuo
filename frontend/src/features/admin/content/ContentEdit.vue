<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request, RequestError } from '../../../api/client';
import type { components } from '../../../api/generated/schema';
import { useSession } from '../../../stores/session';

const route = useRoute(),
  router = useRouter(),
  session = useSession();
const draft = reactive<components['schemas']['ContentInput']>({
  slug: '',
  title: '',
  body_html: '',
  kind: 'PAGE',
  published: false,
  sort_order: 0,
});
const version = ref(0),
  busy = ref(false),
  ready = ref(false),
  error = ref(''),
  saved = ref('');
const pending = ref<{
  path: string;
  body: string;
  key: string;
  method: string;
}>();
const conflict = ref(false);
const canWrite = computed(() => session.session?.permissions.includes('content.write'));
const editing = computed(() => Boolean(route.params.id));
let generation = 0;

/**
 * acceptContent 用途：同步服务端完整快照与版本，防止版本对应过期表单字段。
 * 参数：page 已读取或保存的内容；省略时初始化新草稿。
 * 返回值：无，同步更新表单与版本。
 */
function acceptContent(page?: components['schemas']['Content']) {
  Object.assign(draft, {
    slug: page?.slug ?? '',
    title: page?.title ?? '',
    body_html: page?.body_html ?? '',
    kind: page?.kind ?? 'PAGE',
    published: page?.published ?? false,
    sort_order: page?.sort_order ?? 0,
  });
  version.value = page?.version ?? 0;
}

/**
 * load 用途：读取内容完整快照；新建时清空上一页状态。
 * 参数：无。
 * 返回值：加载 Promise，失败不开放提交，旧响应不会覆盖新路由。
 */
async function load() {
  const current = ++generation;
  ready.value = false;
  busy.value = true;
  pending.value = undefined;
  error.value = saved.value = '';
  conflict.value = false;
  try {
    const page = editing.value
      ? await request<components['schemas']['Content']>('/api/v1/admin/content/' + route.params.id)
      : undefined;
    if (current !== generation) return;
    acceptContent(page);
    ready.value = true;
  } catch (e) {
    if (current === generation)
      error.value =
        e instanceof RequestError && e.status === 403
          ? '没有内容读取权限。'
          : '内容读取失败，请重试。';
  } finally {
    if (current === generation) busy.value = false;
  }
}

/**
 * save 用途：提交完整快照，结果不确定时冻结表单并使用原键原正文确认。
 * 参数：无。
 * 返回值：保存 Promise；冲突要求重新加载，成功采用服务端清洗结果。
 */
async function save() {
  if (busy.value || !ready.value || !canWrite.value || conflict.value) return;
  pending.value ??= {
    path: '/api/v1/admin/content' + (editing.value ? '/' + route.params.id : ''),
    method: editing.value ? 'PATCH' : 'POST',
    key: crypto.randomUUID(),
    body: JSON.stringify(editing.value ? { ...draft, version: version.value } : draft),
  };
  const action = pending.value;
  const current = generation;
  busy.value = true;
  error.value = saved.value = '';
  try {
    const result = await request<components['schemas']['Content']>(
      action.path,
      { method: action.method, body: action.body },
      action.key,
    );
    if (current !== generation) return;
    pending.value = undefined;
    if (!editing.value) await router.replace('/admin/content/' + result.id);
    else {
      acceptContent(result);
      saved.value =
        '已保存，版本 ' +
        result.version +
        (result.published ? '，公开页面已发布。' : '，当前为草稿。');
    }
  } catch (e) {
    if (current !== generation) return;
    if (e instanceof RequestError && e.status < 500) {
      pending.value = undefined;
      conflict.value = e.status === 409;
      error.value = conflict.value
        ? '版本或 slug 冲突，请重新加载核对；slug 必须唯一。'
        : e.status === 403
          ? '没有写入权限或会话校验失败，请重新登录后重试。'
          : '保存被拒绝，请检查字段、长度和内容类型。';
    } else error.value = '保存结果尚未确认。表单已锁定，请使用原请求确认结果。';
  } finally {
    if (current === generation) busy.value = false;
  }
}
watch(() => route.params.id, load, { immediate: true });
</script>

<template>
  <main class="admin-page editor">
    <RouterLink to="/admin/content">← 返回内容管理</RouterLink>
    <h1>{{ editing ? '查看与编辑内容' : '新增内容' }}</h1>
    <p v-if="version">当前版本 {{ version }}。</p>
    <p>正文仅保留安全文字与排版标签，图片、链接、脚本及样式会被移除。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="saved" role="status">{{ saved }}</p>
    <button v-if="(!ready || conflict) && !busy" @click="load">重新加载并核对</button>
    <p v-if="busy && !ready" role="status">加载中…</p>
    <form v-if="ready" @submit.prevent="save">
      <fieldset :disabled="busy || !!pending || !canWrite || conflict">
        <legend>内容快照</legend>
        <label for="content-title">标题</label
        ><input id="content-title" v-model="draft.title" maxlength="120" required />
        <label for="content-slug">slug（公开地址）</label
        ><input
          id="content-slug"
          v-model="draft.slug"
          pattern="[a-z0-9-]{1,64}"
          maxlength="64"
          required
          aria-describedby="slug-help"
        />
        <small id="slug-help"
          >仅小写字母、数字和连字符。固定入口：guide / about / contact / notice。</small
        >
        <label for="content-kind">内容类型</label
        ><select id="content-kind" v-model="draft.kind">
          <option value="PAGE">信息页</option>
          <option value="ANNOUNCEMENT">公告</option>
          <option value="BANNER">首页推荐</option>
        </select>
        <label for="content-body">正文 HTML</label
        ><textarea id="content-body" v-model="draft.body_html" rows="12" maxlength="50000" />
        <label for="content-sort">展示顺序</label
        ><input
          id="content-sort"
          v-model.number="draft.sort_order"
          type="number"
          min="0"
          max="1000000"
          required
        />
        <small
          >首页从最新 100 条公开内容中展示推荐与公告，按展示顺序升序排列；后台列表固定按 ID
          降序。</small
        >
        <label class="inline"
          ><input
            v-model="draft.published"
            type="checkbox"
          />发布到公开页面（取消勾选即撤下）</label
        >
        <button type="submit">{{ busy ? '保存中…' : '保存完整内容' }}</button>
      </fieldset>
    </form>
    <button v-if="pending" :disabled="busy" @click="save">使用原请求确认结果</button>
    <RouterLink v-if="ready && draft.published && editing && !pending" :to="'/pages/' + draft.slug"
      >查看公开页面 →</RouterLink
    >
  </main>
</template>

<style scoped>
select {
  min-height: 44px;
  font: inherit;
  padding: 6px 12px;
}
small {
  color: #78716c;
}
input[type='checkbox'] {
  min-width: 24px;
  min-height: 24px;
}
</style>
