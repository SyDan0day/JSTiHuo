<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import AddressFields from '../../components/AddressFields.vue';
import { getOrderScope, request, RequestError, setCsrf } from '../../api/client';
import { useSession } from '../../stores/session';
import type { components } from '../../api/generated/schema';

const route = useRoute();
const router = useRouter();
const session = useSession();
const isAdmin = route.path.startsWith('/admin/');
const path = isAdmin ? '/api/v1/admin/orders/' + route.params.id : '/api/v1/order';
const order = ref<components['schemas']['Order']>();
const form = ref<components['schemas']['OrderEdit']>({
  version: 1,
  address: {
    name: '',
    phone: '',
    province_code: '',
    city_code: '',
    district_code: '',
    detail: '',
  },
  ship_date: '',
  customer_note: '',
});
const days = ref<components['schemas']['CalendarDay'][]>([]);
const writable = ref(isAdmin);
const busy = ref(false);
const error = ref('');
const code = ref('');
const secret = ref('');
const answer = ref('');
const challenge = ref<components['schemas']['Challenge']>();
const pending = ref<{ body: string; key: string }>();
const canEdit = computed(() =>
  isAdmin ? session.session?.permissions.includes('order.write') : writable.value,
);

/**
 * load 用途：读取脱敏订单并恢复相应作用域和日期建议。
 * 参数：无，使用当前路由与同源 Cookie。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示错误，顾客读取日历不占名额，不预填旧 PII。
 */
async function load() {
  busy.value = true;
  error.value = '';
  try {
    order.value = await request(path);
    if (!order.value) return;
    form.value.version = order.value.version;
    form.value.ship_date = order.value.ship_date ?? '';
    form.value.customer_note = order.value.customer_note;
    if (!isAdmin) {
      writable.value = getOrderScope() === 'WRITE';
      days.value = (
        await request<components['schemas']['Calendar']>('/api/v1/redemption/calendar')
      ).items;
    }
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '读取失败，请重试';
  } finally {
    busy.value = false;
  }
}

/**
 * refreshChallenge 用途：为原卡密复核建立 VOUCHER 专用新挑战。
 * 参数：无，保留现有 ORDER 会话，只新建 PREAUTH。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：挑战失败显示提示，清空旧答案。
 */
async function refreshChallenge() {
  try {
    await request('/api/v1/session/bootstrap');
    challenge.value = await request('/api/v1/challenges', {
      method: 'POST',
      body: JSON.stringify({ purpose: 'VOUCHER' }),
    });
    answer.value = '';
  } catch {
    error.value = '验证码读取失败，请重试';
  }
}

/**
 * stepUp 用途：以对应原卡密把查询会话升级为 WRITE。
 * 参数：无，使用卡号、密码及挑战输入。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功接受新的 CSRF，失败换挑战，提交和结束立即清空卡密。
 */
async function stepUp() {
  if (!challenge.value) return;
  busy.value = true;
  error.value = '';
  try {
    const body = JSON.stringify({
      code: code.value,
      secret: secret.value,
      challenge_id: challenge.value.id,
      challenge_answer: answer.value,
    });
    secret.value = '';
    const result = await request<components['schemas']['Session']>(
      '/api/v1/order-session/step-up',
      { method: 'POST', body },
    );
    setCsrf(result.csrf_token, 'order');
    writable.value = true;
    challenge.value = undefined;
  } catch (e) {
    error.value = e instanceof RequestError ? e.detail.code : '复核失败，请重试';
    await refreshChallenge();
  } finally {
    secret.value = '';
    busy.value = false;
  }
}

/**
 * submit 用途：提交完整替换地址和日期，并支持结果不确定时的同键恢复。
 * 参数：无，使用页面版本快照和地址表单。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功清理 PII 并返回详情。
 * 约束：业务失败解除待确认，网络错误保留原正文。
 */
