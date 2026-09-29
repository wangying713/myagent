# Demo 运行与观测

在 `my-agent` 目录统一运行，demo 内只保留 Agent、工具、业务逻辑和需要看到的输出：

```bash
uv run python -m myagent.runtime.lab experiments/hello_world_traced.py
uv run python -m myagent.runtime.lab experiments/image_tool_output.py
uv run python -m myagent.runtime.lab experiments/image_tool_traced.py
uv run python -m myagent.runtime.lab experiments/raw_loop.py --step 3
uv run python -m myagent.runtime.lab experiments/decorator_demo.py
uv run python -m myagent.runtime.lab experiments/_probe_ctx.py
```

`config.env` 配置模型的 `OPENAI_API_KEY`、`OPENAI_BASE_URL`、`MODEL`，以及 `OPENOBSERVE_ENDPOINT`、`OPENOBSERVE_USER`、`OPENOBSERVE_PASSWORD`。环境变量可覆盖这些值，即使文件里没有对应项。`OPENOBSERVE_ENDPOINT` 是 traces OTLP 地址，例如 `http://localhost:5080/api/default/v1/traces`；日志默认使用同组织的 `/v1/logs`，也可以指定 `OPENOBSERVE_LOGS_ENDPOINT`。`OTEL_SERVICE_NAME` 默认是 `my-agent`。

入口打印一次 trace ID，自动采集调用树、HTTP、logging 和 print，结束时刷新。没有观测凭据会明确提示不发送数据。直接 `python experiments/xxx.py` 不执行统一观测初始化。不要在 demo 内再调用 `setup()`、`flush()`、`logging.basicConfig()` 或另装 exporter。

## 在 OpenObserve 中看什么

1. 打开 Traces，选择对应 trace 流、`my-agent` 服务和本次运行时间，按终端 trace ID 找到执行。
2. 根 span 是 demo 文件名；展开后可以看到 Agent、模型调用、工具和 HTTP。统一入口内省略 SDK 工作流、task、turn 包装层，子调用连接到最近的保留节点；日志也归属该节点。Agent、handoff、护栏、自定义操作仍然保留。
3. 模型显示为 `模型调用 1 · 模型名`，工具显示为 `工具 1 · 函数名`，HTTP 显示方法和路径。模型与工具分别编号；并发时序号表示启动顺序。
4. 在 span 详情选择 View Logs；关联字段是 OTLP 的 `trace_id`、`span_id`。如无法跳转，在 Management → Organization Parameters 的 Log details 中检查这两个字段的映射，并选择实际接收数据的日志流。具体菜单随平台版本可能不同。

