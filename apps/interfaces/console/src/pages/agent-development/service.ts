import type { ApiResult } from '../api-explorer/data';
import { sendApi, validateBody } from '../api-explorer/service';
import type { AgentDemo } from './data';

export { validateBody };

export interface StreamEvent {
  code: string;
  msg: string;
  traceID: string;
  data: {
    delta: string;
    done: boolean;
    execution: unknown;
  };
}

function parseStreamEvent(text: string): StreamEvent {
  const value: unknown = JSON.parse(text);
  if (
    !value ||
    typeof value !== 'object' ||
    !('code' in value) ||
    typeof value.code !== 'string' ||
    !('msg' in value) ||
    typeof value.msg !== 'string' ||
    !('traceID' in value) ||
    typeof value.traceID !== 'string' ||
    !('data' in value) ||
    !value.data ||
    typeof value.data !== 'object' ||
    !('delta' in value.data) ||
    typeof value.data.delta !== 'string' ||
    !('done' in value.data) ||
    typeof value.data.done !== 'boolean'
  ) {
    throw new Error('流式事件格式不正确，已停止接收。');
  }
  return value as StreamEvent;
}

/** 普通请求复用 API 页的请求入口，保留原始参数和错误响应。 */
export function sendDemo(
  demo: AgentDemo,
  body: string,
  signal: AbortSignal,
): Promise<ApiResult> {
  return sendApi({ ...demo, method: 'POST' }, body, signal);
}

/** 浏览器 Axios 使用 XHR，不能逐段读响应；仅 SSE 使用同源 fetch 和相同的取消信号。 */
export async function streamDemo(
  demo: AgentDemo,
  body: string,
  signal: AbortSignal,
  onEvent: (event: StreamEvent) => void,
): Promise<ApiResult> {
  const started = performance.now();
  const response = await fetch(demo.path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    body,
    signal,
  });
  if (
    !response.ok ||
    !response.headers.get('content-type')?.startsWith('text/event-stream')
  ) {
    return {
      status: response.status,
      body: await response.text(),
      traceID: '',
      elapsedMS: Math.round(performance.now() - started),
    };
  }
  if (!response.body) {
    throw new Error('浏览器没有提供可读取的响应流。');
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder('utf-8', { fatal: true });
  let buffer = '';
  let eventLines: string[] = [];
  let size = 0;
  try {
    for (;;) {
      const chunk = await reader.read();
      size += chunk.value?.byteLength ?? 0;
      if (size > 4 * 1024 * 1024) {
        throw new Error('响应超过 4 MiB，已停止接收。');
      }
      buffer += decoder.decode(chunk.value, { stream: !chunk.done });
      let end = buffer.indexOf('\n');
      while (end >= 0) {
        const line = buffer.slice(0, end).replace(/\r$/, '');
        buffer = buffer.slice(end + 1);
        if (!line && eventLines.length) {
          const raw = eventLines.join('\n');
          eventLines = [];
          const event = parseStreamEvent(raw);
          onEvent(event);
          if (event.data.done) {
            return {
              status: response.status,
              body: raw,
              traceID: event.traceID,
              elapsedMS: Math.round(performance.now() - started),
            };
          }
          if (event.code !== 'ok') {
            throw new Error(event.msg);
          }
        } else if (line.startsWith('data:')) {
          eventLines.push(line.slice(5).replace(/^ /, ''));
        }
        end = buffer.indexOf('\n');
      }
      if (chunk.done) {
        throw new Error('连接提前结束，回答未完成；已收到的内容仍然保留。');
      }
    }
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
}
