import { createRouter, createWebHistory } from 'vue-router';
import { useSession } from '../stores/session';
export const router = createRouter({
  history: createWebHistory(),
  /**
   * 用途：新页面从顶部展示，浏览器前进或后退时恢复阅读位置。
   * 参数：to 目标路由；from 来源路由；savedPosition 历史滚动位置。
   * 返回值：应恢复的坐标，首次导航返回页面顶部。
   */
  scrollBehavior(to, from, savedPosition) {
    return savedPosition ?? { left: 0, top: 0 };
  },
  routes: [
    {
      path: '/admin/admins',
      component: () => import('../features/admin/settings/Access.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/roles',
      component: () => import('../features/admin/settings/Access.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/settings',
      component: () => import('../features/admin/settings/Settings.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/audit',
      component: () => import('../features/admin/settings/Audit.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/regions',
      component: () => import('../features/admin/settings/Regions.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/tools/qr',
      component: () => import('../features/admin/settings/QR.vue'),
      meta: { admin: true },
    },
    {
      path: '/orders/after-sales',
      component: () => import('../features/orders/AfterSales.vue'),
      meta: { customer: true },
    },
    {
      path: '/admin/after-sales',
      component: () => import('../features/orders/AfterSales.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/notifications',
      component: () => import('../features/admin/notifications/Notifications.vue'),
      meta: { admin: true },
    },
    { path: '/', component: () => import('../features/public/Home.vue') },
    { path: '/pages/:slug', component: () => import('../features/public/Page.vue') },
    { path: '/home/index/index.html', redirect: '/' },
    { path: '/home/index/product.html', redirect: '/products' },
    { path: '/home/index/card_verify.html', redirect: '/verify' },
    { path: '/home/index/card_exchange.html', redirect: '/redeem' },
    { path: '/home/index/order_query.html', redirect: '/orders/query' },
    { path: '/home/index/about_us.html', redirect: '/pages/about' },
    { path: '/home/index/contact_us.html', redirect: '/pages/contact' },
    { path: '/home/index/notice.html', redirect: '/pages/notice' },
    {
      path: '/feedback',
      component: () => import('../features/feedback/Feedback.vue'),
      meta: { customer: true },
    },
    {
      path: '/admin/feedback',
      component: () => import('../features/admin/feedback/Feedback.vue'),
      meta: { admin: true },
    },
    { path: '/home/index/feedback.html', redirect: '/feedback' },
    {
      path: '/verify',
      meta: { customer: true },
      component: () => import('../features/redemption/Verify.vue'),
    },
    { path: '/redeem', redirect: '/verify' },
    {
      path: '/redeem/select',
      meta: { customer: true },
      component: () => import('../features/redemption/Select.vue'),
    },
    {
      path: '/redeem/address',
      meta: { customer: true },
      component: () => import('../features/redemption/Address.vue'),
    },
    {
      path: '/redeem/confirm',
      meta: { customer: true },
      component: () => import('../features/redemption/Confirm.vue'),
    },
    {
      path: '/orders/query',
      meta: { customer: true },
      component: () => import('../features/orders/Query.vue'),
    },
    {
      path: '/orders/edit',
      meta: { customer: true },
      component: () => import('../features/orders/Edit.vue'),
    },
    {
      path: '/orders/result',
      meta: { customer: true },
      component: () => import('../features/orders/Result.vue'),
    },
    {
      path: '/orders/tracking',
      meta: { customer: true },
      component: () => import('../features/orders/Tracking.vue'),
    },
    {
      path: '/admin/carriers',
      component: () => import('../features/admin/shipping/Carriers.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/pickups',
      component: () => import('../features/admin/shipping/Pickup.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/orders/:id/ship',
      component: () => import('../features/admin/shipping/Ship.vue'),
      meta: { admin: true },
    },
    { path: '/products', component: () => import('../features/catalog/Products.vue') },
    {
      path: '/admin/files',
      component: () => import('../features/admin/files/Files.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/jobs/:id',
      component: () => import('../features/admin/files/Job.vue'),
      meta: { admin: true },
    },
    { path: '/products/:id', component: () => import('../features/catalog/Detail.vue') },
    {
      path: '/admin/content',
      component: () => import('../features/admin/content/ContentList.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/content/new',
      component: () => import('../features/admin/content/ContentEdit.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/content/:id',
      component: () => import('../features/admin/content/ContentEdit.vue'),
      meta: { admin: true },
    },
    { path: '/admin/content/:id/edit', redirect: (to) => '/admin/content/' + to.params.id },
    {
      path: '/admin/orders/:id/edit',
      component: () => import('../features/orders/Edit.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/orders/:id',
      component: () => import('../features/orders/AdminDetail.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/orders',
      component: () => import('../features/orders/AdminOrders.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/voucher-types',
      component: () => import('../features/admin/vouchers/Manage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/voucher-batches',
      component: () => import('../features/admin/vouchers/Manage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/voucher-batches/new',
      component: () => import('../features/admin/vouchers/Manage.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/voucher-batches/:id',
      component: () => import('../features/admin/vouchers/Batch.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/vouchers',
      component: () => import('../features/admin/vouchers/Vouchers.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/inventory',
      component: () => import('../features/admin/inventory/Inventory.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/inventory/documents',
      component: () => import('../features/admin/inventory/Documents.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/products',
      component: () => import('../features/admin/catalog/Products.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/products/new',
      component: () => import('../features/admin/catalog/ProductEdit.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/products/:id/edit',
      component: () => import('../features/admin/catalog/ProductEdit.vue'),
      meta: { admin: true },
    },
    { path: '/admin/login', component: () => import('../features/admin/auth/Login.vue') },
    {
      path: '/admin/change-password',
      component: () => import('../features/admin/auth/ChangePassword.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin/security',
      component: () => import('../features/admin/auth/Reauth.vue'),
      meta: { admin: true },
    },
    {
      path: '/admin',
      component: () => import('../layouts/AdminLayout.vue'),
      meta: { admin: true },
    },
    { path: '/:pathMatch(.*)*', component: () => import('../layouts/NotFound.vue') },
  ],
});
router.beforeEach(async (to) => {
  if (!to.meta.admin) return;
  const store = useSession();
  if (!store.session) await store.restore();
  if (!store.session) return '/admin/login';
  if (store.session.force_password_change && to.path != '/admin/change-password')
    return '/admin/change-password';
});
