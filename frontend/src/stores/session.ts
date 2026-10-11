import {defineStore} from 'pinia'
import {ref} from 'vue'
import type {components} from '../api/generated/schema'
import {request,setCsrf} from '../api/client'
type Session=components['schemas']['Session']
export const useSession=defineStore('session',()=>{const session=ref<Session|null>(null);function accept(value:Session){session.value=value;setCsrf(value.csrf_token)};async function restore(){try{accept(await request<Session>('/api/v1/admin/session'))}catch{session.value=null;setCsrf('')}};function clear(){session.value=null;setCsrf('')};return {session,accept,restore,clear}})