[OpenObserve 官方关联说明](https://openobserve.ai/docs/user-guide/data-exploration/traces/traces/)。

| 信息 | 在哪里看 |
|---|---|
| demo 文件、执行成功/失败 | 根 span 的 `app.run.entrypoint`、`app.run.status` |
| 模型、工具、HTTP 次数与 token | 根 span 的 `app.run.llm_calls`、`app.run.tool_calls`、`app.run.http_calls`、`app.run.total_tokens` |
| 出错 span 数、警告日志数 | `app.run.error_span_count`、`app.run.warning_log_count`；父子 span 都报错时可能计数多次，不是独立故障数量 |
| 同类调用序号 | 子 span 的 `app.call.sequence`；操作类别使用 span 类型及 `openinference.span.kind` |
| 模型和工具的输入输出 | 对应 span 的 `input.value`、`output.value`；具体内容由 SDK/埋点提供 |
| 请求参数和返回 JSON | HTTP span 的 `app.http.request.body`、`app.http.response.body`；OpenObserve 中通常为 `app_http_request_body`、`app_http_response_body` |
| URL 查询参数 | `app.http.request.query`，敏感参数脱敏 |
| 正文采集情况 | `app.http.request.body.capture_status` / `app.http.response.body.capture_status`；是否截断看 `.body.truncated` |
| 请求地址、状态、耗时范围 | HTTP span 的 `url.full`、`http.response.status_code`、`app.http.duration_scope` |
| 打印和普通日志 | 日志 `event.name=console.output` 或 `app.log`，包含 demo 文件及当前 trace/span 标识 |
| 整次执行摘要 | 日志 `event.name=run.completed` |

自定义字段统一使用 `app.*`，标准协议字段和框架字段保持原名。平台通常把点号展开成下划线，例如 `app.http.request.body` → `app_http_request_body`。`app.run.status=ok` 只代表脚本正常退出，不代表回答正确；查看错误 span 和 WARNING 级别日志进行排查。`app.run.warning_log_count` 只统计 WARNING，不把 ERROR 混入警告数量。

这次命名仅作用于新执行，历史记录不迁移、不删除，不同时上报旧字段。根 span 不再重复输入输出预览，也不再填充未知费用和未经评估的结果状态。日常先看操作名称、耗时、状态、请求正文、响应正文和关联日志。

其他自定义字段：`app.run.root` 标识根 span，`app.run.arguments` 记录启动参数，`app.run.input_tokens` / `app.run.output_tokens` 记录累计 token，`app.llm.base_url` 记录脱敏后的模型服务地址，`app.console.stream` 区分 stdout/stderr。HTTP 内容类型、已缓冲正文的字节数和调试请求 ID 分别使用 `app.http.request.content_type` / `app.http.response.content_type`、`app.http.request.body.size` / `app.http.response.body.size`、`app.http.response.request_id`。

## 图片 demo 的关系

```text
image_tool_output.py
├─ Assistant
│  ├─ 模型调用 1
│  │  └─ HTTP POST /…/chat/completions
│  ├─ 工具 1 · fetch_random_image
│  │  └─ 关联日志：图片工具被调用
│  └─ 模型调用 2
│     └─ HTTP POST /…/chat/completions
└─ 关联日志：最终输出、执行摘要
```

这是成功调用工具时的简化示意；真实执行可能发生重试或多次工具调用。工具只是返回图片 URL，本地不下载图片，不会产生图片下载 HTTP span。`Runner.run()` 返回之后的 print 归属于外层 demo，属于正常现象。`image_tool_output.py` 要求模型支持图片输入；`image_tool_traced.py` 默认返回文本链接。

## 噪音控制

- 官方风格 demo 保留 `print()` 即可；不要再用 `log.info()` 重复同一句输出。自己写的业务日志也会自动关联当前操作。
- HTTP 基础信息默认采集，支持 httpx/httpx2 同步和异步客户端；传输库的 INFO 成功日志不上报，避免一份请求同时占据 span 和日志列表。导出过程不创建 HTTP span。
- HTTP 默认记录已经缓冲的 JSON 请求/响应正文，直接挂在 HTTP span 详情，不额外生成日志。保留 messages、tools、模型参数、模型返回及 JSON 错误信息。Authorization 等请求头不采集；常见密钥字段、Bearer/sk- 凭据脱敏，data URI 图片省略，URL 查询参数独立脱敏记录。任意自然语言里的隐私并不能完全自动识别；模型与工具语义 span 的内容仍按原有方式采集。
- 每份正文最多 16,384 字符，超出后 `.body.truncated=true`（截断内容可能不是完整 JSON）；超过 1 MiB 的原始正文、非 JSON、无效 JSON、未缓冲内容不采集，并通过 `.body.capture_status` 明确原因。旧 trace 不会补回正文，修改后需重新运行 demo。
- `--debug-http` 只额外增加响应请求 ID（内容类型和已缓冲正文大小默认已有），不开启底层 DEBUG 日志，JSON 正文默认已有，无需打开此选项。
- 流式响应不主动读取正文，`.body.capture_status=skipped_streaming`，模型最终内容查看模型 span 的 `output.value`；流式请求 HTTP 耗时截止响应头返回，标记为 `response_headers`；非流式为 `response_body`。完整模型耗时看模型 span。一次 SDK 重试通常会多一个 HTTP span。
- print 按行采集，同一 asyncio 任务及 span 的半行输出会合并；不同任务/不同 span 不混合。普通 logging 终端输出不会再次被作为 print 上报。子进程和直接写文件描述符的输出不在 Python print 采集范围内。

```bash
uv run python -m myagent.runtime.lab --debug-http experiments/hello_world_traced.py
```

`raw_loop.py` 没有 Agents SDK 自动埋点，因此保留少量 `recorded_call()` 来标记模型和工具边界。`_probe_ctx.py` 专门学习上下文，保留它的观测示例；普通 demo 不需要复制这些代码。

## 验证

```bash
uv run pytest -q
```

测试使用内存 exporter、模拟 HTTP 和 ScriptedModel，不联网、不消耗模型 token，覆盖日志去重、并发上下文、真实 SDK 工具关联、HTTP 状态与流、脚本退出码和刷新。进程被 SIGKILL 时无法保证数据刷新。

## 精简层级的边界

统一入口提供“执行入口 → Agent → 模型/工具 → HTTP”的默认视图。只有 SDK 的工作流、TaskSpanData 和 TurnSpanData 包装被折叠，不按名称删除节点，因此自定义操作即使叫 `turn` 也保留。包装层的错误作为 `sdk.wrapper.error` 事件记录在保留的父节点并标记错误，不静默丢弃。没有统一入口根 span 时，SDK 的独立 trace 保留原有根节点。

实现集中在 `runtime/compact_tracing.py`，复用 OpenInference 的输入输出解析及上下文管理，依赖其处理器内部映射；升级该依赖后应运行 SDK 集成测试。测试包含真实 SDK 工具调用、并发 Agent、自定义操作和包装层错误。历史 trace 的层级不会改变，重新运行后才出现精简结构。

请求模型的实际参数统一查看 `app_http_request_body`，不再重复提取 model、stream、temperature、消息数或工具名称等字段。正文继续使用既有脱敏和截断规则。

## Agent 输入预览的范围

Agent 输入预览现在是一个结构化对象，不再只显示用户消息：

- `instructions`：系统提示词。动态提示词直接复用首次 `on_llm_start` 的 SDK 求值结果，不重复调用用户函数；尚未进入模型调用时会显示等待求值。
- `messages`：进入该 Agent 时的消息快照，包含当时传入的历史。
- `first_model_messages`：首次模型调用 hook 收到的消息；各后续轮次仍去对应 HTTP 节点查看。
- `configured_tools`：显式配置的工具名称、类型、描述、参数 JSON Schema、strict 与启用配置。动态启用状态不额外求值。
- `configured_model` / `configured_model_settings`：Agent 配置的模型和设置；null 或空配置不表示最终请求没有模型，而可能来自默认配置、RunConfig 或 provider。
- `handoffs` / `mcp_servers`：配置的交接目标和 MCP 服务名称。不会为预览额外访问 MCP；实际可用工具清单看对应 HTTP 请求的 tools。

这些是开始配置和首次调用的快照，不是把整次运行的所有请求拼成一个输入。工具输入输出仍在工具节点，完整发送参数及响应在每个 HTTP 节点；流式响应以已标记来源的 SDK 汇总结果为准。全部预览沿用脱敏、16,384 字符上限及截断标记；关闭内容采集时不显示这份配置内容。实际请求可能经过 SDK 过滤、配置覆盖或协议转换，因此以 HTTP 内容为最终依据。
