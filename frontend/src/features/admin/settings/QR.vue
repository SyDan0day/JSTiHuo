<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';
const entry = ref('/verify'),
  canvas = ref<HTMLCanvasElement>(),
  error = ref(''),
  image = ref('');
const entries = [
  { path: '/', label: '首页' },
  { path: '/verify', label: '卡券鉴真入口' },
  { path: '/products', label: '公开商品目录' },
  { path: '/orders/query', label: '订单查询入口' },
  { path: '/pages/guide', label: '提货须知' },
];
const url = computed(
  () =>
    new URL(
      entries.find((item) => item.path === entry.value)?.path ?? '/verify',
      window.location.origin,
    ).href,
);
let generation = 0;
/**
 * render 用途：在浏览器本地编码白名单公开入口二维码。
 * 参数：无，来源仅为当前站点与固定路径，不支持输入卡密或任意 URL。
 * 返回值：绘制 Promise，生成 PNG 下载链接，不发网络写请求。
 */
async function render() {
  const current = ++generation;
  error.value = '';
  image.value = '';
  try {
    const rendered = document.createElement('canvas');
    await QRCode.toCanvas(rendered, url.value, {
      width: 280,
      margin: 4,
      errorCorrectionLevel: 'M',
    });
    if (current !== generation || !canvas.value) return;
    canvas.value.width = rendered.width;
    canvas.value.height = rendered.height;
    const context = canvas.value.getContext('2d');
    if (!context) throw new Error('canvas unavailable');
    context.drawImage(rendered, 0, 0);
    image.value = canvas.value.toDataURL('image/png');
  } catch {
    if (current === generation) error.value = '二维码生成失败，请重试。';
  }
}
watch(url, render);
onMounted(render);
</script>
<template>
  <main class="admin-page">
    <RouterLink to="/admin">返回工作台</RouterLink>
    <h1>公开入口二维码</h1>
    <p>只编码当前站点的公开入口，不包含卡密、令牌、个人资料或管理员会话。</p>
    <label for="qr-entry">入口页面</label
    ><select id="qr-entry" v-model="entry">
      <option v-for="item in entries" :key="item.path" :value="item.path">{{ item.label }}</option>
    </select>
    <p>{{ url }}</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <canvas ref="canvas" role="img" aria-label="公开入口二维码">{{ url }}</canvas>
    <p><a v-if="image" :href="image" download="public-entry-qr.png">下载公开入口二维码 PNG</a></p>
    <button @click="render">重新生成二维码</button>
  </main>
</template>
<style scoped>
main {
  overflow-wrap: anywhere;
}
canvas {
  display: block;
  max-width: 100%;
  height: auto;
}
select {
  max-width: 100%;
}
</style>
