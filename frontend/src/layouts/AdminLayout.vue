<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { useSession } from '../stores/session';
import { request } from '../api/client';
import type { components } from '../api/generated/schema';
const store = useSession(),
  router = useRouter(),
  error = ref('');
const stats = ref<components['schemas']['Dashboard']>();
/**
 * loadStats 用途：读取当前账号授权的业务对象统计。
 * 参数：无。
 * 返回值：加载 Promise，失败显示真实错误，不显示占位统计。
 */
async function loadStats() {
  if (!store.session?.permissions.includes('dashboard.read')) return;
  try {
    stats.value = await request('/api/v1/admin/dashboard');
  } catch {
    error.value = '统计读取失败，请刷新重试。';
  }
}
onMounted(loadStats);
/**
 * logout 用途：撤销管理员会话并返回登录页。
 * 参数：无。
 * 返回值：完成操作的 Promise，失败显示错误。
 */
async function logout() {
  try {
    await request('/api/v1/admin/session', { method: 'DELETE' });
    store.clear();
    router.replace('/admin/login');
  } catch {
    error.value = '退出失败，请重试';
  }
}
</script>
<template>
  <header>
    <strong>JSTiHuo · 管理后台</strong>
    <nav>
      <RouterLink to="/admin/security">身份复核</RouterLink
      ><RouterLink to="/admin/change-password">修改密码</RouterLink
      ><button @click="logout">退出</button>
    </nav>
  </header>
  <main>
    <h1>管理工作台</h1>
    <p>已通过管理员身份验证。可从商品管理维护目录。</p>
    <p role="alert">{{ error }}</p>
    <section v-if="stats" aria-label="业务统计">
      <h2>业务统计</h2>
      <p>未软删商品 {{ stats.products }} · 未软删卡券 {{ stats.vouchers }}</p>
      <p>
        待发 {{ stats.pending_orders }} · 已发 {{ stats.shipped_orders }} · 完成
        {{ stats.completed_orders }}
      </p>
      <button @click="loadStats">刷新统计</button>
    </section>
    <nav class="operations-links" aria-label="权限与运营设置">
      <RouterLink v-if="store.session?.permissions.includes('admin.read')" to="/admin/admins"
        >管理员账户</RouterLink
      >
      <RouterLink v-if="store.session?.permissions.includes('role.read')" to="/admin/roles"
        >角色与权限</RouterLink
      >
      <RouterLink v-if="store.session?.permissions.includes('config.read')" to="/admin/settings"
        >预约与通知设置</RouterLink
      >
      <RouterLink v-if="store.session?.permissions.includes('audit.read')" to="/admin/audit"
        >只读审计日志</RouterLink
      >
      <RouterLink to="/admin/regions">行政地区目录</RouterLink
      ><RouterLink to="/admin/tools/qr">公开入口二维码</RouterLink>
    </nav>
    <RouterLink v-if="store.session?.permissions.includes('product.read')" to="/admin/products"
      >商品管理</RouterLink
    >
    <p>
      <RouterLink v-if="store.session?.permissions.includes('inventory.read')" to="/admin/inventory"
        >库存管理与审核</RouterLink
      >
    </p>
    <p>
      <RouterLink
        v-if="store.session?.permissions.includes('voucher.read')"
        to="/admin/voucher-batches"
        >卡型、批次与卡券</RouterLink
      >
    </p>
    <p>
      <RouterLink v-if="store.session?.permissions.includes('order.read')" to="/admin/orders"
        >订单管理</RouterLink
      >
    </p>
    <p>
      <RouterLink v-if="store.session?.permissions.includes('carrier.read')" to="/admin/carriers"
        >承运商目录</RouterLink
      >
    </p>
    <p>
      <RouterLink v-if="store.session?.permissions.includes('order.pickup')" to="/admin/pickups"
        >到店核销</RouterLink
      >
    </p>
    <p>当前账号具有 {{ store.session?.permissions.length ?? 0 }} 项权限。</p>
    <p>
      <RouterLink
        v-if="store.session?.permissions.includes('after_sale.read')"
        to="/admin/after-sales"
        >售后与补发案件</RouterLink
      >
    </p>
    <p>
      <RouterLink
        v-if="store.session?.permissions.includes('notification.read')"
        to="/admin/notifications"
        >通知任务与结果核对</RouterLink
      >
    </p>
    <p>
      <RouterLink
        v-if="
          store.session?.permissions.some((permission) =>
            ['file.import', 'file.export'].includes(permission),
          )
        "
        to="/admin/files"
        >导入与脱敏导出</RouterLink
      >
    </p>
    <p>
      <RouterLink v-if="store.session?.permissions.includes('feedback.read')" to="/admin/feedback"
        >反馈处理</RouterLink
      >
    </p>
    <p>
      <RouterLink v-if="store.session?.permissions.includes('content.read')" to="/admin/content">
        公告与信息页内容管理
      </RouterLink>
    </p>
  </main>
</template>
<style scoped>
.operations-links {
  flex-wrap: wrap;
  margin: 1.5rem 0;
}
main {
  overflow-wrap: anywhere;
}
</style>
