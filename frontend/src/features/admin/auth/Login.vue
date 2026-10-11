<script setup lang="ts">
import {ref,onMounted} from 'vue'
import {useRouter} from 'vue-router'
import {request,setCsrf,RequestError} from '../../../api/client'
import type {components} from '../../../api/generated/schema'
import {useSession} from '../../../stores/session'
const router=useRouter(),store=useSession();const name=ref(''),password=ref(''),answer=ref(''),challenge=ref<components['schemas']['Challenge']>(),busy=ref(false),error=ref('')
async function refresh(){try{const pre=await request<components['schemas']['Session']>('/api/v1/session/bootstrap');setCsrf(pre.csrf_token);challenge.value=await request('/api/v1/challenges',{method:'POST',body:JSON.stringify({purpose:'LOGIN'})});answer.value=''}catch{error.value='验证码加载失败，请重试'}}
async function login(){busy.value=true;error.value='';try{if(!challenge.value)return;const session=await request<components['schemas']['Session']>('/api/v1/admin/session',{method:'POST',body:JSON.stringify({login_name:name.value,password:password.value,challenge_id:challenge.value.id,challenge_answer:answer.value})});store.accept(session);await router.replace(session.force_password_change?'/admin/change-password':'/admin')}catch(e){error.value=e instanceof RequestError&&e.status===429?'尝试过于频繁，请稍后再试':'登录失败，请检查凭据和验证码';await refresh()}finally{password.value='';busy.value=false}}
onMounted(refresh)
</script>
<template><main class="auth"><h1>管理员登录</h1><form @submit.prevent="login"><label for="login">账号</label><input id="login" v-model="name" autocomplete="username" required minlength="3" maxlength="64"><label for="password">密码</label><input id="password" v-model="password" type="password" autocomplete="current-password" required minlength="12" maxlength="128"><label for="challenge">图片验证码</label><img v-if="challenge" :src="challenge.image_data_url" alt="四位数字验证码"><button type="button" :disabled="busy" @click="refresh">换一张</button><input id="challenge" v-model="answer" inputmode="numeric" autocomplete="off" required minlength="4" maxlength="8"><p role="alert">{{error}}</p><button :disabled="busy||!challenge">{{busy?'登录中…':'登录'}}</button></form></main></template>
