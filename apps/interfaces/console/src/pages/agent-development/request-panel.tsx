import { Alert, Button, Input, Space, Tag, Typography } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import type { ApiResult } from '../api-explorer/data';
import { formatResponseJSON } from '../api-explorer/format';
import type { AgentDemo } from './data';
import { sendDemo, streamDemo, validateBody } from './service';

export default function RequestPanel({ demo }: { demo: AgentDemo }) {
  const [body, setBody] = useState(demo.defaultBody);
  const [result, setResult] = useState<ApiResult>();
  const [text, setText] = useState('');
  const [error, setError] = useState('');
  const [running, setRunning] = useState(false);
  const active = useRef<AbortController | null>(null);

  useEffect(
    () => () => {
      const controller = active.current;
      active.current = null;
      controller?.abort();
    },
    [],
  );

  async function send() {
    const failure = validateBody(body);
    if (failure) {
      setError(failure);
      return;
    }
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setRunning(true);
    setError('');
    setText('');
    setResult(undefined);
    try {
      const response = demo.streaming
        ? await streamDemo(demo, body, controller.signal, (event) => {
            if (active.current !== controller || controller.signal.aborted)
              return;
            setText((current) => current + event.data.delta);
            if (event.code !== 'ok') setError(event.msg);
          })
        : await sendDemo(demo, body, controller.signal);
      if (active.current === controller && !controller.signal.aborted) {
        setResult(response);
        try {
          const envelope = JSON.parse(response.body);
          if (envelope.code !== 'ok')
            setError(envelope.msg || '本次执行未完成，请查看结果。');
        } catch {
          setError('响应不是 JSON，请查看原始结果。');
        }
      }
    } catch (failure) {
      if (active.current === controller) {
        setError(
          controller.signal.aborted
            ? '已停止请求；已收到的内容仍然保留。'
            : failure instanceof Error
              ? failure.message
              : '请求失败，请检查后台服务。',
        );
      }
    } finally {
      if (active.current === controller) {
        active.current = null;
        setRunning(false);
      }
    }
  }

  return (
    <section className="agent-demo-request" aria-label="运行 Demo">
      <Typography.Title level={3}>动手试一下</Typography.Title>
      <Typography.Paragraph>{demo.description}</Typography.Paragraph>
      <Space wrap>
        <Tag color="blue">POST</Tag>
        <Typography.Text code copyable>
          {demo.path}
        </Typography.Text>
      </Space>
      <label className="agent-demo-label" htmlFor="demo-request">
        请求参数（JSON）
      </label>
      <Input.TextArea
        id="demo-request"
        value={body}
        disabled={running}
        onChange={(event) => setBody(event.target.value)}
        autoSize={{ minRows: 5, maxRows: 14 }}
        spellCheck={false}
      />
      <Space className="agent-demo-actions" wrap>
        <Button type="primary" loading={running} onClick={send}>
          发送请求
        </Button>
        <Button disabled={!running} onClick={() => active.current?.abort()}>
          停止请求
        </Button>
        <Button
          disabled={running}
          onClick={() => {
            setBody(demo.defaultBody);
            setError('');
          }}
        >
          恢复默认参数
        </Button>
      </Space>
      {error && <Alert type="warning" title={error} showIcon />}
      {demo.streaming && (running || text) && (
        <div aria-live="polite" className="agent-demo-stream">
          <Typography.Text strong>实时回答</Typography.Text>
          <pre data-testid="stream-answer">{text || '等待模型生成…'}</pre>
        </div>
      )}
      {result && (
        <div className="agent-demo-result">
          <Space wrap>
            <Tag color={result.status < 400 ? 'green' : 'orange'}>
              HTTP {result.status}
            </Tag>
            <span>{result.elapsedMS} ms</span>
            {result.traceID && (
              <Typography.Text copyable>{result.traceID}</Typography.Text>
            )}
          </Space>
          <Typography.Title level={4}>执行结果</Typography.Title>
          <Typography.Paragraph type="secondary">
            execution 是运行情况，steps
            是每一步的输入输出；失败时也会保留已经完成的部分。
          </Typography.Paragraph>
          <Typography.Text copyable={{ text: result.body }}>
            复制原始响应
          </Typography.Text>
          <pre data-testid="demo-result">
            {formatResponseJSON(result.body, { expandNested: true })}
          </pre>
        </div>
      )}
    </section>
  );
}
