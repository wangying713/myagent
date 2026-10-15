# OpenObserve：看清实际执行过程

## 配置与入口

当前只发送 Traces。默认 OTLP/HTTP 地址为 `http://localhost:5080/api/default/v1/traces`，界面为 [OpenObserve](http://localhost:5080/web/)。组织和 stream 可通过配置调整，服务名默认 `go-agent-demos`。

根目录 [.env.example](../../.env.example) 提供变量名与占位值。`OTEL_EXPORTER_OTLP_TRACES_HEADERS` 是逗号分隔的 header，值按 URL 编码，例如 `Authorization=Basic%20REPLACE_ME,stream-name=default`；必须把占位符换成本机真实鉴权。Basic 编码也是凭据，不写入文档、源码或截图。

当前没有独立的观测检查 HTTP 接口或 Go 验证命令。按管理台实际运行的 trace ID，在平台查询；不能运行旧 Python 检查脚本来代替当前服务验收。

## trace 与 span

Trace 对应一次运行，span 对应一步；同一运行共享 trace ID，每个 span 有自己的 span ID，并通过父 ID 关联。

```text
agent.run
├── chat deepseek-flash  # 模型提出乘法调用
├── Tool call            # Go 校验与分发，返回 391
└── chat deepseek-flash  # 带上真实结果，模型回答
```

正常聊天为 2 个 span，消息历史为 3 个，典型单工具循环为 4 个。模型与工具都是运行根节点的直接子节点。一般可按 `1 + model_calls + tool_calls` 核对实际数量，但收到模型响应前失败、或流程在发请求前被取消等情形要结合步骤判断。

Trace ID 关联观测，tool call ID 关联协议消息，不能互换。工具分发失败也需要留记录，并标记错误。

## 查看一次运行

1. 在管理台执行一课，复制 trace ID；观测关闭时没有有效 trace ID。
2. 等待批量导出，在 OpenObserve 的 Traces 中选择对应组织、stream 和运行时间范围。
3. 按 trace ID 搜索，核对根节点、子节点和服务名。
4. 展开模型 span，比较请求和响应；工具课程再核对工具名、调用 ID、参数与真实结果。
5. 如果页面已完成但查不到记录，检查终端观测提示及配置，不能把模型成功当入库成功。

当前没有将后台 HTTP 处理本身作为额外 span，也不自动记录 DNS、连接池或上报请求。没有重复发送 Logs 或 Metrics，不为每个字段建立独立 span。

## 当前实际属性

| 属性 | 用途 |
| --- | --- |
| `app.lesson` | 根 span 的课程标识 |
| `app.llm_calls` / `app.tool_calls` | 根 span 的模型与工具分发次数 |
| `app.total_tokens` / `app.usage.complete` | 根 span 的已知累计用量与完整性 |
| `app.request` / `app.response` | 模型或工具的实际请求/响应脱敏副本 |
| `app.request.truncated` / `app.response.truncated` | 正文是否截断 |
| `gen_ai.input.messages` / `gen_ai.output.messages` | 将同一正文包装为文本消息，供界面预览 |
| `gen_ai.operation.name` | `chat` 或 `execute_tool` |
| `gen_ai.request.model` / `gen_ai.provider.name` | 请求模型和提供方 |
| `gen_ai.response.finish_reasons` | 模型结束原因 |
| `app.usage.total_tokens` | 模型 span 的已知本次总用量 |
| `app.tool.name` / `app.tool.call.id` | 工具名称和协议调用 ID |
| `app.tool.error` | 工具被拒绝的稳定错误码 |
| `http.request.method` / `server.address` / `http.response.status_code` | 模型 HTTP 请求与已收到的状态 |
| `app.elapsed_ms` / `error.type` / span 状态 | 耗时、错误类型和失败状态 |

工具输入当前是完整 tool call JSON，参数仍是 JSON 字符串；工具输出为实际 `ok/result` 或 `ok/error`。工具同样使用 `app.request` / `app.response`，没有旧版的 `app.tool.request` / `app.tool.response`。

SQL 字段名与界面呈现以本机平台实际 schema 为准，不把界面布局当协议要求。若正文超限，副本包含 `[TRUNCATED]`；不能把被截断的 JSON 直接重放为请求。

## 内容开关与脱敏

`TELEMETRY_CAPTURE_CONTENT=true` 默认采集正文，`TELEMETRY_CONTENT_MAX_CHARS=32768` 限制每份脱敏副本的字符数。已知密钥、鉴权和常见敏感字段替换成 `[REDACTED]`，不会修改发送给模型的真实请求或回填消息。

关闭观测正文仍保留耗时、状态等元信息。管理台执行记录是独立的学习展示，仍会展示脱敏正文；该开关不会隐藏页面内容。脱敏不是任意隐私识别，学习用虚构材料。

## 生命周期与故障

[观测实现](../../infra/openobserve/telemetry.go) 使用私有 provider、100% 采样和批量导出：队列最多 256 个 span，每批最多 32 个，批次等待 1 秒，上报请求超时 3 秒且不自动重试。观测 HTTP 不继承系统代理、不跟随重定向。

provider 在服务启动时创建，在服务退出时有界关闭刷新，不随单次 HTTP 请求关闭。上报失败只提示固定中文信息，不覆盖模型结果；队列溢出、断网或强制结束可能丢数据。`Flush` 返回不等于数据已经可查询，入库仍须平台核对。

| 现象 | 排查方向 |
| --- | --- |
| 连接被拒绝 | 本机服务或容器是否启动，5080 端口是否正确 |
| 鉴权失败 | Basic 值、组织权限、header 编码；不要打印真实凭据 |
| 地址不存在 | 地址是否以 `/api/组织/v1/traces` 结尾 |
| 页面有回答但无记录 | 上报提示、采集开关、组织、stream、时间范围与服务名 |
| 有统计无正文 | `TELEMETRY_CAPTURE_CONTENT` 或收到响应之前的网络故障 |
| Logs 页面为空 | 当前只写 Traces，属于预期 |

相关验收见 [验证说明](verification.md)。本节按当前 Go 实现说明，不沿用旧 Python 字段与验证脚本。
