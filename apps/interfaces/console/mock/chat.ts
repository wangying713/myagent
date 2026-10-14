import type { Request, Response } from 'express';

// 保留官方 AI 助手页面，发送时只返回本机演示数据，不连接外部模型。
export default {
  'POST /api/demo/chat': (_req: Request, res: Response) => {
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    const chunk = {
      id: 'local-demo',
      object: 'chat.completion.chunk',
      choices: [{ index: 0, delta: { role: 'assistant', content: '你好！这是管理台模板的本机演示。你可以浏览分析页、表单页、查询表格和权限示例。' }, finish_reason: null }],
    };
    res.write(`data: ${JSON.stringify(chunk)}\n\n`);
    res.end('data: [DONE]\n\n');
  },
};
