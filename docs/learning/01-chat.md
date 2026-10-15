# 01：看懂一次模型调用

## 目标与阅读顺序

能解释请求发给谁、模型看到哪些材料，以及接口成功为什么不等于任务完成。

先看 [执行器](../../apps/demos/runner.go) 的普通聊天分支，再看 [客户端](../../infra/deepseek/client.go) 的 `Chat`，最后看 [协议](../../domain/agent/protocol.go) 的 `Response.Validate`。Go 结构体和切片保存数据，`json.Marshal` 将数据编码成 JSON，`http.Client.Do` 发送请求；通过 `error` 返回失败，使用 `defer` 释放响应 body。

## 页面请求与模型请求

管理台发送 `POST /api/demos/chat`，正文 `{}` 使用默认问题，也可发送：

```json
{"question":"用一句话解释什么是递归。"}
```

Go 服务再发送模型请求。下面是当前默认配置对应的请求示意，鉴权只放请求头：

```text
POST https://api.deepseek.com/v1/chat/completions
Authorization: Bearer <本机配置的密钥>
Content-Type: application/json
```

```json
{
  "model": "deepseek-flash",
  "messages": [
    {"role": "system", "content": "你是一个简洁的中文助手。"},
    {"role": "user", "content": "用一句话解释什么是递归。"}
  ],
  "thinking": {"type": "disabled"},
  "max_tokens": 512,
  "stream": false
}
```

模型名和地址来自配置；客户端补齐思考关闭、非流式、输出上限 512。密钥不会发送给前端，也不进入 prompt。工具描述在第三课才加入请求。

## 如何读返回结果

模型响应示意如下，文本和用量是示意值：

```json
{
  "choices": [{
    "message": {"role":"assistant","content":"递归是一个过程在解决问题时调用自身。"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens":30,"completion_tokens":15,"total_tokens":45}
}
```

客户端先检查 HTTP 状态，再解析唯一的候选响应并校验角色、正文和结束原因。`stop` 配合非空正文表示正常文本完成；`tool_calls` 交给工具循环；`length` 等其他原因报告无效或未完成，不把截断正文当完整答案。HTTP 错误与协议错误保留已收到的响应副本，方便定位。

页面的 `data.answer` 是最终回答；`data.steps` 展示本次实际模型请求和原始响应的脱敏副本；`completed` 表示程序流程完成，不能证明开放式回答事实正确。用量未返回时，页面会说明统计不完整。

## 操作与练习

打开普通聊天页签，执行默认问题。应有一次模型步骤；正常观测为一个根 span 和一个模型 span，共 2 个。

1. 在页面把“递归”改为“函数”，检查请求的 user 正文变化。
2. 在执行器中临时调整 system 提示词，重启 Go 服务，比较请求和回答后恢复。前端输入只控制问题。
3. 在客户端中临时将 `MaxTokens` 改为 1，观察结束原因和未完成状态后恢复。该字段当前固定在客户端，不是 HTTP 参数。

每次只改一个变量。可先用 [离线接口测试](../../apps/interfaces/http/api/handler_test.go) 构造截断响应，避免为验证错误反复调用真实模型。

## 通过标准

- 能从实际请求找到 URL、鉴权位置、system 和 user 消息。
- 能从响应找到回答、`finish_reason` 和 `usage`，区分 HTTP 成功、协议完成与答案正确。
- 能按 trace ID 找到两个 span，解释它们的父子关系。
- 知道模型不会自行读取本机文件，材料需要程序显式提供。

下一课：[消息历史](02-messages.md)。
