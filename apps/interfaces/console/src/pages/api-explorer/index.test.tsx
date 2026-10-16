import { act, fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import ApiExplorer from './index';
import { sendApi } from './service';

type ApiResult = import('./data').ApiResult;

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children: unknown }) => children,
}));
jest.mock('./service', () => ({
  ...jest.requireActual('./service'),
  sendApi: jest.fn(),
}));
const mockSend = jest.mocked(sendApi);

it('切换接口取消旧请求、保留参数草稿，迟到响应不会覆盖当前结果', async () => {
  let finishOld: ((result: ApiResult) => void) | undefined;
  mockSend.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finishOld = resolve;
      }),
  );
  mockSend.mockResolvedValueOnce({
    status: 200,
    body: '{"data":{"status":"ok"}}',
    traceID: '',
    elapsedMS: 1,
  });
  render(<ApiExplorer />);
  fireEvent.click(screen.getByRole('button', { name: /POST.*聊天接口/ }));
  fireEvent.change(screen.getByLabelText('请求参数（JSON）'), {
    target: { value: '{"question":"修改后的问题"}' },
  });
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  const oldSignal = mockSend.mock.calls[0][2];
  expect(mockSend.mock.calls[0][1]).toBe('{"question":"修改后的问题"}');
  fireEvent.click(screen.getByRole('button', { name: /GET.*健康检查/ }));
  expect(oldSignal.aborted).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: /发送请求/ }));
  await screen.findByText('{"data":{"status":"ok"}}');
  await act(async () => {
    finishOld?.({
      status: 200,
      body: '迟到的旧结果',
      traceID: '',
      elapsedMS: 10,
    });
  });
  expect(screen.queryByText('迟到的旧结果')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: /POST.*聊天接口/ }));
  expect(
    (screen.getByLabelText('请求参数（JSON）') as HTMLTextAreaElement).value,
  ).toBe('{"question":"修改后的问题"}');
});
