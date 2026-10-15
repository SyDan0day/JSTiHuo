<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { request } from '../../../api/client';
import { useCommand } from '../../../api/command';
import { useSession } from '../../../stores/session';
import type { components } from '../../../api/generated/schema';
type Admin = components['schemas']['Admin'];
type Role = components['schemas']['Role'];
const route = useRoute(),
  router = useRouter(),
  session = useSession();
const rolesMode = route.path === '/admin/roles';
const { busy, error, pending, send } = useCommand();
const loading = ref(false),
  loadError = ref(''),
  success = ref('');
const entries = ref<(Admin | Role)[]>([]),
  roles = ref<Role[]>([]),
  next = ref('');
const directory = ref<components['schemas']['Permission'][]>([]);
const q = ref(typeof route.query.q === 'string' ? route.query.q : ''),
  status = ref(typeof route.query.status === 'string' ? route.query.status : '');
const id = ref(''),
  version = ref(1),
  name = ref(''),
  enabled = ref(true),
  temporary = ref('');
const permissionKeys = ref<string[]>([]),
  roleIds = ref<string[]>([]),
  manualRoles = ref('');
const writable = computed(() =>
  session.session?.permissions.includes(rolesMode ? 'role.write' : 'admin.write'),
);
const groups = computed(() => [...new Set(directory.value.map((p) => p.key.split('.')[0]))]);
const grantableRoles = computed(() =>
  roles.value.filter(
    (role) =>
      role.enabled &&
      role.permission_keys.every((key) => session.session?.permissions.includes(key)),
  ),
);
/**
 * loadDirectory 用途：读取全部可授予权限或可选角色，避免只显示首个分页。
 * 参数：无。
 * 返回值：加载 Promise；权限不足时仍由服务端拒绝授予。
 */
async function loadDirectory() {
  if (!session.session?.permissions.includes('role.read')) {
    directory.value = (session.session?.permissions ?? []).map((key) => ({
      key,
      description: key,
    }));
    return;
  }
  let after = '';
  do {
    const page = await request<components['schemas']['PermissionList']>(
      '/api/v1/admin/permissions?limit=100' + (after ? '&after_id=' + after : ''),
    );
    directory.value.push(...page.items);
    after = page.next_after_id ?? '';
  } while (after);
  if (rolesMode) return;
  do {
    const page = await request<components['schemas']['RoleList']>(
      '/api/v1/admin/roles?limit=100' + (after ? '&after_id=' + after : ''),
    );
    roles.value.push(...page.items);
    after = page.next_after_id ?? '';
  } while (after);
}
/**
 * load 用途：分页读取管理员或角色，筛选只在 URL 保存非敏感字段。
 * 参数：after 为排他游标，空值重新筛选。
 * 返回值：加载 Promise，失败保留错误入口。
 */
async function load(after = '') {
  if (busy.value || pending.value || loading.value) return;
  loading.value = true;
  loadError.value = '';
  try {
    await router.replace({ query: { q: q.value || undefined, status: status.value || undefined } });
    const query = new URLSearchParams({ q: q.value, status: status.value, limit: '20' });
    if (after) query.set('after_id', after);
    const page = await request<{ items: (Admin | Role)[]; next_after_id?: string }>(
      '/api/v1/admin/' + (rolesMode ? 'roles' : 'admins') + '?' + query,
    );
    entries.value = after ? [...entries.value, ...page.items] : page.items;
    next.value = page.next_after_id ?? '';
  } catch {
    loadError.value = '目录读取失败或没有查看权限。';
  } finally {
    loading.value = false;
  }
}
/**
 * edit 用途：创建新表单或复制选中对象的完整版本快照。
 * 参数：item 为已有管理员或角色，省略时新建。
 * 返回值：无，不允许编辑超出当前授予上限的角色。
 */
function edit(item?: Admin | Role) {
  if (busy.value || pending.value) return;
  if (
    item &&
    'permission_keys' in item &&
    item.permission_keys.some((key) => !directory.value.some((p) => p.key === key))
  ) {
    loadError.value = '该角色包含当前账号无权授予的操作，不能修改。';
    return;
  }
  id.value = item?.id ?? '';
  version.value = item?.version ?? 1;
  enabled.value = item?.enabled ?? true;
  name.value = item ? ('name' in item ? item.name : item.login_name) : '';
  temporary.value = '';
  permissionKeys.value = item && 'permission_keys' in item ? [...item.permission_keys] : [];
  roleIds.value = item && 'role_ids' in item ? [...item.role_ids] : [];
  manualRoles.value = roleIds.value.join(',');
  success.value = '';
}
/**
 * save 用途：提交完整角色或管理员快照并恢复当前会话权威状态。
 * 参数：无，使用表单版本与明确选择的权限或角色。
 * 返回值：保存 Promise，改权使当前会话失效时返回登录页。
 */
