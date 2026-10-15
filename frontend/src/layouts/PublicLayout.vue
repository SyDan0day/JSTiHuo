<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import PublicIcon from '../components/PublicIcon.vue';
import '../styles/public.css';

const route = useRoute();
const steps = ['卡券鉴真', '选择礼品', '收货与预约', '确认提货'];
const stepRoutes = ['/verify', '/redeem/select', '/redeem/address', '/redeem/confirm'];
const activeStep = computed(() => stepRoutes.indexOf(route.path));
</script>

<template>
  <div class="public-site">
    <a class="skip-link" href="#public-main">跳到主要内容</a>
    <div class="public-notice">
      <span class="status-dot" aria-hidden="true"></span>
      一张礼卡，让心意如期而至
      <RouterLink to="/pages/guide">了解提货流程 <span aria-hidden="true">↗</span></RouterLink>
    </div>
    <header class="public-header">
      <RouterLink class="brand" to="/" aria-label="JSTiHuo 礼品提货首页">
        <span class="brand-mark"><PublicIcon name="gift" /></span>
        <span>JSTiHuo<span class="brand-divider">/</span><small>礼品提货</small></span>
      </RouterLink>
      <nav aria-label="主要导航">
        <RouterLink to="/" exact-active-class="is-current">首页</RouterLink>
        <RouterLink to="/products">礼品目录</RouterLink>
        <RouterLink to="/pages/guide">提货须知</RouterLink>
        <RouterLink to="/orders/query">订单查询</RouterLink>
      </nav>
      <RouterLink class="public-action header-action" to="/verify">
        立即提货 <PublicIcon name="arrow" />
      </RouterLink>
    </header>
    <main id="public-main" class="public-main" tabindex="-1">
      <ol v-if="activeStep >= 0" class="redemption-steps" aria-label="提货流程">
        <li
          v-for="(step, index) in steps"
          :key="step"
          :class="{
            'is-active': index === activeStep,
            'is-complete': index < activeStep,
          }"
          :aria-current="index === activeStep ? 'step' : undefined"
        >
          <span>{{ index < activeStep ? '✓' : '0' + (index + 1) }}</span
          >{{ step }}
        </li>
      </ol>
      <slot />
    </main>
    <footer class="public-footer">
      <div class="footer-main">
        <div>
          <RouterLink class="brand" to="/" aria-label="返回首页">
            <span class="brand-mark"><PublicIcon name="gift" /></span>JSTiHuo
          </RouterLink>
          <p>把心意，送到身边。</p>
        </div>
        <nav aria-label="服务信息">
          <RouterLink to="/pages/about">关于我们</RouterLink>
          <RouterLink to="/pages/contact">联系我们</RouterLink>
          <RouterLink to="/pages/notice">系统公告</RouterLink>
          <RouterLink to="/feedback">售后反馈</RouterLink>
        </nav>
      </div>
      <div class="footer-bottom">
        <small>JSTiHuo · 礼品提货服务</small>
        <small>卡券权益与预约结果以系统实际校验为准。</small>
        <RouterLink to="/admin">管理端 <span aria-hidden="true">↗</span></RouterLink>
      </div>
    </footer>
    <nav class="public-bottom-nav" aria-label="移动快捷导航">
      <RouterLink to="/" exact-active-class="is-current"><PublicIcon name="home" />首页</RouterLink>
      <RouterLink to="/verify"><PublicIcon name="shield" />鉴真</RouterLink>
      <RouterLink to="/redeem"><PublicIcon name="gift" />提货</RouterLink>
      <RouterLink to="/orders/query"><PublicIcon name="truck" />进度</RouterLink>
      <RouterLink to="/pages/contact"><PublicIcon name="help" />联系</RouterLink>
    </nav>
  </div>
</template>
