<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { request } from '../../api/client';
import type { components } from '../../api/generated/schema';
import PublicLayout from '../../layouts/PublicLayout.vue';
import ProductGrid from '../../components/ProductGrid.vue';
import PublicIcon from '../../components/PublicIcon.vue';

const products = ref<components['schemas']['ProductPublic'][]>([]);
const content = ref<components['schemas']['Content'][]>([]);
const loading = ref(false),
  productError = ref(''),
  contentError = ref('');
const bannerIndex = ref(0);
const banners = computed(() =>
  content.value.filter((c) => c.kind === 'BANNER').sort((a, b) => a.sort_order - b.sort_order),
);
const banner = computed(() => banners.value[bannerIndex.value]);
const announcements = computed(() =>
  content.value
    .filter((c) => c.kind === 'ANNOUNCEMENT')
    .sort((a, b) => a.sort_order - b.sort_order)
    .slice(0, 4),
);

/**
 * load 用途：分别读取首页公开内容和礼品，单项失败不隐藏另一项。
 * 参数：无。
 * 返回值：完成加载的 Promise，各区域显示实际空态或失败状态。
 */
async function load() {
  loading.value = true;
  productError.value = contentError.value = '';
  const results = await Promise.allSettled([
    request<components['schemas']['ProductPublicList']>('/api/v1/products?limit=8'),
    request<components['schemas']['ContentList']>('/api/v1/content?limit=100'),
  ]);
  const [gifts, pages] = results;
  if (gifts.status === 'fulfilled') products.value = gifts.value.items;
  else productError.value = '礼品加载失败，请重试。';
  if (pages.status === 'fulfilled') {
    content.value = pages.value.items;
    bannerIndex.value = 0;
  } else contentError.value = '公告加载失败，请重试。';
  loading.value = false;
}

/**
 * changeBanner 用途：由用户手动切换首页已发布横幅。
 * 参数：step 前后移动的偏移量。
 * 返回值：无，不自动轮播，避免打断阅读。
 */
function changeBanner(step: number) {
  if (banners.value.length)
    bannerIndex.value = (bannerIndex.value + step + banners.value.length) % banners.value.length;
}
onMounted(load);
</script>

