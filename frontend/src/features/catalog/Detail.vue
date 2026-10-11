<script setup lang="ts">
import {ref,onMounted} from 'vue'
import {useRoute} from 'vue-router'
import {request,RequestError} from '../../api/client'
import type {components} from '../../api/generated/schema'
const route=useRoute(),product=ref<components['schemas']['ProductPublic']>(),error=ref('')
async function load(){error.value='';try{product.value=await request('/api/v1/products/'+route.params.id)}catch(e){error.value=e instanceof RequestError&&e.status===404?'商品不存在或已下架':'商品读取失败，请重试'}}
onMounted(load)
</script>
<template><header><RouterLink to="/products">返回礼品目录</RouterLink></header><main><p v-if="error" role="alert">{{error}} <button @click="load">重试</button></p><template v-else-if="product"><h1>{{product.name}}</h1><div class="detail-images"><img v-for="src in product.image_urls" :key="src" :src="src" :alt="product.name"></div><dl><template v-for="(a,i) in product.attributes" :key="i"><dt>{{a.name}}</dt><dd>{{a.value}}</dd></template></dl><article v-html="product.description_html"/></template><p v-else role="status">加载中…</p></main></template>