async function submit() {
  pending.value ??= { body: JSON.stringify(form.value), key: crypto.randomUUID() };
  busy.value = true;
  error.value = '';
  try {
    await request(path, { method: 'PATCH', body: pending.value.body }, pending.value.key);
    pending.value = undefined;
    form.value.address = {
      name: '',
      phone: '',
      province_code: '',
      city_code: '',
      district_code: '',
      detail: '',
    };
    await router.replace(isAdmin ? '/admin/orders/' + route.params.id : '/orders/result');
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ((
            {
              VERSION_CONFLICT: '订单已变化，请刷新后核对新版本',
              CAPACITY_EXCEEDED: '新日期已满，原名额保持不变，请重选',
              REGION_RESTRICTED: '所选商品不配送至该地区',
              ILLEGAL_TRANSITION: '已超过修改截止时间或订单不再待发',
            } as Record<string, string>
          )[e.detail.code] ?? e.detail.code)
        : '结果未确认，请重试原请求';
    if (e instanceof RequestError && e.status < 500) pending.value = undefined;
  } finally {
    busy.value = false;
  }
}

onMounted(load);
</script>
<template>
  <main class="auth">
    <RouterLink :to="isAdmin ? '/admin/orders/' + route.params.id : '/orders/result'"
      >返回详情</RouterLink
    >
    <h1>修改收货信息</h1>
    <p>只支持待发配送订单，预约日前一天 17 点前修改；需输入完整新地址。</p>
    <p v-if="order">
      {{ order.order_no }} · {{ order.recipient.name }} · {{ order.recipient.phone }}
    </p>
    <p role="alert">{{ error }}</p>
    <button :disabled="busy || !!pending" @click="load">刷新权威版本</button>
    <section v-if="!isAdmin && !writable">
      <p>查询会话只有读取权限。请用对应原卡密复核后修改。</p>
      <button v-if="!challenge" :disabled="busy" @click="refreshChallenge">
        开始原卡密复核
      </button>
      <form v-if="challenge" @submit.prevent="stepUp">
        <label for="stepup-code">原卡号</label
        ><input
          id="stepup-code"
          v-model="code"
          required
          maxlength="32"
          autocomplete="off"
        />
        <label for="stepup-secret">原卡密</label
        ><input
          id="stepup-secret"
          v-model="secret"
          type="password"
          required
          minlength="6"
          maxlength="32"
          autocomplete="off"
        />
        <label for="stepup-answer">四位验证码</label
        ><img :src="challenge.image_data_url" alt="四位数字验证码" />
        <button type="button" :disabled="busy" @click="refreshChallenge">换一张</button>
        <input
          id="stepup-answer"
          v-model="answer"
          required
          minlength="4"
          maxlength="8"
          inputmode="numeric"
          autocomplete="off"
        />
        <button :disabled="busy">确认身份复核</button>
      </form>
    </section>
    <form v-if="canEdit && order?.status === 'PENDING'" @submit.prevent="submit">
      <AddressFields v-model="form.address" :disabled="busy || !!pending" />
      <label for="edit-ship-date">预约发货日期</label>
      <input
        v-if="isAdmin"
        id="edit-ship-date"
        v-model="form.ship_date"
        type="date"
        required
        :disabled="busy || !!pending"
      />
      <select
        v-else
        id="edit-ship-date"
        v-model="form.ship_date"
        required
        :disabled="busy || !!pending"
      >
        <option v-for="d in days" :key="d.date" :value="d.date" :disabled="!d.available">
          {{ d.date }} · {{ d.available ? '可预约' : '不可预约' }}
        </option>
      </select>
      <label for="edit-note">备注</label
      ><textarea
        id="edit-note"
        v-model="form.customer_note"
        maxlength="200"
        :disabled="busy || !!pending"
      />
      <button :disabled="busy">{{ pending ? '重试原请求' : '确认替换收货信息' }}</button>
    </form>
  </main>
</template>
<style scoped>
select {
  font: inherit;
  min-height: 44px;
  padding: 8px;
}
</style>
