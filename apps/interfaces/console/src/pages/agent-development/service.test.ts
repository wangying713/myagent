import { ReadableStream } from 'node:stream/web';
import { TextDecoder, TextEncoder } from 'node:util';
import { agentDemos } from './data';
import { streamDemo } from './service';

const previousFetch = global.fetch;
const previousDecoder = global.TextDecoder;
beforeAll(() => {
  global.TextDecoder = TextDecoder as typeof global.TextDecoder;
});
afterEach(() => {
  global.fetch = previousFetch;
});
afterAll(() => {
  global.TextDecoder = previousDecoder;
});

function response(chunks: Uint8Array[]) {
  return {
    ok: true,
    status: 200,
    headers: { get: () => 'text/event-stream; charset=utf-8' },
    body: new ReadableStream({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(chunk);
        controller.close();
      },
    }),
  } as unknown as Response;
}

it('跨字节读取中文，跨分片读取事件，最终事件保留业务错误', async () => {
  const events = [
    {
      code: 'ok',
      msg: '成功',
      traceID: 'trace',
      data: { delta: '你好', done: false, execution: null },
    },
    {
      code: 'timeout',
      msg: '超时',
      traceID: 'trace',
      data: {
        delta: '',
        done: true,
        execution: { answer: '你好', completed: false },
      },
    },
  ];
  const raw = new TextEncoder().encode(
    events.map((event) => `data: ${JSON.stringify(event)}\r\n\r\n`).join(''),
  );
  global.fetch = jest
    .fn()
    .mockResolvedValue(
      response(Array.from(raw, (byte) => new Uint8Array([byte]))),
    );
  const received: string[] = [];
  const result = await streamDemo(
    agentDemos[6],
    '{}',
    new AbortController().signal,
    (event) => received.push(event.data.delta),
  );
  expect(received).toEqual(['你好', '']);
  expect(result.traceID).toBe('trace');
  expect(JSON.parse(result.body).code).toBe('timeout');
});

it('没有最终事件时拒绝将断流当作成功，已收到的内容仍交给页面', async () => {
  const raw = new TextEncoder().encode(
    'data: {"code":"ok","msg":"成功","traceID":"","data":{"delta":"部分","done":false,"execution":null}}\n\n',
  );
  global.fetch = jest.fn().mockResolvedValue(response([raw]));
  const onEvent = jest.fn();
  await expect(
    streamDemo(agentDemos[6], '{}', new AbortController().signal, onEvent),
  ).rejects.toThrow('连接提前结束');
  expect(onEvent.mock.calls[0][0].data.delta).toBe('部分');
});

it('流建立前的参数错误保留 HTTP 状态和正文', async () => {
  global.fetch = jest.fn().mockResolvedValue({
    ok: false,
    status: 400,
    text: async () => '{"code":"invalid_input"}',
  });
  const result = await streamDemo(
    agentDemos[6],
    '{}',
    new AbortController().signal,
    jest.fn(),
  );
  expect(result.status).toBe(400);
  expect(result.body).toContain('invalid_input');
});
