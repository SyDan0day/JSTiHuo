<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { request, RequestError, setCsrf } from '../../api/client';
import { useRedemption } from '../../stores/redemption';
import type { components } from '../../api/generated/schema';
const router = useRouter(),
  store = useRedemption(),
  code = ref(''),
  secret = ref(''),
  answer = ref(''),
  challenge = ref<components['schemas']['Challenge']>(),
  busy = ref(false),
  error = ref('');

/**
 * refresh 用途：创建预认证并换取 VOUCHER 专用挑战。
 * 参数：无，使用组件状态。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示可恢复错误，不保留答案。
 */
async function refresh() {
  try {
    await request('/api/v1/session/bootstrap');
    challenge.value = await request('/api/v1/challenges', {
      method: 'POST',
      body: JSON.stringify({ purpose: 'VOUCHER' }),
    });
    answer.value = '';
  } catch {
    error.value = '验证码加载失败，请重试';
  }
}

/**
 * verify 用途：提交凭证鉴真并进入选品或显示状态。
 * 参数：无，使用卡号、密码和验证码输入。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 错误：失败显示统一错误并换挑战。
 * 约束：密码提交后及结束时立即清空，不放入store 或 URL。
 */
async function verify() {
  busy.value = true;
  error.value = '';
  try {
    if (!challenge.value) return;
    const body = JSON.stringify({
      code: code.value.trim().toUpperCase(),
      secret: secret.value,
      challenge_id: challenge.value.id,
      challenge_answer: answer.value,
    });
    secret.value = '';
    const v = await request<components['schemas']['Verification']>(
      '/api/v1/voucher-sessions',
      { method: 'POST', body },
    );
    store.clear();
    store.verification = v;
    setCsrf(v.csrf_token, 'voucher');
    if (
      v.active &&
      !v.redeemed &&
      Date.now() >= Date.parse(v.valid_from) &&
      Date.now() < Date.parse(v.valid_until)
    )
      await router.push('/redeem/select');
  } catch (e) {
    error.value =
      e instanceof RequestError
        ? ({
            RATE_LIMITED: '尝试过于频繁，请稍后重试',
            CHALLENGE_INVALID: '验证码无效，请重新输入',
            INVALID_CREDENTIALS: '卡号或密码不正确',
          }[e.detail.code] ?? e.detail.code)
        : '网络异常，请重试';
    await refresh();
  } finally {
    secret.value = '';
    busy.value = false;
  }
}
onMounted(refresh);
</script>
<template>
  <main class="auth">
    <RouterLink to="/">首页</RouterLink>
    <h1>卡券鉴真与提货</h1>
    <form @submit.prevent="verify">
      <label for="card-code">卡号</label
      ><input
        id="card-code"
        v-model="code"
        autocomplete="off"
        maxlength="32"
        minlength="6"
        required
      /><label for="card-secret">卡密</label
      ><input
        id="card-secret"
        v-model="secret"
        type="password"
        autocomplete="off"
        maxlength="32"
        minlength="6"
        required
      /><label for="captcha">四位图片验证码</label
      ><img
        v-if="challenge"
        :src="challenge.image_data_url"
        alt="四位数字验证码"
      /><button type="button" :disabled="busy" @click="refresh">换一张</button
      ><input
        id="captcha"
        v-model="answer"
        inputmode="numeric"
        autocomplete="off"
        maxlength="8"
        minlength="4"
        required
      />
      <p role="alert">{{ error }}</p>
      <button :disabled="busy || !challenge">
        {{ busy ? '验证中…' : '鉴真并继续' }}
      </button>
    </form>
    <section v-if="store.verification">
      <h2>卡券真实</h2>
      <p>
        {{
          store.verification.redeemed
            ? '此卡已消费'
            : !store.verification.active
              ? '此卡尚未激活'
              : Date.now() >= Date.parse(store.verification.valid_until)
                ? '此卡已到期'
                : '此卡尚未到生效时间'
        }}
      </p>
      <p>
        有效期 {{ new Date(store.verification.valid_from).toLocaleString() }} 至
        {{ new Date(store.verification.valid_until).toLocaleString() }}
      </p>
      <RouterLink to="/orders/query">查询已有订单</RouterLink>
    </section>
  </main>
</template>
