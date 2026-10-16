import { request } from '@umijs/max';
import type { ApiResult, ProjectApi } from './data';

/** 只校验 JSON 语法与对象形状，字段和业务参数继续交给实际接口校验。 */
export function validateBody(body: string): string {
  try {
    const value: unknown = JSON.parse(body);
    if (value === null || typeof value !== 'object' || Array.isArray(value)) {
      return '请求参数必须是 JSON 对象，例如 {}。';
    }
  } catch {
    return 'JSON 格式有误，请检查引号、逗号和括号。';
  }
  return '';
}

function responseTraceID(body: string): string {
  try {
    const value: unknown = JSON.parse(body);
    if (
      value !== null &&
      typeof value === 'object' &&
      'traceID' in value &&
      typeof value.traceID === 'string'
    ) {
      return value.traceID;
    }
  } catch {
    // 非 JSON 响应也照常展示原文，不猜测或生成 trace ID。
  }
  return '';
}

/** 保留实际 HTTP 状态与正文，4xx/5xx 也作为调试结果展示。 */
export async function sendApi(
  api: ProjectApi,
  body: string,
  signal: AbortSignal,
): Promise<ApiResult> {
  const started = performance.now();
  const response = await request<string>(api.path, {
    method: api.method,
    baseURL: '',
    headers:
      api.method === 'POST' ? { 'Content-Type': 'application/json' } : {},
    data: api.method === 'POST' ? body : undefined,
    // 直接传递编辑后的文本，避免重新编码改变字段、重复键或大整数。
    transformRequest: [(value: string | undefined) => value],
    responseType: 'text',
    transformResponse: [(value: string) => value],
    getResponse: true,
    signal,
    timeout: 0,
    skipErrorHandler: true,
    validateStatus: (status) => status >= 200 && status < 600,
  });
  return {
    status: response.status,
    body: response.data,
    traceID: responseTraceID(response.data),
    elapsedMS: Math.round(performance.now() - started),
  };
}
