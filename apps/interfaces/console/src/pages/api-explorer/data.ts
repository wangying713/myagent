export interface ProjectApi {
  id: string;
  title: string;
  method: 'GET' | 'POST' | 'HEAD';
  path: string;
  description: string;
  defaultBody?: string;
}

/** 与 Go router.go 中实际注册的接口保持一致，参数示例独立于演示页面。 */
export const projectApis: ProjectApi[] = [
  {
    id: 'health',
    title: '健康检查',
    method: 'GET',
    path: '/api/health',
    description: '检查后台服务是否正常运行，无请求参数。',
  },
  {
    id: 'chat',
    title: '聊天接口',
    method: 'POST',
    path: '/api/agent/chat',
    description: 'question 为可选字符串；省略时使用后台默认问题。',
    defaultBody: JSON.stringify(
      { question: '用一句话解释什么是递归。' },
      null,
      2,
    ),
  },
  {
    id: 'messages',
    title: '消息历史接口',
    method: 'POST',
    path: '/api/agent/messages',
    description: 'questions 为两个非空问题的数组；省略时使用后台默认问题。',
    defaultBody: JSON.stringify(
      { questions: ['请记住我给一只小猫取名叫豆包。', '小猫叫什么名字？'] },
      null,
      2,
    ),
  },
  {
    id: 'run',
    title: 'Agent 运行接口',
    method: 'POST',
    path: '/api/agent/run',
    description: 'question 为可选字符串；允许模型在运行预算内调用乘法工具。',
    defaultBody: JSON.stringify(
      { question: '每盒有 17 支笔，买 23 盒，一共多少支？' },
      null,
      2,
    ),
  },
  {
    id: 'health-head',
    title: '健康检查（仅响应头）',
    method: 'HEAD',
    path: '/api/health',
    description: '检查后台服务，只返回响应头，无请求体和响应正文。',
  },
];

export interface ApiResult {
  status: number;
  body: string;
  traceID: string;
  elapsedMS: number;
}
