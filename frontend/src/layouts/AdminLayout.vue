<script setup lang="ts">
import {ref} from 'vue'
import {useRouter} from 'vue-router'
import {useSession} from '../stores/session'
import {request} from '../api/client'
const store=useSession(),router=useRouter(),error=ref('');async function logout(){try{await request('/api/v1/admin/session',{method:'DELETE'});store.clear();router.replace('/admin/login')}catch{error.value='退出失败，请重试'}}
</script>
<template><header><strong>JSTiHuo · 管理后台</strong><nav><RouterLink to="/admin/security">身份复核</RouterLink><RouterLink to="/admin/change-password">修改密码</RouterLink><button @click="logout">退出</button></nav></header><main><h1>管理工作台</h1><p>已通过管理员身份验证。可从商品管理维护目录。</p><p role="alert">{{error}}</p><RouterLink v-if="store.session?.permissions.includes('product.read')" to="/admin/products">商品管理</RouterLink><p><RouterLink v-if="store.session?.permissions.includes('inventory.read')" to="/admin/inventory">库存管理与审核</RouterLink></p><p>当前账号具有 {{store.session?.permissions.length??0}} 项权限。</p></main></template>
