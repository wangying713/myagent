import { request } from '@umijs/max';
import type { Lesson, RunResponse } from './data';

/** 调用 Go 三课接口；业务失败也读取结构化正文，保留已完成的步骤。 */
export async function runLesson(
  lesson: Lesson,
  questions: string[],
  signal: AbortSignal,
): Promise<RunResponse> {
  const response = await request<RunResponse>(`/api/demos/${lesson}`, {
    method: 'POST',
    baseURL: '',
    data: lesson === 'messages' ? { questions } : { question: questions[0] },
    signal,
    timeout: 0,
    skipErrorHandler: true,
    validateStatus: (status) => status >= 200 && status < 600,
  });
  if (!response.data || typeof response.data.completed !== 'boolean') {
    throw new Error('后台返回格式无效，请检查 Go 服务和接口代理');
  }
  return {
    ...response,
    data: { ...response.data, steps: response.data.steps ?? [] },
  };
}
