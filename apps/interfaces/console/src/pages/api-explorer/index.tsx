import { ReloadOutlined, SendOutlined, StopOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  Row,
  Space,
  Tag,
  Typography,
} from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import type { ApiResult, ProjectApi } from './data';
import { projectApis } from './data';
import { sendApi, validateBody } from './service';

export default function ApiExplorer() {
  const [api, setApi] = useState(projectApis[0]);
  const [drafts, setDrafts] = useState<Record<string, string>>(() =>
    Object.fromEntries(
      projectApis.map((item) => [item.id, item.defaultBody ?? '']),
    ),
  );
  const [result, setResult] = useState<ApiResult>();
  const [notice, setNotice] = useState('');
  const [running, setRunning] = useState(false);
  const controller = useRef<AbortController | null>(null);
  const body = drafts[api.id];

  useEffect(
    () => () => {
      controller.current?.abort();
      controller.current = null;
    },
    [],
  );

  const handleSelect = (target: ProjectApi) => {
    if (target.id === api.id) {
      return;
    }
    controller.current?.abort();
    controller.current = null;
    setRunning(false);
    setApi(target);
    setResult(undefined);
    setNotice('');
  };

  const handleSend = async () => {
    if (controller.current) {
      return;
    }
    const invalid = api.method === 'POST' ? validateBody(body) : '';
    if (invalid) {
      setNotice(invalid);
      return;
    }
    const current = new AbortController();
    controller.current = current;
    setRunning(true);
    setNotice('');
    setResult(undefined);
    try {
      const response = await sendApi(api, body, current.signal);
      if (controller.current === current) {
        setResult(response);
      }
    } catch {
      if (controller.current === current) {
        setNotice('请求未能完成，请检查后台服务和网络连接。');
      }
    } finally {
      if (controller.current === current) {
        controller.current = null;
        setRunning(false);
      }
    }
  };

  const handleCancel = () => {
    controller.current?.abort();
    controller.current = null;
    setRunning(false);
    setNotice('已取消本次请求。');
  };

  return (
    <PageContainer
      title="API"
      content="选择项目接口，编辑请求参数并发送，查看实际 HTTP 响应。"
    >
      <Row gutter={[24, 24]}>
        <Col xs={24} lg={7} xl={6}>
          <Card title={`接口列表 · ${projectApis.length}`}>
            <Space
              orientation="vertical"
              size="middle"
              style={{ width: '100%' }}
            >
              {projectApis.map((item) => (
                <Button
                  key={item.id}
                  block
                  type={item.id === api.id ? 'primary' : 'default'}
                  aria-pressed={item.id === api.id}
                  onClick={() => handleSelect(item)}
                  style={{
                    height: 'auto',
                    padding: 12,
                    textAlign: 'left',
                    display: 'block',
                  }}
                >
                  <span style={{ display: 'block', marginBottom: 6 }}>
                    <Tag color={item.method === 'POST' ? 'blue' : 'green'}>
                      {item.method}
                    </Tag>
                    {item.title}
                  </span>
                  <span
                    style={{
                      display: 'block',
                      fontFamily: 'monospace',
                      whiteSpace: 'normal',
                      overflowWrap: 'anywhere',
                    }}
                  >
                    {item.path}
                  </span>
                </Button>
              ))}
            </Space>
          </Card>
        </Col>
        <Col xs={24} lg={17} xl={18}>
          <Space orientation="vertical" size="large" style={{ width: '100%' }}>
            <Card title={api.title}>
              <Space wrap style={{ marginBottom: 16 }}>
                <Tag color={api.method === 'POST' ? 'blue' : 'green'}>
                  {api.method}
                </Tag>
                <Typography.Text code copyable>
                  {api.path}
                </Typography.Text>
              </Space>
              <Typography.Paragraph type="secondary">
                {api.description}
              </Typography.Paragraph>
              {api.method === 'POST' ? (
                <Form layout="vertical">
                  <Form.Item
                    label="请求参数（JSON）"
                    htmlFor="api-request-body"
                  >
                    <Input.TextArea
                      id="api-request-body"
                      value={body}
                      disabled={running}
                      spellCheck={false}
                      autoSize={{ minRows: 9, maxRows: 20 }}
                      style={{ fontFamily: 'monospace' }}
                      onChange={(event) => {
                        const value = event.target.value;
                        setDrafts((previous) => ({
                          ...previous,
                          [api.id]: value,
                        }));
                        setNotice('');
                      }}
                    />
                  </Form.Item>
                </Form>
              ) : (
                <Typography.Paragraph>
                  此接口无需请求参数。
                </Typography.Paragraph>
              )}
              <Space wrap>
                <Button
                  type="primary"
                  icon={<SendOutlined />}
                  loading={running}
                  onClick={handleSend}
                >
                  发送请求
                </Button>
                {api.method === 'POST' && (
                  <Button
                    icon={<ReloadOutlined />}
                    disabled={running}
                    onClick={() => {
                      setDrafts((previous) => ({
                        ...previous,
                        [api.id]: api.defaultBody ?? '',
                      }));
                      setNotice('');
                    }}
                  >
                    恢复默认参数
                  </Button>
                )}
                {running && (
                  <Button icon={<StopOutlined />} onClick={handleCancel}>
                    取消请求
                  </Button>
                )}
              </Space>
            </Card>
            {notice && <Alert type="warning" title={notice} showIcon />}
            <Card
              title="响应结果"
              extra={
                result && (
                  <Space>
                    <Tag color={result.status < 400 ? 'success' : 'error'}>
                      HTTP {result.status}
                    </Tag>
                    <Typography.Text type="secondary">
                      {result.elapsedMS} ms
                    </Typography.Text>
                  </Space>
                )
              }
            >
              {result ? (
                <Space
                  orientation="vertical"
                  size="middle"
                  style={{ width: '100%' }}
                >
                  {result.traceID && (
                    <Typography.Text copyable={{ text: result.traceID }}>
                      Trace ID：{result.traceID}
                    </Typography.Text>
                  )}
                  {result.body ? (
                    <>
                      <Typography.Text copyable={{ text: result.body }}>
                        复制响应 JSON
                      </Typography.Text>
                      <pre
                        style={{
                          margin: 0,
                          maxHeight: 560,
                          overflow: 'auto',
                          whiteSpace: 'pre-wrap',
                          overflowWrap: 'anywhere',
                          fontFamily: 'monospace',
                        }}
                      >
                        {result.body}
                      </pre>
                    </>
                  ) : (
                    <Empty description="此响应没有正文" />
                  )}
                </Space>
              ) : (
                <Empty
                  description={
                    running ? '正在等待接口响应…' : '发送请求后，在这里查看响应'
                  }
                />
              )}
            </Card>
          </Space>
        </Col>
      </Row>
    </PageContainer>
  );
}
