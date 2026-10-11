<script setup lang="ts">
import {ref,onMounted} from 'vue'
import {request,RequestError} from '../../../api/client'
import {useSession} from '../../../stores/session'
import type {components} from '../../../api/generated/schema'
import {ElTable,ElTableColumn,ElButton,ElInput,ElSelect,ElOption,ElTag} from 'element-plus'
import 'element-plus/dist/index.css'
const items=ref<components['schemas']['Product'][]>([]),next=ref(''),q=ref(''),status=ref(''),busy=ref(false),error=ref(''),store=useSession()
async function load(after=''){busy.value=true;error.value='';try{const params=new URLSearchParams({limit:'20',q:q.value,status:status.value});if(after)params.set('after_id',after);const result=await request<components['schemas']['ProductList']>('/api/v1/admin/products?'+params);items.value=result.items;next.value=result.next_after_id??''}catch(e){error.value=e instanceof RequestError&&e.status===403?'没有商品读取权限':'加载失败，请重试'}finally{busy.value=false}}
onMounted(()=>load())
</script>
<template><main class="admin-page"><div class="page-title"><h1>商品管理</h1><RouterLink to="/admin">工作台</RouterLink><RouterLink v-if="store.session?.permissions.includes('product.write')" to="/admin/products/new">新增商品</RouterLink></div><form class="filters" @submit.prevent="load()"><ElInput v-model="q" placeholder="商品名称或编码" aria-label="搜索商品" maxlength="100"/><ElSelect v-model="status" placeholder="全部状态" aria-label="商品状态"><ElOption label="全部状态" value=""/><ElOption label="草稿" value="DRAFT"/><ElOption label="已上架" value="ENABLED"/><ElOption label="已下架" value="DISABLED"/></ElSelect><ElButton native-type="submit" :loading="busy">查询</ElButton></form><p v-if="error" role="alert">{{error}}</p><ElTable :data="items" empty-text="暂无商品"><ElTableColumn prop="id" label="商品 ID" width="100"/><ElTableColumn prop="code" label="编码"/><ElTableColumn prop="name" label="名称"/><ElTableColumn label="状态"><template #default="{row}"><ElTag>{{row.status==='ENABLED'?'已上架':row.status==='DISABLED'?'已下架':'草稿'}}</ElTag></template></ElTableColumn><ElTableColumn prop="version" label="版本" width="80"/><ElTableColumn label="操作"><template #default="{row}"><RouterLink :to="'/admin/products/'+row.id+'/edit'">查看 / 编辑</RouterLink></template></ElTableColumn></ElTable><ElButton v-if="next" :disabled="busy" @click="load(next)">下一页</ElButton><ElButton @click="load()">刷新 / 返回首页</ElButton></main></template>
