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

export const plannedLessons = [
  {
    key: 'structured-output',
    title: '结构化输出',
    description: '把一句话变成 Go 能直接读取和检查的数据。',
    example: '输入“帮我买三本笔记本”，提取商品和数量，只生成数据，不实际下单。',
    steps: [
      '约定结果格式，例如 {"item":"笔记本","quantity":3}。',
      '让模型按约定返回 JSON，再用 Go 解析成结构体。',
      '检查字段是否齐全、类型是否正确、数量是否在 1～100 之间。',
      '分别展示模型原文、解析结果和失败原因，缺少信息时不猜答案。',
    ],
    exercise:
      '试试“买三本笔记本”“买一千本笔记本”和“买点笔记本”，再用离线样本检查非法 JSON、缺失字段和错误类型。',
    completion:
      '能分清“模型回答了”和“程序能用这份数据”；错误数据会被拒绝，缺少数量会提示补充。',
  },
  {
    key: 'streaming',
    title: '流式回答',
    description: '让回答边生成边显示，理解文字怎样从模型传到浏览器。',
    example: '输入“用生活中的例子解释 Go 的 context”，观察回答逐步出现。',
    steps: [
      '先用离线样本认识分段响应，再由 Go 读取并拼接完整文字。',
      '把内容逐段传给页面，区分等待首段、正在生成、完成和中断。',
      '点击停止时，让浏览器和 Go 后台一起取消本次请求。',
      '保留已收到的文字，遇到中断标记为未完成，并对比首段耗时和总耗时。',
    ],
    exercise:
      '正常生成一次，再中途停止一次；用离线样本模拟断线，检查残留文字和完成状态。',
    completion:
      '页面逐步显示文字；停止后不再继续追加；断线和取消不会显示为成功。先完成文本流式，思考模式与流式工具调用留作后续练习。',
  },
  {
    key: 'multiple-tools',
    title: '多个工具协作',
    description: '从只有乘法工具，进阶到模型按任务选择和组合工具。',
    example:
      '输入“每盒 17 支笔，买 4 盒，一共几支、多少钱？”，先查本地示例单价，再计算数量和总价。',
    steps: [
      '保留乘法工具，增加一个读取本地示例价格的工具，不接真实交易。',
      '写清每个工具能做什么、需要什么参数，让模型决定调用顺序。',
      '复用现有循环，逐个执行调用，并把每个结果交回模型。',
      '观察查价后再计算的过程；未知商品和错误参数要返回明确原因。',
    ],
    exercise:
      '分别问单纯乘法、查询单价、组合计算，再查询一个不存在的商品，比较实际调用记录。',
    completion:
      '能从记录看出选了哪个工具、传了什么参数、结果如何用于下一步；继续遵守取消和调用次数限制，不要求每次调用顺序完全一致。',
  },
];
