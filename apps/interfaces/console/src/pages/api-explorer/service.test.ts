import { request } from '@umijs/max';
import { projectApis } from './data';
import { sendApi, validateBody } from './service';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
const mockRequest = jest.mocked(request);

beforeEach(() => mockRequest.mockReset());

it('保留编辑后的原始 JSON、错误状态及响应数值精度', async () => {
  const body = '{"question":"测试","unknown":9007199254740993}';
  const response =
    '{"code":"invalid_input","data":null,"traceID":"trace-test","count":9007199254740993}';
  mockRequest.mockResolvedValueOnce({ status: 400, data: response });
  const signal = new AbortController().signal;
  const result = await sendApi(projectApis[1], body, signal);
  expect(mockRequest).toHaveBeenCalledWith(
    '/api/agent/chat',
    expect.objectContaining({
      method: 'POST',
      baseURL: '',
      data: body,
      signal,
      getResponse: true,
      responseType: 'text',
      skipErrorHandler: true,
    }),
  );
  expect(result).toEqual(
    expect.objectContaining({
      status: 400,
      body: response,
      traceID: 'trace-test',
    }),
  );
});

it('HEAD 不发送请求体并保留空响应', async () => {
  mockRequest.mockResolvedValueOnce({ status: 200, data: '' });
  const result = await sendApi(
    projectApis[4],
    '不应发送',
    new AbortController().signal,
  );
  expect(mockRequest).toHaveBeenCalledWith(
    '/api/health',
    expect.objectContaining({ method: 'HEAD', data: undefined }),
  );
  expect(result.body).toBe('');
  expect(result.traceID).toBe('');
});

it('语法检查不代替后台业务校验', () => {
  expect(validateBody('{"unknown":1}')).toBe('');
  expect(validateBody('{"question":""}')).toBe('');
  for (const invalid of ['', '{', '[]', 'null', '"text"']) {
    expect(validateBody(invalid)).not.toBe('');
  }
});
