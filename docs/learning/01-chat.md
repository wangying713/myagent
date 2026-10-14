# 第一课：一次 HTTP 模型调用

```sh
go run ./apps/interfaces/learning -lesson chat -env-file .env -verify
```

阅读顺序：`apps/interfaces/learning/lessons.go` 的 `chatLesson` → `domain/agent/entity/chat.go` 的请求与响应 → `infra/deepseek/client.go` 的 `Chat`。

## 发生了什么

程序构造 system 和 user 消息，编码 JSON，通过 HTTP POST 请求 `/chat/completions`。鉴权放在 header，业务消息放在 body。响应中读取 assistant 文本、`finish_reason` 和服务端用量。

HTTP 200 只说明请求得到了成功状态；`finish_reason=length`、空回答和无效结构都不能当成完整答案。客户端会保留脱敏后的实际响应，然后明确返回错误。

正常运行在同一 trace 下有两个 span：`agent.run` 和一次 `chat 模型名`。没有 HTTP 自动埋点，因此不会额外出现 DNS、连接池或上报请求节点。

## 小练习

1. 用 `-question "用一句话解释什么是闭包"` 换问题，查看 `app.request` 中的 user 内容。
2. 在课程请求中临时设置 `MaxTokens: 1`，观察有响应但不能正常完成的情况，再恢复。
3. 关闭 `TELEMETRY_CAPTURE_CONTENT`，比较 token、耗时、状态与正文的变化。

通过标准：能找到请求地址、鉴权位置、正文、回答和结束原因；能解释为什么缺失用量不应该当成零；能按 trace ID 查回结果。
