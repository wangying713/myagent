# OpenObserve：查看并验证真实调用链

## 配置和独立检查

在 `.env` 中配置完整的 `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` 和 `OTEL_EXPORTER_OTLP_TRACES_HEADERS`。默认组织为 `default`、端点为 `http://localhost:5080/api/default/v1/traces`；stream 用 `stream-name` header 指定，默认 `default`。

```sh
go run ./apps/interfaces/learning -lesson observe -env-file .env
```

这个命令不需要模型密钥，不调用模型。它发送 `observability.check`，刷新导出队列，再在最近 24 小时内按 trace ID 查询。只有记录实际查回且 ID、父子关系有效，才输出验收成功。

## 课程中的记录

| 步骤 | 内容 |
| --- | --- |
| `agent.run` | 课程、实际模型请求数、工具分发数、已知累计用量与完整性 |
| `chat 模型名` | 实际 HTTP 请求/响应、状态、耗时、模型名、结束原因与 token |
| `Tool call` | 工具名、调用 ID、模型给出的参数、程序实际返回值与失败码 |

模型和工具步骤都是运行根节点的子节点。工具 span 的 `app.llm_call` 表示它来自第几次模型请求。

`app.request` / `app.response` 保存完整 JSON 的脱敏副本；`gen_ai.input.messages` / `gen_ai.output.messages` 提供界面预览。工具的输入输出是直接 JSON，正文另保存在 `app.tool.request` / `app.tool.response`。OpenObserve 查询字段可能把点转换成下划线。

不会给 HTTP 客户端、DNS、上报请求自动建 span，也不重复向 Logs 和 Metrics 发送相同数据。上报使用 OTLP Protobuf、批量队列与全量采样。

## 完整正文的含义

保留实际 JSON 结构和响应扩展字段，但凭据会被替换，正文会受长度限制；这不改变发送给模型或返回给业务的内容。

默认每个属性最多 32768 个 Unicode 字符。`.chars` 记录清理后、截断前长度，`.truncated` 标明是否截断；截断内容可能不再是合法 JSON。预览的 JSON 外层也受同样限制，可能比原始正文更早截断。

可以设 `TELEMETRY_CAPTURE_CONTENT=false`，仅保留调用关系、用量、耗时和状态。已知 API key、鉴权值、常见敏感字段和凭据形式会被清理；不要把自动脱敏当成通用隐私识别。

## 失败不冒充成功

上报每批只尝试一次、超时三秒。上报失败只提示一次，不覆盖模型结果。退出有五秒刷新时间；内存队列没有持久化保证，强制终止或服务不可用时可能丢记录。

`-verify` 是严格验收模式：业务成功后，还要确认上报与实际查询。它会按真实请求和工具次数算出预期记录数，并检查父子关系；若查不全，命令返回失败，而不是把 HTTP 200 当成成功。

```sh
go run ./apps/interfaces/learning -lesson agent -env-file .env -verify
go run ./apps/interfaces/learning -lesson verify -env-file .env -trace-id 实际trace_id -expect 4
```

独立查询不生成新的 trace。`-expect` 是至少应查回的数量，默认值 1 不能证明整个多步骤课程已经入库。查询最多返回 100 个 span，适合第一阶段的有界运行。

## 排查

- 连接失败：检查现有 OpenObserve 服务和端口。
- 401 / 403：检查鉴权、组织和权限；文档与代码不保存真实凭据。
- 404：确认完整端点以 `/api/组织/v1/traces` 结尾。
- 模型成功但查不到：检查上报提示、stream、时间范围和 trace ID；模型成功与观测成功是两件事。
- 有用量没有正文：检查采集开关和截断标记。
- Logs 页面为空：第一阶段只发送 Traces。

模型、查询和上报客户端不继承环境代理，也不跟随重定向。需要其他网络方式时应明确配置，避免鉴权被意外转发。
