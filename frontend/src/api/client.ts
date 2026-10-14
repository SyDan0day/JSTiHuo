import type { components } from './generated/schema';
type ApiError = components['schemas']['Error'];
export class RequestError extends Error {
  /**
   * constructor 用途：保存统一 API 错误，供界面区别业务失败与网络不确定。
   * 参数：status 为 HTTP 状态，detail 为服务端结构化错误。
   * 返回值：无。
   * 约束：初始化 Error 信息，不存请求体或秘密。
   */
  constructor(
    readonly status: number,
    readonly detail: ApiError,
  ) {
    super(detail.message);
  }
}
// CSRF is held only in memory. A user action supplies its stable idempotency key.
type Scope = 'admin' | 'preauth' | 'voucher' | 'order';
let orderScope: 'READ' | 'WRITE' = 'READ';
/**
 * getOrderScope 用途：读取当前订单会话的服务端作用域提示。
 * 参数：无。
 * 返回值：READ 或 WRITE。
 * 约束：默认 READ，只从成功响应头更新，不替代服务端授权。
 */
export function getOrderScope() {
  return orderScope;
}
const tokens: Record<Scope, string> = { admin: '', preauth: '', voucher: '', order: '' };

/**
 * setCsrf 用途：按会话类型保存 CSRF。
 * 参数：value 为随机 CSRF，scope 为管理员/预认证/卡券/订单类型。
 * 返回值：无。
 * 约束：只写内存，不持久化。
 */
export function setCsrf(value: string, scope: Scope = 'admin') {
  tokens[scope] = value;
}

/**
 * scopeFor 用途：选择请求使用的会话类型。
 * 参数：path 为固定 API 路径，method 为大写 HTTP 方法。
 * 返回值：会话类型。
 * 约束：登录与挑战使用预认证。
 */
function scopeFor(path: string, method: string): Scope {
  if (
    path === '/api/v1/challenges' ||
    path === '/api/v1/voucher-sessions' ||
    path === '/api/v1/order-sessions' ||
    path === '/api/v1/feedback' ||
    (path === '/api/v1/admin/session' && method === 'POST')
  )
    return 'preauth';
  if (path.startsWith('/api/v1/admin/')) return 'admin';
  if (path.startsWith('/api/v1/order')) return 'order';
  return 'voucher';
}

/**
 * request 用途：发起同源 JSON 业务请求并接收 CSRF 轮换。
 * 参数：path 为 API 路径，options 为请求设置，idempotencyKey 为当前动作的固定键。
 * 返回值：结果 Promise。
 * 约束：非成功抛 RequestError，网络错误向调用方传播。
 * 约束：不自动重试、不持久化请求。
 */
export async function request<T>(
  path: string,
  options: RequestInit = {},
  idempotencyKey?: string,
): Promise<T> {
  if (!path.startsWith('/api/v1/')) throw new Error('Invalid API path');
  const method = (options.method ?? 'GET').toUpperCase();
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/json');
  if (options.body && !(options.body instanceof FormData))
    headers.set('Content-Type', 'application/json');
  if (!['GET', 'HEAD'].includes((options.method ?? 'GET').toUpperCase())) {
    headers.set('X-CSRF-Token', tokens[scopeFor(path, method)]);
    if (idempotencyKey) headers.set('Idempotency-Key', idempotencyKey);
  }
  const response = await fetch(path, { ...options, headers, credentials: 'same-origin' });
  if (!response.ok) throw new RequestError(response.status, await response.json());
  const orderHint = response.headers.get('X-Order-Scope');
  if (orderHint === 'READ' || orderHint === 'WRITE') orderScope = orderHint;
  const scope = response.headers.get('X-CSRF-Scope'),
    token = response.headers.get('X-CSRF-Token');
  if (
    token &&
    (scope === 'admin' || scope === 'preauth' || scope === 'voucher' || scope === 'order')
  )
    setCsrf(token, scope);
  const data = response.status === 204 ? undefined : await response.json();
  if (path === '/api/v1/session/bootstrap' && data?.csrf_token)
    setCsrf(data.csrf_token, 'preauth');
  return data as T;
}

// Credentials remain out of JSON/state: a user-triggered authorized binary download.

/**
 * download 用途：授权下载短期凭证文件。
 * 参数：path 为固定格式的文件下载路径。
 * 返回值：完成操作的 Promise，无业务返回值。
 * 约束：权限/复核/过期等响应抛 RequestError，网络错误传播。
 * 约束：只创建内存Blob URL，触发用户下载后释放 URL。
 */
export async function download(path: string) {
  if (!/^\/api\/v1\/admin\/files\/[1-9][0-9]*\/download$/.test(path))
    throw new Error('Invalid download path');
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'X-CSRF-Token': tokens.admin },
  });
  if (!response.ok) throw new RequestError(response.status, await response.json());
  const blob = await response.blob(),
    url = URL.createObjectURL(blob),
    link = document.createElement('a');
  link.href = url;
  link.download = 'credentials.csv';
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
