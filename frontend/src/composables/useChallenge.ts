import { ref } from 'vue';
import { request } from '../api/client';
import type { components } from '../api/generated/schema';

/**
 * useChallenge 用途：管理公共鉴真和查询页面的一次性图形挑战。
 * 参数：purpose 为服务端校验的挑战用途。
 * 返回值：挑战、答案、加载和错误状态，以及刷新方法。
 */
export function useChallenge(purpose: 'VOUCHER' | 'ORDER_QUERY') {
  const challenge = ref<components['schemas']['Challenge']>(),
    answer = ref(''),
    loading = ref(false),
    error = ref('');

  /**
   * refresh 用途：串行获取新挑战，刷新开始即废弃旧图片与答案。
   * 参数：preserveError 为是否保留引发自动刷新的业务错误。
   * 返回值：刷新完成的 Promise；失败显示重试提示且不保留旧挑战。
   */
  async function refresh(preserveError = false) {
    if (loading.value) return;
    loading.value = true;
    challenge.value = undefined;
    answer.value = '';
    if (!preserveError) error.value = '';
    try {
      await request('/api/v1/session/bootstrap');
      challenge.value = await request('/api/v1/challenges', {
        method: 'POST',
        body: JSON.stringify({ purpose }),
      });
    } catch {
      const message = '验证码加载失败，请重试';
      error.value = error.value ? `${error.value}；${message}` : message;
    } finally {
      loading.value = false;
    }
  }
  return { challenge, answer, loading, error, refresh };
}