<template>
  <PublicLayout>
    <section class="public-hero" aria-labelledby="home-title">
      <div class="hero-art" aria-hidden="true">
        <div class="orbit orbit-one"></div>
        <div class="orbit orbit-two"></div>
        <span class="art-coordinate coordinate-left">GIFT / CONNECTED</span>
        <span class="art-coordinate coordinate-right">A LITTLE THOUGHT. A LOT OF JOY.</span>
        <div class="digital-gift">
          <span class="gift-corner corner-top"></span
          ><span class="gift-corner corner-bottom"></span>
          <PublicIcon name="gift" />
          <span class="gift-scan"></span>
        </div>
        <span class="art-node node-left"></span><span class="art-node node-right"></span>
      </div>
      <p class="eyebrow hero-eyebrow"><span class="status-dot"></span> 心意在线 · 美好即将抵达</p>
      <h1 id="home-title">美好心意，<span>从这里启程。</span></h1>
      <p class="hero-description">
        让每一份礼遇，轻松抵达。<br class="mobile-break" />凭礼品卡甄选心仪好物，预约专属配送时间。
      </p>
      <div class="public-actions hero-actions">
        <RouterLink class="public-action" to="/verify"
          >开始鉴真与提货 <PublicIcon name="arrow"
        /></RouterLink>
        <RouterLink class="public-action secondary-action" to="/orders/query">
          <PublicIcon name="truck" /> 查询订单进度
        </RouterLink>
      </div>
      <p class="hero-note"><PublicIcon name="shield" /> 凭卡核验权益，无需注册账号</p>
    </section>

    <nav class="service-shortcuts" aria-label="服务快捷入口">
      <RouterLink to="/verify">
        <PublicIcon name="shield" />
        <div><strong>卡券鉴真</strong><span>核验卡券，安心开启礼遇</span></div>
        <span class="shortcut-number">01</span><PublicIcon class="shortcut-arrow" name="arrow" />
      </RouterLink>
      <RouterLink to="/redeem">
        <PublicIcon name="gift" />
        <div><strong>预约提货</strong><span>甄选好礼，自主预约日期</span></div>
        <span class="shortcut-number">02</span><PublicIcon class="shortcut-arrow" name="arrow" />
      </RouterLink>
      <RouterLink to="/orders/query">
        <PublicIcon name="truck" />
        <div><strong>订单进度</strong><span>随时查询，心意有迹可循</span></div>
        <span class="shortcut-number">03</span><PublicIcon class="shortcut-arrow" name="arrow" />
      </RouterLink>
    </nav>

    <section v-if="banner" class="content-banner" aria-label="服务推荐">
      <div>
        <p class="eyebrow">WHAT’S NEW / 服务资讯</p>
        <h2>{{ banner.title }}</h2>
      </div>
      <RouterLink :to="'/pages/' + banner.slug">阅读详情 <PublicIcon name="arrow" /></RouterLink>
      <div v-if="banners.length > 1" class="banner-controls">
        <button type="button" aria-label="上一条推荐" @click="changeBanner(-1)">←</button>
        <span role="status">{{ bannerIndex + 1 }} / {{ banners.length }}</span>
        <button type="button" aria-label="下一条推荐" @click="changeBanner(1)">→</button>
      </div>
    </section>

    <section class="home-gifts" aria-labelledby="gifts-title" :aria-busy="loading">
      <div class="public-section-title">
        <div>
          <p class="eyebrow">CURATED FOR YOU / 精选礼遇</p>
          <h2 id="gifts-title">发现一份喜欢<span class="accent">。</span></h2>
        </div>
        <RouterLink class="text-link" to="/products"
          >探索全部礼品 <PublicIcon name="arrow"
        /></RouterLink>
      </div>
      <p class="section-description">
        用一份恰好的礼物，回应每一份用心。可提货礼品以卡券权益为准。
      </p>
      <p v-if="productError" role="alert">
        {{ productError }}<button :disabled="loading" @click="load">重试</button>
      </p>
      <p v-else-if="loading && !products.length" class="public-empty" role="status">
        正在加载礼品…
      </p>
      <p v-else-if="!products.length" class="public-empty">礼品正在准备中，请稍后再来看看。</p>
      <ProductGrid :products="products" />
    </section>

    <section class="how-it-works" aria-labelledby="steps-title">
      <div class="public-section-title">
        <div>
          <p class="eyebrow">HOW IT WORKS / 提货流程</p>
          <h2 id="steps-title">简单几步，静待好礼。</h2>
        </div>
        <RouterLink class="text-link" to="/pages/guide"
          >完整提货须知 <PublicIcon name="arrow"
        /></RouterLink>
      </div>
      <ol class="process-grid">
        <li>
          <span class="process-number">01</span><PublicIcon name="shield" />
          <h3>验证您的礼品卡</h3>
          <p>输入卡号与卡密，核验卡券状态及可兑换权益。</p>
        </li>
        <li>
          <span class="process-number">02</span><PublicIcon name="gift" />
          <h3>挑选心仪礼品</h3>
          <p>在卡券对应的礼品中，选一份属于自己的喜欢。</p>
        </li>
        <li>
          <span class="process-number">03</span><PublicIcon name="box" />
          <h3>预约收货信息</h3>
          <p>填写收货地址，选择系统开放的发货日期。</p>
        </li>
        <li>
          <span class="process-number">04</span><PublicIcon name="truck" />
          <h3>确认并等待送达</h3>
          <p>核对信息并提交，随时查看订单与物流进度。</p>
        </li>
      </ol>
    </section>

    <section class="home-support" aria-labelledby="help-title">
      <div class="support-intro">
        <p class="eyebrow">HERE TO HELP / 提货帮助</p>
        <h2 id="help-title">关于提货，<br />您可能想了解。</h2>
        <RouterLink class="text-link" to="/pages/contact"
          >联系我们 <PublicIcon name="arrow"
        /></RouterLink>
      </div>
      <div class="faq-list">
        <details>
          <summary>如何查看礼品卡可以兑换哪些商品？<span aria-hidden="true">+</span></summary>
          <p>
            点击“开始鉴真与提货”，输入卡号、卡密和验证码。卡券验证通过且符合提货条件后，即可查看对应的礼品与可选数量。
          </p>
        </details>
        <details>
          <summary>可以选择发货日期吗？<span aria-hidden="true">+</span></summary>
          <p>
            可以。选好礼品并填写收货信息后，可选择系统开放的预约发货日期。可用日期与名额以提交时的系统校验为准。
          </p>
        </details>
        <details>
          <summary>提交后，在哪里查看订单和物流？<span aria-hidden="true">+</span></summary>
          <p>
            前往“订单查询”，使用卡号或订单号、收货手机号及验证码查询。发货后，订单页面可查看物流信息。
          </p>
        </details>
        <details>
          <summary>提货遇到问题，如何获取帮助？<span aria-hidden="true">+</span></summary>
          <p>
            您可以通过“联系我们”查看服务信息，或在“售后反馈”中提交问题。请妥善保管卡密，勿在反馈内容中填写。
          </p>
          <RouterLink to="/feedback">提交售后反馈 →</RouterLink>
        </details>
      </div>
    </section>

    <section class="public-announcements" aria-labelledby="news-title" :aria-busy="loading">
      <div class="public-section-title">
        <h2 id="news-title">服务公告</h2>
        <RouterLink class="text-link" to="/pages/notice"
          >全部公告 <PublicIcon name="arrow"
        /></RouterLink>
      </div>
      <p v-if="contentError" role="alert">
        {{ contentError }}<button :disabled="loading" @click="load">重试</button>
      </p>
      <p v-else-if="loading && !content.length" role="status">正在加载公告…</p>
      <ul v-else-if="announcements.length">
        <li v-for="item in announcements" :key="item.id">
          <RouterLink :to="'/pages/' + item.slug"
            >{{ item.title }} <PublicIcon name="arrow"
          /></RouterLink>
        </li>
      </ul>
      <p v-else class="muted">暂无新公告。</p>
    </section>

    <section class="closing-cta" aria-labelledby="ready-title">
      <p class="eyebrow">A GIFT, JUST FOR YOU</p>
      <h2 id="ready-title">让期待，成为此刻的美好。</h2>
      <RouterLink class="public-action" to="/verify"
        >开启您的礼遇 <PublicIcon name="arrow"
      /></RouterLink>
    </section>
  </PublicLayout>
</template>
