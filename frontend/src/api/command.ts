import { ref } from 'vue';
import { request, RequestError } from './client';

/**
 * useCommand 用途：保存一次写命令快照，供结果不确定时由用户同键重试。
 * 参数：无。
 * 返回值：发送函数、忙碌状态、待确认命令及可见错误，仅存于组件内存。
 */
export function useCommand() {
  const busy = ref(false),
    error = ref('');
  const pending = ref<{ path: string; body: string; key: string; method: string }>();
  /**
   * send 用途：执行当前命令，未确认命令存在时只重放原请求。
   * 参数：path 为固定 API；body 为完整命令；method 为写入方法。
   * 返回值：成功对象或 undefined；确定失败清理快照，网络及服务错误保留。
   */
  async function send<T>(path: string, body: unknown, method = 'POST'): Promise<T | undefined> {
    if (busy.value) return;
    pending.value ??= { path, body: JSON.stringify(body), key: crypto.randomUUID(), method };
    busy.value = true;
    error.value = '';
    try {
      const command = pending.value;
      const result = await request<T>(
        command.path,
        { method: command.method, body: command.body },
        command.key,
      );
      pending.value = undefined;
      return result;
    } catch (e) {
      if (e instanceof RequestError && e.status < 500) {
        pending.value = undefined;
        error.value =
          e.detail.code === 'REAUTH_REQUIRED'
            ? '需要先完成身份复核，再返回重试。'
            : `请求被拒绝（${e.detail.code}），请刷新核对状态、版本和权限。`;
      } else error.value = '结果尚未确认，请重试原请求；勿重复创建。';
    } finally {
      busy.value = false;
    }
  }
  return { busy, error, pending, send };
}
