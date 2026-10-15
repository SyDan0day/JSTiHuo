<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue';
import { request, RequestError } from '../../api/client';
import type { components } from '../../api/generated/schema';

const form = reactive({ contact_name: '', phone: '', email: '', order_no: '', message: '' });
const challenge = ref<components['schemas']['Challenge']>();
const answer = ref(''),
  error = ref('');
const result = ref<components['schemas']['Feedback']>();
const busy = ref(false),
  uncertain = ref(false);

/**
 * refresh 用途：取得 FEEDBACK 专用一次性挑战。
 * 参数：无。
 * 返回值：加载 Promise；失败清空失效挑战并显示重试提示。
 */
async function refresh() {
  challenge.value = undefined;
  answer.value = '';
  try {
    await request('/api/v1/session/bootstrap');
    challenge.value = await request('/api/v1/challenges', {
      method: 'POST',
      body: JSON.stringify({ purpose: 'FEEDBACK' }),
    });
  } catch {
    error.value = '验证码读取失败，请重试。';
  }
}

/**
 * submit 用途：提交真实公开反馈，不自动重复消费一次性挑战。
 * 参数：无，使用当前完整表单和挑战。
 * 返回值：提交 Promise；成功清空联系人，结果不确定时禁止重复提交。
 * 约束：联系人不进入 URL 或本地存储，反馈不授予订单读取权限。
 */
async function submit() {
  if (busy.value || uncertain.value || !challenge.value) return;
  busy.value = true;
  error.value = '';
  try {
    result.value = await request<components['schemas']['Feedback']>('/api/v1/feedback', {
      method: 'POST',
      body: JSON.stringify({
        ...form,
        challenge_id: challenge.value.id,
        challenge_answer: answer.value,
      }),
    });
    Object.assign(form, { contact_name: '', phone: '', email: '', order_no: '', message: '' });
    challenge.value = undefined;
    answer.value = '';
  } catch (e) {
    if (!(e instanceof RequestError) || e.status >= 500) {
      uncertain.value = true;
      error.value = '反馈结果尚未确认，请勿重复提交。可联系运营核对；此页不会自动重试。';
    } else {
      error.value =
        e.status === 429
          ? '提交过于频繁，请一小时后再试。'
          : e.detail.code === 'CHALLENGE_INVALID'
            ? '验证码无效，请重新输入。'
            : '提交被拒绝，请检查字段或重新获取验证码。';
      if (e.status !== 429) await refresh();
    }
  } finally {
    busy.value = false;
  }
}
watch(
  () => form.message,
  (value) => {
    const points = Array.from(value);
    if (points.length > 200) form.message = points.slice(0, 200).join('');
  },
);
onMounted(refresh);
</script>

<template>
  <section class="flow-page auth">
    <h1>售后反馈</h1>
    <p>请描述遇到的问题，最多 200 个字。填写订单号仅供核对，不会授予订单访问权限。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <section v-if="result" role="status">
      <h2>反馈已收到</h2>
      <p>反馈编号：{{ result.id }}。处理结果由运营人员核实，不自动发送通知。</p>
      <RouterLink to="/orders/query">查询订单</RouterLink>
    </section>
    <form v-else @submit.prevent="submit">
      <fieldset :disabled="busy || uncertain">
        <legend>联系信息与问题</legend>
        <label for="feedback-name">联系人</label
        ><input
          id="feedback-name"
          v-model="form.contact_name"
          required
          maxlength="40"
          autocomplete="name"
        />
        <label for="feedback-phone">手机</label
        ><input
          id="feedback-phone"
          v-model="form.phone"
          type="tel"
          pattern="1[3-9][0-9]{9}"
          maxlength="11"
          required
          autocomplete="tel"
        />
        <label for="feedback-email">邮箱（可选）</label
        ><input
          id="feedback-email"
          v-model="form.email"
          type="email"
          maxlength="100"
          autocomplete="email"
        />
        <label for="feedback-order">订单号（可选）</label
        ><input id="feedback-order" v-model="form.order_no" maxlength="40" autocomplete="off" />
        <label for="feedback-message">问题描述</label
        ><textarea
          id="feedback-message"
          v-model="form.message"
          required
          maxlength="400"
          rows="5"
          aria-describedby="feedback-count"
        /><small id="feedback-count">{{ Array.from(form.message).length }} / 200 字</small>
        <img v-if="challenge" :src="challenge.image_data_url" alt="四位数字验证码" />
        <button type="button" @click="refresh">换一张验证码</button>
        <label for="feedback-answer">验证码</label
        ><input
          id="feedback-answer"
          v-model="answer"
          required
          minlength="4"
          maxlength="8"
          inputmode="numeric"
          autocomplete="off"
        />
        <button type="submit" :disabled="!challenge">{{ busy ? '提交中…' : '提交反馈' }}</button>
      </fieldset>
    </form>
    <RouterLink to="/pages/contact">联系运营</RouterLink>
  </section>
</template>
