<script setup lang="ts">
import PublicLayout from './layouts/PublicLayout.vue';
import RateLimitNotice from './components/RateLimitNotice.vue';
</script>

<template>
  <RateLimitNotice />
  <RouterView v-slot="{ Component, route }">
    <PublicLayout v-if="route.meta.customer"
      ><component :is="Component" :key="route.path"
    /></PublicLayout>
    <component :is="Component" v-else :key="route.path" />
  </RouterView>
</template>
