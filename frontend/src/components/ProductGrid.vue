<script setup lang="ts">
import type { components } from '../api/generated/schema';
defineProps<{ products: components['schemas']['ProductPublic'][] }>();
</script>

<template>
  <div class="product-grid">
    <RouterLink
      v-for="(product, index) in products"
      :key="product.id"
      :to="'/products/' + product.id"
      class="product-card"
    >
      <img
        v-if="product.image_urls[0]"
        :src="product.image_urls[0]"
        :alt="product.name"
        :loading="index === 0 ? 'eager' : 'lazy'"
      />
      <div v-else class="product-placeholder" aria-hidden="true">礼</div>
      <h2>{{ product.name }}</h2>
      <span>查看礼品 <span aria-hidden="true">↗</span></span>
    </RouterLink>
  </div>
</template>
