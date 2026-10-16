export type Lesson = 'chat' | 'messages' | 'agent';

export interface Usage {
  prompt_tokens: number | null;
  completion_tokens: number | null;
  total_tokens: number | null;
}

export interface Step {
  kind: 'model' | 'tool';
  name: string;
  request: string;
  response: string;
  elapsed_ms: number;
  finish_reason?: string;
  usage?: Usage;
  error?: string;
}

export interface RunResult {
  lesson: Lesson;
  completed: boolean;
  answer: string;
  telemetry_enabled: boolean;
  elapsed_ms: number;
  model_calls: number;
  tool_calls: number;
  known_tokens: number;
  usage_complete: boolean;
  steps: Step[];
}

export interface Rsp<T> {
  code: string;
  data: T;
  traceID: string;
  msg: string;
}

export type RunRsp = Rsp<RunResult | null>;

export const lessons: Record<
  Lesson,
  { title: string; description: string; questions: string[] }
> = {
  chat: {
    title: '普通聊天',
    description:
      '发送 system 和 user 消息，查看实际 HTTP 请求、模型响应和最终回答。',
    questions: ['用一句话解释什么是递归。'],
  },
  messages: {
    title: '消息历史',
    description: '一次执行两轮。第二次请求显式携带第一轮的用户问题和模型回答。',
    questions: [
      '我给一只虚构小猫取名叫豆包，请记住。',
      '我给小猫取了什么名字？',
    ],
  },
  agent: {
    title: '工具调用循环',
    description:
      '模型提出 multiply 调用，Go 校验参数并执行乘法，回填结果后继续请求模型。',
    questions: ['每盒有 17 支笔，买 23 盒，一共多少支？'],
  },
};
