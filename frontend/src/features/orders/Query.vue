<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError, setCsrf } from '../../api/client';
import type { components } from '../../api/generated/schema';
const router = useRouter(),
  identifier = ref(''),
  phone = ref(''),
  answer = ref(''),
  challenge = ref<components['schemas']['Challenge']>(),
  busy = ref(false),
  error = ref('');
/**
 * refresh 用途：建立 ORDER_QUERY 专用预认证挑战。
 * 参数：无，使用当前 Cookie。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示重试提示，清空旧答案。
 */
async function refresh() {
  try {
    await request('/api/v1/session/bootstrap');
    challenge.value = await request('/api/v1/challenges', {
      method: 'POST',
      body: JSON.stringify({ purpose: 'ORDER_QUERY' }),
    });
    answer.value = '';
  } catch {
    error.value = '验证码读取失败，请重试';
  }
}
/**
 * query 用途：以标识、手机、挑战换取只读订单会话。
 * 参数：无，使用输入字段。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功清空手机并进入脱敏订单页，失败统一提示且重新获取挑战，不把 PII 放入 URL。
 */
async function query() {
  if (!challenge.value) return;
  busy.value = true;
  error.value = '';
  try {
    const v = await request<components['schemas']['Session']>('/api/v1/order-sessions', {
      method: 'POST',
      body: JSON.stringify({
        identifier: identifier.value,
        phone: phone.value,
        challenge_id: challenge.value.id,
        challenge_answer: answer.value,
      }),
    });
    setCsrf(v.csrf_token, 'order');
    phone.value = '';
    await router.replace('/orders/result');
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ({
            INVALID_CREDENTIALS: '标识或收货手机号不匹配',
            RATE_LIMITED: '尝试过于频繁，请稍后重试',
            CHALLENGE_INVALID: '验证码无效，请重新输入',
          }[e.detail.code] ?? e.detail.code)
        : '网络异常，请重试';
    await refresh();
  } finally {
    busy.value = false;
  }
}
onMounted(refresh);
</script>
<template>
  <main class="auth">
    <RouterLink to="/">首页</RouterLink>
    <h1>查询提货订单</h1>
    <p>使用卡号或订单号与完整收货手机查询；查询会话只能读取订单。</p>
    <form @submit.prevent="query">
      <label for="order-identifier">卡号 / 订单号</label
      ><input
        id="order-identifier"
        v-model="identifier"
        maxlength="40"
        required
        autocomplete="off"
      /><label for="query-phone">收货手机号</label
      ><input
        id="query-phone"
        v-model="phone"
        type="tel"
        required
        pattern="[0-9]{11}"
        maxlength="11"
        autocomplete="off"
      /><label for="query-challenge">四位验证码</label
      ><img
        v-if="challenge"
        :src="challenge.image_data_url"
        alt="四位数字验证码"
      /><button type="button" :disabled="busy" @click="refresh">换一张</button
      ><input
        id="query-challenge"
        v-model="answer"
        required
        minlength="4"
        maxlength="8"
        inputmode="numeric"
        autocomplete="off"
      />
      <p role="alert">{{ error }}</p>
      <button :disabled="busy || !challenge">查询订单</button>
    </form>
  </main>
</template>
