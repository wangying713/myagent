import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { App } from 'antd';
import React from 'react';
import { login } from '@/services/ant-design-pro/api';
import Login from './index';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    id,
  }: {
    defaultMessage?: string;
    id: string;
  }) => defaultMessage ?? id,
  Helmet: () => null,
  SelectLang: () => null,
  useIntl: () => ({
    formatMessage: ({
      defaultMessage,
      id,
    }: {
      defaultMessage?: string;
      id: string;
    }) => defaultMessage ?? id,
  }),
  useModel: jest.fn(),
}));

jest.mock('@/services/ant-design-pro/api', () => ({ login: jest.fn() }));
jest.mock('@/services/ant-design-pro/login', () => ({
  getFakeCaptcha: jest.fn(),
}));

describe('登录页', () => {
  const fetchUserInfo = jest.fn();
  const setInitialState = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
    jest.mocked(useModel).mockReturnValue({
      initialState: { fetchUserInfo },
      setInitialState,
    });
    fetchUserInfo.mockResolvedValue({ name: '测试用户' });
  });

  it('展示账号密码表单', () => {
    render(React.createElement(App, null, React.createElement(Login)));
    expect(screen.getByText('Ant Design')).toBeTruthy();
    expect(screen.getByPlaceholderText('用户名: admin or user')).toBeTruthy();
    expect(screen.getByPlaceholderText('密码: ant.design')).toBeTruthy();
  });

  it('提交账号并在成功后加载当前用户', async () => {
    jest.mocked(login).mockResolvedValue({ status: 'ok', type: 'account' });
    render(React.createElement(App, null, React.createElement(Login)));
    fireEvent.change(screen.getByPlaceholderText('用户名: admin or user'), {
      target: { value: 'admin' },
    });
    fireEvent.change(screen.getByPlaceholderText('密码: ant.design'), {
      target: { value: 'ant.design' },
    });
    fireEvent.click(screen.getByRole('button', { name: /登.*录|submit/ }));
    await waitFor(() =>
      expect(login).toHaveBeenCalledWith({
        username: 'admin',
        password: 'ant.design',
        autoLogin: true,
        type: 'account',
      }),
    );
    await waitFor(() => expect(fetchUserInfo).toHaveBeenCalledTimes(1));
    expect(setInitialState).toHaveBeenCalledTimes(1);
  });

  it('登录失败时保留表单并显示错误', async () => {
    jest.mocked(login).mockResolvedValue({ status: 'error', type: 'account' });
    render(React.createElement(App, null, React.createElement(Login)));
    fireEvent.change(screen.getByPlaceholderText('用户名: admin or user'), {
      target: { value: 'admin' },
    });
    fireEvent.change(screen.getByPlaceholderText('密码: ant.design'), {
      target: { value: 'wrong-password' },
    });
    fireEvent.click(screen.getByRole('button', { name: /登.*录|submit/ }));
    expect(
      await screen.findByText('账户或密码错误(admin/ant.design)'),
    ).toBeTruthy();
    expect(fetchUserInfo).not.toHaveBeenCalled();
    expect(setInitialState).not.toHaveBeenCalled();
  });
});
