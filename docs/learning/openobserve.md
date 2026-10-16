# OpenObserve：看清实际执行过程

## 配置与入口

当前只发送 Traces。默认 OTLP/HTTP 地址为 `http://localhost:5080/api/default/v1/traces`，界面为 [OpenObserve](http://localhost:5080/web/)。组织和 stream 可通过配置调整，服务名默认 `agent`。

根目录 [.env.example](../../.env.example) 提供变量名与占位值。`OTEL_EXPORTER_OTLP_TRACES_HEADERS` 是逗号分隔的 header，值按 URL 编码，例如 `Authorization=Basic%20REPLACE_ME,stream-name=default`；必须把占位符换成本机真实鉴权。Basic 编码也是凭据，不写入文档、源码或截图。

当前没有独立的观测检查 HTTP 接口或 Go 验证命令。按管理台实际运行的 trace ID，在平台查询；不能运行旧 Python 检查脚本来代替当前服务验收。

## trace 与 span

Trace 对应一次 HTTP 请求，span 对应处理阶段；HTTP、领域运行、模型与工具共享 trace ID，每个 span 有自己的 span ID，并通过父 ID 关联。

```text
/api/agent/chat
└── agent.run
    ├── /v1/chat/completions  # 模型提出乘法调用
    ├── Tool call            # Go 校验与分发，返回 391
    └── /v1/chat/completions  # 带上真实结果，模型回答
```

正常聊天为 3 个 span，消息历史为 4 个，典型单工具循环为 5 个。领域运行是 HTTP 根节点的子节点，模型与工具都是领域运行的直接子节点。一般可按 `2 + model_calls + tool_calls` 核对实际数量，但收到模型响应前失败、或流程在发请求前被取消等情形要结合步骤判断。

Trace ID 关联观测，tool call ID 关联协议消息，不能互换。工具分发失败也需要留记录，并标记错误。

## 查看一次运行

1. 在管理台执行一课，复制 trace ID；观测关闭时没有有效 trace ID。
2. 等待批量导出，在 OpenObserve 的 Traces 中选择对应组织、stream 和运行时间范围。
3. 在追踪列表的查询编辑器中按字段精确筛选：`trace_id = '复制的32位traceID'`，核对根节点、子节点和服务名。详情页的“在 Span 中搜索”只用于当前链路内查找。
4. 时间范围须覆盖该次请求；管理台保留的旧结果可能早于平台默认的“过去 15 分钟”，此时选择“过去 1 天”或该次请求的绝对时间后再查询。
5. 点开 HTTP 根 span，在“属性”前的“预览”查看请求与响应；再展开模型或工具 span，核对实际模型消息、工具名、调用 ID、参数与结果。
6. 如果页面已完成但查不到记录，检查终端观测提示及配置，不能把模型成功当入库成功。

HTTP 根 span 在解码前建立，入口名称只展示 URI 路由模板，例如 `/api/agent/chat`；方法保留在属性中，服务名仍用于标识应用。未匹配路由时统一显示 `[unmatched]`，避免记录原始未知路径。入口包含健康检查、路由和输入拒绝等请求；业务响应之外的 `traceID` 可关联实际链路。关闭观测时不伪造 trace ID。当前不自动记录 DNS、连接池或上报请求。没有重复发送 Logs 或 Metrics，不为每个字段建立独立 span。

## 当前实际属性

| 属性 | 用途 |
| --- | --- |
| `http.route` | HTTP 入口注册的路由模板，不采集原始 URL |
| `app.lesson` | 领域运行 span 的课程标识 |
| `app.llm_calls` / `app.tool_calls` | 领域运行 span 的模型与工具分发次数 |
| `app.total_tokens` / `app.usage.complete` | 领域运行 span 的已知累计用量与完整性 |
| `app.request` / `app.response` | HTTP、模型或工具的实际请求/响应脱敏副本 |
| `app.request.truncated` / `app.response.truncated` | 正文是否截断 |
| `gen_ai.input.messages` / `gen_ai.output.messages` | 直接传递同一正文供界面 JSON 预览，不添加角色或消息包装 |
| `gen_ai.operation.name` | `chat` 或 `execute_tool` |
| `gen_ai.request.model` / `gen_ai.provider.name` | 请求模型和提供方 |
| `gen_ai.response.finish_reasons` | 模型结束原因 |
| `app.usage.total_tokens` | 模型 span 的已知本次总用量 |
| `app.tool.name` / `app.tool.call.id` | 工具名称和协议调用 ID |
| `app.tool.error` | 工具被拒绝的稳定错误码 |
| `http.request.method` / `server.address` / `url.full` / `http.response.status_code` | 模型 HTTP 请求方法、目标主机、完整请求地址与已收到的状态 |
| `app.elapsed_ms` / `error.type` / span 状态 | 耗时、错误类型和失败状态 |

HTTP 预览直接展示实际请求 JSON 和响应 JSON，不增加 `body/method/uri/status` 或用户、助手包装；响应体保留接口本身的 `code/data/traceID/msg` 外层，错误回退响应也能查看。方法、路由和状态保留在 span 属性中。请求体只采集处理器实际读取的部分，不提前读取；请求头与查询字符串不采集。正文在内存中按字符上限的四倍字节数保留有界副本，再统一脱敏并按字符限长；超限、无效 JSON 按文本展示。关闭正文采集时不包装读写器，也不生成预览字段。预览复用 OpenObserve 的输入输出展示字段，字段值为实际正文的脱敏副本，HTTP span 仍保持 Server 类型，不上报模型名称或模型用量。

模型的 Client span 名称只展示实际 HTTP URI 路径，例如 `/v1/chat/completions`，通过 `url.full` 保留完整请求地址；`gen_ai.operation.name` 和 `gen_ai.request.model` 继续记录操作与模型。OpenObserve 瀑布图中客户端行左侧可能显示目标主机，这是 `server.address` 的展示，与上报服务名 `service.name` 分开。

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
| 页面有回答但无记录 | 用 trace_id 字段精确查询，扩大时间范围覆盖该次请求，再检查组织、stream、服务名和上报提示 |
| 有统计无正文 | `TELEMETRY_CAPTURE_CONTENT` 或收到响应之前的网络故障 |
| Logs 页面为空 | 当前只写 Traces，属于预期 |

相关验收见 [验证说明](verification.md)。本节按当前 Go 实现说明，不沿用旧 Python 字段与验证脚本。
