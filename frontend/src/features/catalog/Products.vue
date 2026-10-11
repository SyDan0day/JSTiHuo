<script setup lang="ts">
import {ref,onMounted} from 'vue'
import {request} from '../../api/client'
import type {components} from '../../api/generated/schema'
const products=ref<components['schemas']['ProductPublic'][]>([]),next=ref(''),busy=ref(false),error=ref('')
async function load(after=''){busy.value=true;error.value='';try{const p=await request<components['schemas']['ProductPublicList']>('/api/v1/products?limit=20'+(after?'&after_id='+after:''));products.value=after?[...products.value,...p.items]:p.items;next.value=p.next_after_id??''}catch{error.value='商品加载失败，请重试'}finally{busy.value=false}}
onMounted(()=>load())
</script>
<template><header><strong>JSTiHuo · 礼品目录</strong><RouterLink to="/admin">管理端</RouterLink></header><main><h1>挑选一份心意</h1><p>查看礼品详情与规格。</p><p v-if="error" role="alert">{{error}} <button @click="load()">重试</button></p><p v-else-if="busy&&!products.length" role="status">加载中…</p><p v-else-if="!products.length">暂无上架商品。</p><div class="product-grid"><RouterLink v-for="p in products" :key="p.id" :to="'/products/'+p.id" class="product-card"><img :src="p.image_urls[0]" :alt="p.name" loading="lazy"><h2>{{p.name}}</h2><span>查看详情 →</span></RouterLink></div><button v-if="next" :disabled="busy" @click="load(next)">加载更多</button></main></template>
