import { PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Card,
  Collapse,
  Descriptions,
  Empty,
  Form,
  Input,
  Space,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useRef, useState } from 'react';
import type { Lesson, RunResponse } from './data';
import { lessons } from './data';
import { runLesson } from './service';

function prettyJSON(text: string): string {
  if (!text) {
    return '尚未收到响应';
  }
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function ProtocolText({ text }: { text: string }) {
  return (
    <pre
      style={{
        whiteSpace: 'pre-wrap',
        overflowWrap: 'anywhere',
        maxHeight: 420,
        overflow: 'auto',
        margin: 0,
      }}
    >
      {prettyJSON(text)}
    </pre>
  );
}

export default function GoAgentDemo() {
  const [lesson, setLesson] = useState<Lesson>('chat');
  const [questions, setQuestions] = useState<Record<Lesson, string[]>>({
    chat: [...lessons.chat.questions],
    messages: [...lessons.messages.questions],
    agent: [...lessons.agent.questions],
  });
  const [result, setResult] = useState<RunResponse>();
  const [notice, setNotice] = useState('');
  const [running, setRunning] = useState(false);
  const controller = useRef<AbortController | null>(null);

  useEffect(
    () => () => {
      controller.current?.abort();
      controller.current = null;
    },
    [],
  );

  const handleRun = async () => {
    if (controller.current) {
      return;
    }
    const current = new AbortController();
    controller.current = current;
    setRunning(true);
    setResult(undefined);
    setNotice('');
    try {
      const response = await runLesson(
        lesson,
        questions[lesson],
        current.signal,
      );
      if (controller.current === current) {
        setResult(response);
      }
    } catch {
      if (controller.current === current) {
        setNotice(
          current.signal.aborted
            ? '已取消本次请求。'
            : '接口请求失败，请确认 Go 服务已启动。',
        );
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
    setNotice('已取消本次请求，后台会停止后续模型和工具调用。');
  };

  const handleLessonChange = (key: string) => {
    if (key === 'chat' || key === 'messages' || key === 'agent') {
      setLesson(key);
      setResult(undefined);
      setNotice('');
    }
  };

  const data = result?.data;
  const valid = questions[lesson].every(
    (question) => question.trim().length > 0,
  );
  return (
    <PageContainer
      title="Go Agent 三课演示"
      content="从原 Python demo 迁移：页面请求 Go 接口，展示实际执行结果。"
    >
      <Space orientation="vertical" size="large" style={{ width: '100%' }}>
        <Card>
          <Tabs
            activeKey={lesson}
            onChange={handleLessonChange}
            items={(Object.keys(lessons) as Lesson[]).map((key) => ({
              key,
              label: lessons[key].title,
              disabled: running,
            }))}
          />
          <Typography.Paragraph type="secondary">
            {lessons[lesson].description}
          </Typography.Paragraph>
          <Form layout="vertical">
            {questions[lesson].map((question, index) => (
              <Form.Item
                key={index === 0 ? 'first-question' : 'second-question'}
                label={
                  lesson === 'messages' ? `第 ${index + 1} 轮问题` : '问题'
                }
              >
                <Input.TextArea
                  value={question}
                  disabled={running}
                  maxLength={4096}
                  showCount
                  autoSize={{ minRows: 2, maxRows: 6 }}
                  onChange={(event) => {
                    const value = event.target.value;
                    setQuestions((previous) => ({
                      ...previous,
                      [lesson]: previous[lesson].map((text, position) =>
                        position === index ? value : text,
                      ),
                    }));
                  }}
                />
              </Form.Item>
            ))}
          </Form>
          <Space>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={running}
              disabled={!valid}
              onClick={handleRun}
            >
              执行演示
            </Button>
            <Button
              icon={<StopOutlined />}
              disabled={!running}
              onClick={handleCancel}
            >
              取消
            </Button>
          </Space>
        </Card>
        {notice && <Alert type="info" title={notice} showIcon />}
        {result?.error && (
          <Alert
            type="error"
            title={result.error.message}
            description={`错误码：${result.error.code}。下方保留已完成的执行记录。`}
            showIcon
          />
        )}
        {data ? (
          <>
            <Card
              title="执行结果"
              extra={
                <Tag color={data.completed ? 'success' : 'error'}>
                  {data.completed ? '已完成' : '未完成'}
                </Tag>
              }
            >
              <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>
                {data.answer || '尚未获得最终回答'}
              </Typography.Paragraph>
              <Descriptions
                column={{ xs: 1, sm: 2, lg: 3 }}
                items={[
                  {
                    key: 'elapsed',
                    label: '总耗时',
                    children: `${data.elapsed_ms} ms`,
                  },
                  {
                    key: 'models',
                    label: '模型调用',
                    children: `${data.model_calls} 次`,
                  },
                  {
                    key: 'tools',
                    label: '工具分发',
                    children: `${data.tool_calls} 次`,
                  },
                  {
                    key: 'usage',
                    label: '已知累计 token',
                    children: `${data.known_tokens}${data.usage_complete ? '' : '（部分用量未知）'}`,
                  },
                  {
                    key: 'trace',
                    label: 'Trace ID',
                    span: 2,
                    children: data.trace_id ? (
                      <Typography.Text copyable>
                        {data.trace_id}
                      </Typography.Text>
                    ) : (
                      '观测已关闭'
                    ),
                  },
                ]}
              />
              {data.telemetry_enabled && (
                <Typography.Text type="secondary">
                  Trace ID 可用于在 OpenObserve 中查询；生成 ID
                  不代表记录已确认入库。
                </Typography.Text>
              )}
            </Card>
            <Card title={`执行过程 · ${data.steps.length} 个步骤`}>
              {data.steps.length > 0 ? (
                <Collapse
                  items={data.steps.map((step, index) => ({
                    key: `${index}-${step.kind}`,
                    label: `${index + 1}. ${step.name} · ${step.elapsed_ms} ms${step.finish_reason ? ` · ${step.finish_reason}` : ''}`,
                    children: (
                      <Space orientation="vertical" style={{ width: '100%' }}>
                        {step.error && (
                          <Alert type="warning" title={step.error} />
                        )}
                        <Typography.Text strong>
                          {step.kind === 'tool'
                            ? '工具调用与参数'
                            : '发送的请求与消息历史'}
                        </Typography.Text>
                        <ProtocolText text={step.request} />
                        <Typography.Text strong>
                          {step.kind === 'tool'
                            ? '工具执行结果'
                            : '收到的模型响应（含本轮用量）'}
                        </Typography.Text>
                        <ProtocolText text={step.response} />
                      </Space>
                    ),
                  }))}
                />
              ) : (
                <Empty description="没有执行步骤" />
              )}
            </Card>
          </>
        ) : (
          !running &&
          !notice && (
            <Card>
              <Empty description="选择一课，点击执行演示查看结果" />
            </Card>
          )
        )}
      </Space>
    </PageContainer>
  );
}
