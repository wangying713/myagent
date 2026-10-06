# 01：一次模型调用到底是什么

## 本节目标

能自己构造一份请求，并从返回的 JSON 找到回答。先不用理解所有底层文件，先阅读 `demos/beginner/beginner_01_chat.py`，再找到 `llm.py` 中的 `chat()`。

先会这几个 Python 概念就够了：字典 `{'role': 'user'}` 存键值；列表 `[]` 保存有顺序的消息；函数接收参数并返回结果；`with` 在结束时释放资源；`try/except` 处理错误。JSON 是跨语言交换数据的文本格式，Python 字典不是 JSON 文本，HTTP 客户端负责序列化。

## 一个请求有四部分

```text
POST https://api.deepseek.com/v1/chat/completions
Authorization: Bearer <你的密钥>
Content-Type: application/json

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

URL 指向服务和接口；`POST` 表示发送请求体；header 携带鉴权和格式；body 描述希望模型完成的任务。密钥允许你的程序使用账号的模型服务，不要把它写进 prompt。

`messages` 是模型本次能看到的对话。`model` 选择模型，`max_tokens` 限制本次生成的 token 数，`stream=false` 表示等整个结果回来。请求格式以 [DeepSeek Chat Completions 协议](https://api-docs.deepseek.com/api/create-chat-completion/) 为准。

直接 HTTP 的关键动作就在 `llm.py`：

```python
response = self.http.post(
    self.settings.base_url + '/chat/completions',
    json=body,
    headers={'Authorization': 'Bearer ' + self.settings.api_key},
)
```

这不是模型 SDK。HTTP 库只是替你处理连接和 JSON 编码。其外围代码负责观测、错误与资源释放；真正决定给模型什么材料的是调用者传入的 `messages`。

## 返回值怎么读

下面是便于理解的简化结构，内容和用量是示意值：

```json
{
  "id": "completion-example",
  "model": "deepseek-flash",
  "choices": [{
    "message": {"role": "assistant", "content": "递归是一个过程在解决问题时调用自身。"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 30, "completion_tokens": 15, "total_tokens": 45}
}
```

先确认 HTTP 成功，再确认响应结构，随后查看停止原因，最后读取 `choices[0].message.content`。`choices` 是列表，所以 `[0]` 表示第一个候选结果。

HTTP 200 只说明接口返回了结果。`stop` 表示这轮正常结束；`length` 表示输出触及限制，不能当作完整答案；`tool_calls` 表示需要进入工具流程。客户端遇到截断或异常停止会抛出 `ModelError`，响应仍然保留在 trace 中供查看。正常结束也不证明答案真实，真实性需要任务自己的验证。

## 运行与观察

```bash
cd /Users/wangying/apps/sakelei/ai/agent-learning
uv run python -m demos.beginner.beginner_01_chat
```

终端会显示 messages、响应 JSON 和回答。按 `trace_id` 找到模型 span，对照 `app.request` 与 `app.response`。注意 body 有 `thinking`、`max_tokens`、`stream`，这些是底层默认补上的参数，不是模型自行决定的。

## 三个小实验

1. 把“递归”改成“函数”。预测只会改变哪一项，再看实际请求。
2. 把 system 消息改为“用适合十岁孩子的比喻解释”。比较回答方式，不要只看字数。
3. 把 `model.chat(messages)` 改为 `model.chat(messages, max_tokens=1)`。观察报错和 `finish_reason`，再恢复。通常会截断；不同模型可能返回不同边界行为，以实际记录为准。

一次请求只改一个变量，才能知道差异来自哪里。不要把临时实验失败理解成“程序完全坏了”；正是这些记录帮你区分接口成功和任务完成。

## 自检

- 模型会自动读你电脑上的文件吗？不会；需要程序读取后加入输入，或提供受控工具。
- 为什么要封装客户端？保证所有学习实验自动记录同一组信息，同时保持 HTTP 请求可读。
- 这一段是否已经是 Agent？它只是一次模型调用，还没有程序根据模型的行动请求持续执行任务。

下一节：[消息与上下文](02-messages.md)。
