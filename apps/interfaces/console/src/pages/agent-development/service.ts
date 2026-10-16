import { request } from '@umijs/max';
import type { Lesson, RunRsp } from './data';

/** 调用 Agent 业务接口；业务失败也读取结构化正文，保留已完成的步骤。 */
export async function runLesson(
  lesson: Lesson,
  questions: string[],
  signal: AbortSignal,
): Promise<RunRsp> {
  const paths: Record<Lesson, string> = {
    chat: '/api/agent/chat',
    messages: '/api/agent/messages',
    agent: '/api/agent/run',
  };
  const rsp = await request<RunRsp>(paths[lesson], {
    method: 'POST',
    baseURL: '',
    data: lesson === 'messages' ? { questions } : { question: questions[0] },
    signal,
    timeout: 0,
    skipErrorHandler: true,
    validateStatus: (status) => status >= 200 && status < 600,
  });
  if (
    typeof rsp.code !== 'string' ||
    typeof rsp.msg !== 'string' ||
    typeof rsp.traceID !== 'string' ||
    (rsp.data !== null && typeof rsp.data?.completed !== 'boolean') ||
    (rsp.code === 'ok' && rsp.data === null)
  ) {
    throw new Error('后台返回格式无效，请检查 Go 服务和接口代理');
  }
  return {
    ...rsp,
    data: rsp.data ? { ...rsp.data, steps: rsp.data.steps ?? [] } : null,
  };
}
