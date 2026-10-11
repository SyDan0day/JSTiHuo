<script setup lang="ts">
import {ref} from 'vue'
import {useRouter} from 'vue-router'
import {request,RequestError} from '../../../api/client'
import {useSession} from '../../../stores/session'
const current=ref(''),next=ref(''),confirm=ref(''),error=ref(''),busy=ref(false),router=useRouter(),store=useSession();let pending:{key:string;body:string}|null=null
async function submit(){if(!pending){if(next.value!==confirm.value){error.value='两次新密码不一致';return}pending={key:crypto.randomUUID(),body:JSON.stringify({current_password:current.value,new_password:next.value})}}busy.value=true;error.value='';try{await request('/api/v1/admin/session/password',{method:'POST',body:pending.body},pending.key);pending=null;store.clear();await router.replace('/admin/login')}catch(e){if(e instanceof RequestError&&e.status<500){pending=null;error.value='修改失败，请重新确认当前密码或登录状态'}else{error.value='结果待确认，请点击重试；将使用原请求和原幂等键。'}}finally{busy.value=false;current.value='';next.value='';confirm.value=''}}
</script>
<template><main class="auth"><h1>修改密码</h1><p>首次登录需修改临时密码。修改成功后重新登录。</p><form @submit.prevent="submit"><label for="current">当前密码</label><input id="current" v-model="current" type="password" autocomplete="current-password" minlength="12" maxlength="128" required><label for="next">新密码</label><input id="next" v-model="next" type="password" autocomplete="new-password" minlength="12" maxlength="128" required><label for="confirm">确认新密码</label><input id="confirm" v-model="confirm" type="password" autocomplete="new-password" minlength="12" maxlength="128" required><p role="alert">{{error}}</p><button v-if="error.includes('结果待确认')" type="button" :disabled="busy" @click="submit">使用原请求重试</button><button :disabled="busy">{{busy?'提交中…':'保存并重新登录'}}</button></form></main></template>
