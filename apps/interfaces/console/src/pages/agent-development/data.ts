export interface AgentDemo {
  id: string;
  title: string;
  document: string;
  path: string;
  description: string;
  defaultBody: string;
  sourceTitle: string;
  sourceURL: string;
  projectExample: boolean;
  streaming: boolean;
}

/** 章节顺序与学习路线一致，接口均由 Go router.go 显式注册。 */
export const agentDemos: AgentDemo[] = [
  {
    id: 'overview',
    title: 'Agent 到底是什么',
    document: 'learning/00-overview.md',
    path: '/api/agent-demos/overview',
    description:
      '同一问题分别请求普通聊天和工具循环，对比“模型说了什么”和“程序实际做了什么”。',
    defaultBody: '{\n  "question": "每盒有 17 支笔，买 23 盒，一共多少支？"\n}',
    sourceTitle: '工具调用原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/tool_calls',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'chat',
    title: '普通聊天',
    document: 'learning/01-chat.md',
    path: '/api/agent-demos/chat',
    description: '发送一次模型请求，查看回答、请求内容和耗时。',
    defaultBody: '{\n  "question": "用一句话解释什么是递归。"\n}',
    sourceTitle: '首次调用原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/',
    projectExample: false,
    streaming: false,
  },
  {
    id: 'messages',
    title: '消息历史',
    document: 'learning/02-messages.md',
    path: '/api/agent-demos/messages',
    description:
      '第二次提问会携带第一轮的真实回答；历史只属于本次请求，不会保存到其他会话。',
    defaultBody:
      '{\n  "questions": [\n    "请记住我给一只小猫取名叫豆包。",\n    "小猫叫什么名字？"\n  ]\n}',
    sourceTitle: '多轮对话原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/multi_round_chat',
    projectExample: false,
    streaming: false,
  },
  {
    id: 'tools',
    title: '工具调用与循环',
    document: 'learning/03-agent.md',
    path: '/api/agent-demos/tools',
    description: '模型提出乘法调用，Go 检查并执行，再把结果交回模型。',
    defaultBody: '{\n  "question": "每盒有 17 支笔，买 23 盒，一共多少支？"\n}',
    sourceTitle: '工具调用原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/tool_calls',
    projectExample: false,
    streaming: false,
  },
  {
    id: 'structured',
    title: '结构化输出',
    document: 'learning/04-json.md',
    path: '/api/agent-demos/structured',
    description:
      '模型返回商品和数量的 JSON；Go 检查字段、重复键和数量范围（1～100），本例不会下单。',
    defaultBody: '{\n  "question": "帮我买三本笔记本。"\n}',
    sourceTitle: 'JSON 输出原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/json_mode',
    projectExample: false,
    streaming: false,
  },
  {
    id: 'multiple-tools',
    title: '多个工具协作',
    document: 'learning/multiple-tools.md',
    path: '/api/agent-demos/multiple-tools',
    description:
      '模型可以查询本地商品资料，再调用乘法计算总价。内置笔记本单价 23 元。',
    defaultBody: '{\n  "question": "买 4 本笔记本，一共多少钱？"\n}',
    sourceTitle: '工具调用原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/tool_calls',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'stream',
    title: '流式回答',
    document: 'learning/05-stream-thinking.md',
    path: '/api/agent-demos/stream',
    description:
      '模型生成一段就显示一段；停止会取消后台请求，已收到的内容仍然保留。',
    defaultBody:
      '{\n  "question": "用生活中的例子解释 Go 的 context，分三点说明。"\n}',
    sourceTitle: '流式协议原文',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/api/create-chat-completion',
    projectExample: false,
    streaming: true,
  },
  {
    id: 'reliability',
    title: '失败与执行边界',
    document: 'learning/06-reliability.md',
    path: '/api/agent-demos/reliability',
    description:
      '限制本次模型调用次数。默认只允许一次，工具执行后若还需要模型回答，会返回预算不足及已有步骤。',
    defaultBody:
      '{\n  "question": "每盒有 17 支笔，买 23 盒，一共多少支？",\n  "model_calls": 1\n}',
    sourceTitle: 'context 取消与超时原文',
    sourceURL: 'https://go.dev/blog/context',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'retrieval',
    title: '检索与记忆',
    document: 'learning/07-retrieval-memory.md',
    path: '/api/agent-demos/retrieval',
    description:
      '先在内置资料里按关键词查找，再交给模型回答并附资料编号。没有命中就明确说明，不调用模型。',
    defaultBody: '{\n  "question": "笔记本缺页可以换货吗？"\n}',
    sourceTitle: '多轮消息协议参考',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/multi_round_chat',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'workflow',
    title: '工作流',
    document: 'learning/09-workflows.md',
    path: '/api/agent-demos/workflow',
    description:
      '按拆分、检查、汇总三个固定步骤运行，后一步使用前一步的真实结果。',
    defaultBody:
      '{\n  "question": "我想做一个可以查商品资料和计算总价的小助手。"\n}',
    sourceTitle: '工具调用协议参考',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/tool_calls',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'observation',
    title: '日志与观测',
    document: 'learning/openobserve.md',
    path: '/api/agent-demos/observation',
    description:
      '运行一次工具任务，查看每步耗时、输入输出和 TraceID。是否上报链路由现有服务配置决定。',
    defaultBody: '{\n  "question": "每盒有 17 支笔，买 23 盒，一共多少支？"\n}',
    sourceTitle: 'OpenObserve Traces 原文',
    sourceURL: 'https://openobserve.ai/docs/user-guide/traces/',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'evaluation',
    title: '评估与回归',
    document: 'learning/08-evaluation.md',
    path: '/api/agent-demos/evaluation',
    description:
      '依次运行三道固定乘法题，同时检查工具的实际结果和最终回答；结果展示逐题证据和通过数量。请求为 {}。',
    defaultBody: '{}',
    sourceTitle: 'Go 测试原文',
    sourceURL: 'https://go.dev/doc/tutorial/add-a-test',
    projectExample: true,
    streaming: false,
  },
  {
    id: 'delivery',
    title: '综合练习',
    document: 'learning/10-delivery.md',
    path: '/api/agent-demos/delivery',
    description:
      '只读商品助手：查内置商品资料、查价格、算总价并说明来源，不连接商店，不执行购买。',
    defaultBody:
      '{\n  "question": "买 4 本笔记本多少钱？保修多久？请注明资料出处。"\n}',
    sourceTitle: '工具调用协议参考',
    sourceURL: 'https://api-docs.deepseek.com/zh-cn/guides/tool_calls',
    projectExample: true,
    streaming: false,
  },
];
