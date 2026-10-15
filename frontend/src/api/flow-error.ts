import { RequestError } from './client';

/**
 * flowError 用途：将顾客接口错误转换为可恢复的中文操作提示。
 * 参数：error 业务或网络错误。
 * 返回值：不包含请求字段、凭证或地址的提示。
 */
export function flowError(error: unknown): string {
  if (!(error instanceof RequestError)) return '网络异常，请重试；已提交操作请先确认原请求结果。';
  if (error.status === 429) return '请求过于频繁，请按倒计时等待后再试。';
  const messages: Record<string, string> = {
    AUTH_REQUIRED: '会话已过期，请重新鉴真或查询订单。',
    SCOPE_REQUIRED: '当前会话不能执行此操作，请完成对应身份校验。',
    FORBIDDEN: '没有执行此操作的权限。',
    CSRF_INVALID: '会话校验已失效，请重新鉴真或查询后操作。',
    CHALLENGE_INVALID: '验证码无效或已使用，请换一张后重试。',
    INVALID_ARGUMENT: '输入格式不正确，请检查表单。',
    VALIDATION_FAILED: '输入不符合要求，请检查表单。',
    INVALID_CREDENTIALS: '凭证不匹配，请核对输入。',
    VOUCHER_UNAVAILABLE: '卡券当前不可兑换，请检查激活状态与有效期。',
    SELECTION_INVALID: '可选商品已变化，请重新选择符合权益规则的礼品。',
    VERSION_CONFLICT: '数据已变化，请重新读取并核对。',
    NOT_FOUND: '未找到当前授权范围内的记录。',
    ILLEGAL_TRANSITION: '当前状态或操作时间不允许执行此操作。',
  };
  return (
    messages[error.detail.code] ??
    (error.status >= 500
      ? '服务暂时不可用；写入结果可能尚未确认，请使用原请求确认。'
      : '操作未完成，请检查输入后重试。')
  );
}
