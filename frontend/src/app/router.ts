import { createRouter, createWebHistory } from 'vue-router';
import { useSession } from '../stores/session';
export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('../features/catalog/Products.vue') },
    { path: '/verify', component: () => import('../features/redemption/Verify.vue') },
    { path: '/redeem', redirect: '/verify' },
    {
      path: '/redeem/select',
      component: () => import('../features/redemption/Select.vue'),
    },
    {
      path: '/redeem/address',
      component: () => import('../features/redemption/Address.vue'),
    },
    {
      path: '/redeem/confirm',
      component: () => import('../features/redemption/Confirm.vue'),
    },
    { path: '/orders/query', component: () => import('../features/orders/Query.vue') },
    { path: '/orders/edit', component: () => import('../features/orders/Edit.vue') },
    { path: '/orders/result', component: () => import('../features/orders/Result.vue') },
    {
      path: '/orders/tracking',
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
    { path: '/products/:id', component: () => import('../features/catalog/Detail.vue') },
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