async function save() {
  const selectedRoles = session.session?.permissions.includes('role.read')
    ? [...roleIds.value]
    : manualRoles.value
        .split(',')
        .map((value) => value.trim())
        .filter(Boolean);
  const body = rolesMode
    ? {
        ...(id.value ? { version: version.value } : {}),
        name: name.value,
        enabled: enabled.value,
        permission_keys: [...permissionKeys.value],
      }
    : id.value
      ? { version: version.value, enabled: enabled.value, role_ids: selectedRoles }
      : { login_name: name.value, temporary_password: temporary.value, role_ids: selectedRoles };
  temporary.value = '';
  const result = await send<Admin | Role>(
    '/api/v1/admin/' + (rolesMode ? 'roles' : 'admins') + (id.value ? '/' + id.value : ''),
    body,
    id.value ? 'PATCH' : 'POST',
  );
  if (!result) return;
  await session.restore();
  if (!session.session) {
    await router.replace('/admin/login');
    return;
  }
  edit(result);
  success.value = '已保存。关联管理员的旧会话已按改权规则撤销。';
  await load();
}
onMounted(async () => {
  try {
    await loadDirectory();
  } catch {
    loadError.value = '可授予目录读取失败。';
  }
  await load();
});
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>{{ rolesMode ? '角色与操作权限' : '管理员账户' }}</h1>
    <p>
      保存前需在五分钟内完成身份复核。只能授予自己当前拥有的权限；最后一位可管理身份与角色的管理员受保护。
    </p>
    <RouterLink to="/admin/security">身份复核</RouterLink
    ><RouterLink :to="rolesMode ? '/admin/admins' : '/admin/roles'">{{
      rolesMode ? '管理员账户' : '角色目录'
    }}</RouterLink>
    <p v-if="error || loadError" role="alert">{{ error || loadError }}</p>
    <p v-if="success" role="status">{{ success }}</p>
    <form @submit.prevent="load()">
      <fieldset :disabled="busy || loading || !!pending">
        <label for="access-q">名称搜索</label
        ><input id="access-q" v-model="q" maxlength="100" /><label for="access-status">状态</label
        ><select id="access-status" v-model="status">
          <option value="">全部</option>
          <option value="ENABLED">启用</option>
          <option value="DISABLED">停用</option></select
        ><button>刷新目录</button>
      </fieldset>
    </form>
    <p v-if="!entries.length && !loading">暂无记录。</p>
    <ul>
      <li v-for="item in entries" :key="item.id">
        <button :disabled="!writable || busy || !!pending" @click="edit(item)">
          {{ 'name' in item ? item.name : item.login_name }} ·
          {{ item.enabled ? '启用' : '停用' }} · ID {{ item.id }} · v{{ item.version }}
        </button>
      </li>
    </ul>
    <button v-if="next" :disabled="busy || loading || !!pending" @click="load(next)">
      加载更多
    </button>
    <template v-if="writable"
      ><button :disabled="busy || !!pending" @click="edit()">
        {{ rolesMode ? '新建角色' : '新建管理员' }}
      </button>
      <form @submit.prevent="save">
        <fieldset :disabled="busy || !!pending">
          <legend>{{ id ? '编辑 ID ' + id : '新建' }}</legend>
          <label for="access-name">{{ rolesMode ? '角色名称' : '登录名' }}</label
          ><input
            id="access-name"
            v-model="name"
            required
            :disabled="!rolesMode && !!id"
            :minlength="rolesMode ? 1 : 3"
            maxlength="64"
            autocomplete="off"
          />
          <label v-if="id || rolesMode"><input v-model="enabled" type="checkbox" />启用</label>
          <template v-if="!rolesMode && !id"
            ><label for="access-password">临时密码（首次登录必须改密）</label
            ><input
              id="access-password"
              v-model="temporary"
              type="password"
              required
              minlength="12"
              maxlength="128"
              autocomplete="new-password"
          /></template>
          <template v-if="rolesMode"
            ><fieldset v-for="group in groups" :key="group">
              <legend>{{ group }}</legend>
              <label
                v-for="permission in directory.filter((item) => item.key.startsWith(group + '.'))"
                :key="permission.key"
                ><input v-model="permissionKeys" type="checkbox" :value="permission.key" />{{
                  permission.key
                }}
                · {{ permission.description }}</label
              >
            </fieldset></template
          >
          <fieldset v-else-if="session.session?.permissions.includes('role.read')">
            <legend>可授予的启用角色</legend>
            <label v-for="role in grantableRoles" :key="role.id"
              ><input v-model="roleIds" type="checkbox" :value="role.id" />{{ role.name }}（ID
              {{ role.id }}）</label
            >
          </fieldset>
          <template v-else
            ><label for="access-roles">角色 ID（英文逗号分隔，服务端校验授予上限）</label
            ><input id="access-roles" v-model="manualRoles" required
          /></template>
          <button
            :disabled="
              rolesMode
                ? !permissionKeys.length
                : session.session?.permissions.includes('role.read') && !roleIds.length
            "
          >
            保存{{ rolesMode ? '角色权限' : '管理员' }}
          </button>
        </fieldset>
      </form>
      <button v-if="pending" :disabled="busy" @click="save">使用原请求重试保存</button>
    </template>
  </main>
</template>
<style scoped>
main {
  overflow-wrap: anywhere;
}
input,
select {
  min-width: 0;
  max-width: 100%;
}
label:has(input[type='checkbox']) {
  display: flex;
  gap: 0.6rem;
  align-items: baseline;
  margin: 0.6rem 0;
}
input[type='checkbox'] {
  width: auto;
  flex-shrink: 0;
}
</style>
