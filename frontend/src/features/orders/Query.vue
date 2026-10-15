<script setup lang="ts">
import { useChallenge } from '../../composables/useChallenge';
import PublicIcon from '../../components/PublicIcon.vue';
import { flowError } from '../../api/flow-error';
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError, setCsrf } from '../../api/client';
import type { components } from '../../api/generated/schema';
const router = useRouter(),
  identifier = ref(''),
  phone = ref(''),
  busy = ref(false);
const { challenge, answer, loading, error, refresh } = useChallenge('ORDER_QUERY');
/**
 * query 用途：以标识、手机、挑战换取只读订单会话。
 * 参数：无，使用输入字段。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：成功清空手机并进入脱敏订单页，失败统一提示且重新获取挑战，不把 PII 放入 URL。
 */
async function query() {
  if (busy.value || loading.value || !challenge.value) return;
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
          }[e.detail.code] ?? flowError(e))
        : '网络异常，请重试';
    await refresh(true);
  } finally {
    busy.value = false;
  }
}
onMounted(() => refresh());
</script>
<template>
  <section class="flow-page auth">
    <RouterLink to="/">← 返回首页</RouterLink>
    <div class="form-heading-icon"><PublicIcon name="truck" /></div>
    <p class="eyebrow">FOLLOW YOUR GIFT</p>
    <h1>查询提货订单</h1>
    <p>输入卡号或订单号与收货手机号，查看您的订单和配送进度。</p>
    <form @submit.prevent="query">
      <label for="order-identifier">卡号 / 订单号</label
      ><input
        id="order-identifier"
        placeholder="请输入卡号或订单号"
        v-model="identifier"
        maxlength="40"
        required
        autocomplete="off"
      /><label for="query-phone">收货手机号</label
      ><input
        id="query-phone"
        placeholder="请输入 11 位收货手机号"
        v-model="phone"
        type="tel"
        required
        pattern="[0-9]{11}"
        maxlength="11"
        autocomplete="off"
      /><label for="query-challenge">四位验证码</label>
      <div class="captcha-row">
        <img v-if="challenge" :src="challenge.image_data_url" alt="四位数字验证码" /><button
          type="button"
          :disabled="busy || loading"
          @click="refresh()"
        >
          {{ loading ? '加载中…' : '换一张' }}
        </button>
      </div>
      <input
        id="query-challenge"
        placeholder="请输入图片中的四位数字"
        v-model="answer"
        required
        minlength="4"
        maxlength="8"
        inputmode="numeric"
        autocomplete="off"
      />
      <p role="alert" aria-live="polite">{{ error }}</p>
      <button :disabled="busy || loading || !challenge">查询订单</button>
    </form>
    <div class="form-help">
      <p>查询遇到问题？<RouterLink to="/pages/contact">联系我们 →</RouterLink></p>
    </div>
  </section>
</template>
