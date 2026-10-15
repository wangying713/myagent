# 02：消息、上下文与 token

## 模型怎样“记住”上一句话

本课程显式发送历史。打开消息历史页签：第一轮告知“小猫叫豆包”，第二轮询问名字，一次点击执行两次模型请求。Go 将本次历史保存在 `[]agent.Message`，下一轮发送增长后的切片。

阅读 [执行器](../../apps/demos/runner.go) 中 `messages` 的追加过程。下面是其核心顺序的摘录，不是独立运行程序：

```go
messages = append(messages, agent.Message{Role: "user", Content: question})
response, callErr := model.Chat(ctx, agent.Request{Messages: messages})
if callErr != nil {
	return result, callErr
}
messages = append(messages, response.Message)
```

第二次请求通常有四条消息：system、第一轮 user、第一轮实际 assistant、第二轮 user。不能把预设答案冒充 assistant，也不能只保存文本而丢掉工具协议字段。`Message.Raw` 与自定义 JSON 编码保留模型完整 assistant，包括额外字段和 `content:null`。

## 四种角色

| 角色 | 来源 | 本项目用途 |
| --- | --- | --- |
| `system` | 应用 | 行为约束和输出要求 |
| `user` | 用户或应用材料 | 当前问题与提供的资料 |
| `assistant` | 模型 | 已生成文本或工具调用请求 |
| `tool` | 程序 | 实际工具结果，包含 `tool_call_id` |

资料中的指令仍是资料，不能因为内容写着“忽略规则”就提升成 system。能否执行动作由应用检查，角色和提示词不赋予权限。

## 上下文、历史和长期记忆

上下文是本次模型收到的全部材料；会话历史是应用保存的消息；长期记忆是经过筛选、持久化、以后再读取的信息。缓存优化不应成为省略必要历史的理由。

当前历史只在一次运行里存在。再次点击、刷新页面或重启服务，都不会加载上次的消息。不同 HTTP 请求独立，不是持久会话；没有 session ID 或数据库。

## token 与统计

Token 是文本编码单位，不严格等于字数。第二轮重新发送第一轮历史，输入通常更长。若三轮输入分别为 100、150、210，输出为 50、60、40，累计为 610 token；这是示意算术，与定价无关。

每个模型步骤保留返回的 `usage`；页面 `known_tokens` 累计已知用量，`usage_complete=false` 表示至少一次用量未知或无法安全累计。根 span 用 `app.total_tokens` 和 `app.usage.complete` 表示同一事实。当前模型 span 记录已知总 token，不声称单独写入了全部输入、输出或缓存属性；这些字段可从响应正文查看。

长期追加会增加成本和无关材料。后续可以学习保留最近几轮、摘要和检索；裁剪工具对话时，assistant 调用与对应 tool 结果必须成组保留。

## 操作与练习

1. 修改两轮问题，例如“请记住虚构项目代号是海盐”和“项目代号是什么？”。展开第二次请求，确认历史来自真实第一轮。
2. 临时在执行器中让第二轮只发送最后一句 user，重启服务并比较，实验后恢复。缺少依据首先是应用输入问题。
3. 阅读 `TestConcurrentRunsDoNotShareHistory` 和 `TestThreeLessonsAndParentSpans`，用离线测试理解并发隔离与历史回传。

当前接口严格接收两轮；增加第三轮需要一起修改执行器输入校验、HTTP 契约、前端表单和测试，不只改页面数量。

## 通过标准

- 能指出第二次请求中四条消息的来源，解释为什么实际 assistant 必须保存。
- 能解释第二轮输入用量可能增加，以及未知用量为什么不能算零。
- 能查到正常 3 个 span：根节点和两次模型请求。
- 能区分本次运行历史与持久会话，知道多次点击不会自动延续上下文。

下一课：[工具与 Agent 循环](03-agent.md)。
