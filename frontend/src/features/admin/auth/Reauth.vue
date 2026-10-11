<script setup lang="ts">
import {ref} from 'vue'
import {request,RequestError} from '../../../api/client'
import type {components} from '../../../api/generated/schema'
import {useSession} from '../../../stores/session'
const password=ref(''),busy=ref(false),message=ref(''),store=useSession()
async function submit(){busy.value=true;message.value='';try{const session=await request<components['schemas']['Session']>('/api/v1/admin/session/reauth',{method:'POST',body:JSON.stringify({password:password.value})});store.accept(session);message.value='密码复核成功，有效期五分钟。'}catch(e){message.value=e instanceof RequestError&&e.status===429?'尝试过于频繁，请稍后再试':'复核失败，请检查密码或重新登录'}finally{password.value='';busy.value=false}}
</script>
<template><main class="auth"><h1>敏感操作复核</h1><form @submit.prevent="submit"><label for="reauth-password">当前密码</label><input id="reauth-password" v-model="password" type="password" autocomplete="current-password" required minlength="12" maxlength="128"><p role="status">{{message}}</p><button :disabled="busy">{{busy?'复核中…':'确认身份'}}</button></form><RouterLink to="/admin">返回工作台</RouterLink></main></template>
