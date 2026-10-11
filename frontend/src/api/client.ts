import type {components} from './generated/schema'
type ApiError=components['schemas']['Error']
export class RequestError extends Error{constructor(readonly status:number,readonly detail:ApiError){super(detail.message)}}
// CSRF is held only in memory. A user action supplies its stable idempotency key.
let csrf=''
export function setCsrf(value:string){csrf=value}
export async function request<T>(path:string,options:RequestInit={},idempotencyKey?:string):Promise<T>{
 if(!path.startsWith('/api/v1/'))throw new Error('Invalid API path')
 const headers=new Headers(options.headers);headers.set('Accept','application/json')
 if(options.body && !(options.body instanceof FormData))headers.set('Content-Type','application/json')
 if(!['GET','HEAD'].includes((options.method??'GET').toUpperCase())){headers.set('X-CSRF-Token',csrf);if(idempotencyKey)headers.set('Idempotency-Key',idempotencyKey)}
 const response=await fetch(path,{...options,headers,credentials:'same-origin'})
 if(!response.ok)throw new RequestError(response.status,await response.json())
 return response.status===204?undefined as T:response.json()
}
