import { PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Descriptions,
  Empty,
  Form,
  Input,
  Row,
  Space,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useRef, useState } from 'react';
import type { Lesson, RunRsp } from './data';
import { lessons } from './data';
import { runLesson } from './service';

const LESSON_ORDER: Lesson[] = ['chat', 'messages', 'agent'];

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

export default function AgentDevelopment() {
  const [lesson, setLesson] = useState<Lesson>('chat');
  const [questions, setQuestions] = useState<Record<Lesson, string[]>>({
    chat: [...lessons.chat.questions],
    messages: [...lessons.messages.questions],
    agent: [...lessons.agent.questions],
  });
  const [result, setResult] = useState<RunRsp>();
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

  const handleRun = async (target: Lesson) => {
    if (controller.current) {
      return;
    }
    const current = new AbortController();
    controller.current = current;
    setLesson(target);
    setRunning(true);
    setResult(undefined);
    setNotice('');
    try {
      const response = await runLesson(
        target,
        questions[target],
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

  const data = result?.data;
  return (
    <PageContainer
      title="Agent 开发"
      content="从普通聊天到工具调用循环，点击任一演示，查看 Go 接口的真实执行结果。"
    >
      <Space orientation="vertical" size="large" style={{ width: '100%' }}>
        <Row gutter={[24, 24]}>
          {LESSON_ORDER.map((key, index) => (
            <Col key={key} xs={24} lg={8} style={{ display: 'flex' }}>
              <Card
                title={`${index + 1}. ${lessons[key].title}`}
                style={{
                  width: '100%',
                  display: 'flex',
                  flexDirection: 'column',
                }}
                styles={{
                  body: { display: 'flex', flexDirection: 'column', flex: 1 },
                }}
              >
                <Typography.Paragraph type="secondary">
                  {lessons[key].description}
                </Typography.Paragraph>
                <Form layout="vertical" style={{ flex: 1 }}>
                  {questions[key].map((question, position) => (
                    <Form.Item
                      key={
                        position === 0 ? 'first-question' : 'second-question'
                      }
                      label={
                        key === 'messages'
                          ? `第 ${position + 1} 轮问题`
                          : '问题'
                      }
                      htmlFor={`${key}-question-${position}`}
                    >
                      <Input.TextArea
                        id={`${key}-question-${position}`}
                        value={question}
                        disabled={running}
                        maxLength={4096}
                        showCount
                        autoSize={{ minRows: 2, maxRows: 6 }}
                        onChange={(event) => {
                          const value = event.target.value;
                          setQuestions((previous) => ({
                            ...previous,
                            [key]: previous[key].map((text, questionIndex) =>
                              questionIndex === position ? value : text,
                            ),
                          }));
                        }}
                      />
                    </Form.Item>
                  ))}
                </Form>
                <Button
                  type="primary"
                  block
                  icon={<PlayCircleOutlined />}
                  loading={running && lesson === key}
                  disabled={
                    running ||
                    questions[key].some((question) => !question.trim())
                  }
                  onClick={() => handleRun(key)}
                >
                  运行{lessons[key].title}
                </Button>
              </Card>
            </Col>
          ))}
        </Row>
        {running && (
          <Alert
            type="info"
            showIcon
            title={`正在运行：${lessons[lesson].title}`}
            description="正在等待后台完成模型或工具调用。取消后可重新运行任一演示。"
            action={
              <Button icon={<StopOutlined />} onClick={handleCancel}>
                取消本次执行
              </Button>
            }
          />
        )}
        {notice && <Alert type="info" title={notice} showIcon />}
        {result && result.code !== 'ok' && (
          <Alert
            type="error"
            title={result.msg}
            description={
              <>
                错误码：{result.code}。
                {result.data
                  ? '下方保留已完成的执行记录。'
                  : '本次请求未产生执行结果。'}
                {!result.data && result.traceID && (
                  <Typography.Text copyable>{result.traceID}</Typography.Text>
                )}
              </>
            }
            showIcon
          />
        )}
        {data ? (
          <>
            <Card
              title={`${lessons[lesson].title} · 执行结果`}
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
                    children: result?.traceID ? (
                      <Typography.Text copyable>
                        {result.traceID}
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
              <Empty description="点击上方任一运行按钮，查看回答与执行过程" />
            </Card>
          )
        )}
      </Space>
    </PageContainer>
  );
}
