<script setup lang="ts">
import { onUnmounted, ref } from 'vue';
import { request, RequestError, setCsrf } from '../api/client';
import { flowError } from '../api/flow-error';
import type { components } from '../api/generated/schema';
const emit = defineEmits<{ verified: [] }>();
const code = ref(''),
  secret = ref(''),
  answer = ref(''),
  error = ref(''),
  busy = ref(false);
const challenge = ref<components['schemas']['Challenge']>();
/**
 * refreshChallenge 用途：读取原卡密复核专用的一次性挑战。
 * 参数：无。
 * 返回值：加载 Promise，失败显示恢复入口。
 */
async function refreshChallenge() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  challenge.value = undefined;
  try {
    await request('/api/v1/session/bootstrap');
    challenge.value = await request('/api/v1/challenges', {
      method: 'POST',
      body: JSON.stringify({ purpose: 'VOUCHER' }),
    });
    answer.value = '';
  } catch {
    error.value = '验证码读取失败，请重试。';
  } finally {
    busy.value = false;
  }
}
/**
 * stepUp 用途：用对应原卡密将订单读取会话升级为 WRITE。
 * 参数：无，使用当前挑战与用户输入。
 * 返回值：验证 Promise，成功发出 verified；卡密不持久化、不自动重试。
 */
async function stepUp() {
  if (busy.value || !challenge.value) return;
  busy.value = true;
  error.value = '';
  const body = JSON.stringify({
    code: code.value,
    secret: secret.value,
    challenge_id: challenge.value.id,
    challenge_answer: answer.value,
  });
  secret.value = '';
  try {
    const result = await request<components['schemas']['Session']>(
      '/api/v1/order-session/step-up',
      { method: 'POST', body },
    );
    setCsrf(result.csrf_token, 'order');
    emit('verified');
  } catch (e) {
    error.value = e instanceof RequestError ? flowError(e) : '复核结果未确认，请重新获取验证码。';
    challenge.value = undefined;
  } finally {
    busy.value = false;
    secret.value = '';
  }
}
onUnmounted(() => {
  secret.value = '';
  code.value = '';
  answer.value = '';
});
</script>
<template>
  <section aria-labelledby="stepup-heading">
    <h2 id="stepup-heading">原卡密身份复核</h2>
    <p>查询会话只有读取权限，写入前需用对应原卡密复核。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <button v-if="!challenge" :disabled="busy" @click="refreshChallenge">开始原卡密复核</button>
    <form v-else @submit.prevent="stepUp">
      <fieldset :disabled="busy">
        <label for="stepup-code">原卡号</label
        ><input id="stepup-code" v-model="code" required maxlength="32" autocomplete="off" />
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
        <button type="button" @click="refreshChallenge">换一张</button>
        <input
          id="stepup-answer"
          v-model="answer"
          required
          minlength="4"
          maxlength="8"
          inputmode="numeric"
          autocomplete="off"
        />
        <button>确认身份复核</button>
      </fieldset>
    </form>
  </section>
</template>
