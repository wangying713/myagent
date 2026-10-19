import { act, fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { agentDemos } from './data';
import RequestPanel from './request-panel';
import { sendDemo, streamDemo } from './service';

type ApiResult = import('../api-explorer/data').ApiResult;

jest.mock('./service', () => ({
  ...jest.requireActual('./service'),
  sendDemo: jest.fn(),
  streamDemo: jest.fn(),
}));
const mockSend = jest.mocked(sendDemo);
const mockStream = jest.mocked(streamDemo);
beforeEach(() => jest.clearAllMocks());

it('编辑后的参数原样发送，失败时展示部分步骤', async () => {
  mockSend.mockResolvedValueOnce({
    status: 409,
    body: '{"code":"budget_exceeded","msg":"达到运行预算","data":{"execution":{"completed":false,"steps":[{"name":"multiply"}]}}}',
    traceID: 'demo-trace',
    elapsedMS: 10,
  });
  render(React.createElement(RequestPanel, { demo: agentDemos[7] }));
  fireEvent.change(screen.getByLabelText('请求参数（JSON）'), {
    target: { value: '{"model_calls":1}' },
  });
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  expect(mockSend.mock.calls[0][1]).toBe('{"model_calls":1}');
  expect((await screen.findByTestId('demo-result')).textContent).toContain(
    'multiply',
  );
  expect(screen.getByText('达到运行预算')).toBeTruthy();
});

it('无效 JSON 不发送请求，恢复默认找回示例', () => {
  render(React.createElement(RequestPanel, { demo: agentDemos[1] }));
  fireEvent.change(screen.getByLabelText('请求参数（JSON）'), {
    target: { value: '{' },
  });
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  expect(mockSend).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '恢复默认参数' }));
  expect(
    (screen.getByLabelText('请求参数（JSON）') as HTMLTextAreaElement).value,
  ).toBe(agentDemos[1].defaultBody);
});

it('切换章节取消旧请求，迟到结果不覆盖新章节', async () => {
  let finishOld: ((value: ApiResult) => void) | undefined;
  mockSend.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finishOld = resolve;
      }),
  );
  const view = render(
    React.createElement(RequestPanel, { key: 'chat', demo: agentDemos[1] }),
  );
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  const signal = mockSend.mock.calls[0][2];
  view.rerender(
    React.createElement(RequestPanel, { key: 'tools', demo: agentDemos[3] }),
  );
  expect(signal.aborted).toBe(true);
  await act(async () => {
    finishOld?.({ status: 200, body: '迟到旧结果', traceID: '', elapsedMS: 1 });
  });
  expect(screen.queryByText('迟到旧结果')).toBeNull();
});

it('分段内容立即显示，停止后保留已收到的文字', async () => {
  mockStream.mockImplementationOnce(
    (_demo, _body, signal, onEvent) =>
      new Promise((_resolve, reject) => {
        onEvent({
          code: 'ok',
          msg: '成功',
          traceID: '',
          data: { delta: '已经生成的部分', done: false, execution: null },
        });
        signal.addEventListener('abort', () => reject(new Error('aborted')), {
          once: true,
        });
      }),
  );
  render(React.createElement(RequestPanel, { demo: agentDemos[6] }));
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  expect((await screen.findByTestId('stream-answer')).textContent).toBe(
    '已经生成的部分',
  );
  fireEvent.click(screen.getByRole('button', { name: '停止请求' }));
  expect(
    await screen.findByText('已停止请求；已收到的内容仍然保留。'),
  ).toBeTruthy();
  expect(screen.getByTestId('stream-answer').textContent).toBe(
    '已经生成的部分',
  );
});
