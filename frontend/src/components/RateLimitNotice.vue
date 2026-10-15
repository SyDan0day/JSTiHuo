<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
const seconds = ref(0);
let deadline = 0;
let timer: ReturnType<typeof setInterval>;

/**
 * receive 用途：按服务端 Retry-After 提示倒计时，不自动重试请求。
 * 参数：event 请求客户端发出的限流事件。
 * 返回值：无，只更新全局提示。
 */
function receive(event: Event) {
  deadline = Date.now() + Number((event as CustomEvent<number>).detail) * 1000;
  tick();
}

/**
 * tick 用途：根据截止时刻更新剩余秒数。
 * 参数：无。
 * 返回值：无，不发起网络请求。
 */
function tick() {
  seconds.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
}
onMounted(() => {
  window.addEventListener('jst-rate-limit', receive);
  timer = setInterval(tick, 1000);
});
onUnmounted(() => {
  window.removeEventListener('jst-rate-limit', receive);
  clearInterval(timer);
});
</script>
<template>
  <div v-if="seconds" class="rate-limit-notice" role="status">
    请求过于频繁，请等待 {{ seconds }} 秒后再试。
  </div>
</template>
<style scoped>
.rate-limit-notice {
  padding: 12px 24px;
  background: #fff7ed;
  color: #9a3412;
  border-bottom: 1px solid #fed7aa;
}
</style>
