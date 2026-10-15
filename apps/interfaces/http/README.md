# 后台服务 API

`main.go` 解析启动参数、加载配置并组装 DeepSeek、OpenObserve 和 Agent 业务；`server.go` 管理 HTTP 服务及请求的生命周期；`router.go` 逐条注册完整路由；`proto/agent.proto` 定义业务消息并生成 Go 类型，`entity/agent.go` 定义共用 JSON 数据，`agent` 包（`agentapi`）实现具名处理方法和数据转换。通用 JSON 校验、响应编码位于 `infra/httpx`，聊天、历史与工具流程位于 `domain/agent`。Gin 锁定 v1.12.0，依赖见根目录 `go.mod` 与 `go.sum`。生成命令与兼容边界见 [Proto 消息说明](agent/README.md)。

协议定义和生成文件放在 `proto`，手写共用数据放在 [entity](entity/README.md)，后续新增接口遵循相同规范。

三课原理、操作练习与通过标准见 [中文学习指南](../../../docs/learning/README.md)。

在项目根目录运行：

```sh
./scripts/go run ./apps/interfaces/http -env-file .env
```

默认监听 `127.0.0.1:8080`。服务不会自动读取任何 `.env`；环境变量优先于显式文件。启动时缺少模型或已启用观测的鉴权会明确报错。

| 方法与接口 | 正文 | 含义 |
| --- | --- | --- |
| `GET /api/health` | 无 | 服务存活检查，不调用模型、不检查外部服务 |
| `HEAD /api/health` | 无 | 健康检查的 HEAD 形式，不返回正文 |
| `POST /api/agent/chat` | `{}` 或 `{"question":"问题"}` | 普通聊天 |
| `POST /api/agent/messages` | `{}` 或 `{"questions":["第一轮问题","第二轮问题"]}` | 单次执行两轮，第二轮携带第一轮实际历史 |
| `POST /api/agent/run` | `{}` 或 `{"question":"乘法问题"}` | 只允许乘法工具的有限循环 |

POST 使用 `Content-Type: application/json`。空对象使用原 Python demo 默认问题；拒绝未知或跨课程字段、重复字段、null、非法类型、空白问题及多段 JSON。每个问题最多 4096 字符，正文最多 32 KiB。

响应的 `data` 包含 `completed`、`answer`、`steps`、耗时、调用数、已知 token、用量完整性和 `trace_id`。步骤按真实执行顺序保存脱敏后的请求与响应；工具步骤的请求中保留调用 ID 与 JSON 参数。

失败仍保留已执行记录，并返回 `error.code` 和中文 `error.message`：

| HTTP 状态 | 错误码 | 含义 |
| --- | --- | --- |
| 400 | `invalid_input` | 请求结构或课程输入无效 |
| 404 | `not_found` | 接口不存在，包括不匹配的大小写或末尾斜杠 |
| 405 | `method_not_allowed` | 方法不允许，`Allow` 头给出支持的方法 |
| 413 | `request_too_large` | 正文超限 |
| 415 | `invalid_content_type` | 正文不是 JSON |
| 409 | `budget_exceeded` | 达到预算，任务未完成 |
| 408 | `canceled` | 调用方取消；客户端已经断开时无法再收到响应 |
| 504 | `timeout` | 总运行或单次模型请求超时 |
| 502 | `invalid_model_response` / `model_request_failed` | 模型协议或服务错误 |
| 500 | `internal_error` | 尚未写出响应时发生未预期 panic，返回固定中文错误 |
| 503 | `service_unavailable` | 服务退出期间拒绝刚进入处理器的新请求，不调用业务 |

## Gin 使用约定

- `router.go` 的 `newRouter` 返回 `*gin.Engine`，显式注册 `/api/agent/chat`、`/api/agent/messages`、`/api/agent/run` 和 `/api/health`。旧 `/api/demos/*` 已移除；不通过循环或课程字符串生成路由。
- `agent/chat.go`、`messages.go`、`run.go` 使用 `(ctx, req) → (rsp, err)` 签名，分别调用 `domain/agent.Service` 对应方法，返回生成的响应消息及错误。它们不接收 Gin 上下文或直接写 HTTP 响应。
- `router.go` 使用 `httpx.Adapt` 逐条绑定 API；`agentapi.NewAgentAdapter` 注入 JSON 格式和业务错误映射，由通用适配器负责严格解码、请求超时、上下文释放和写出响应。失败时保留返回的部分结果。
- `infra/httpx` 只处理传输契约，不认识问题、模型、工具或 Agent 预算；Agent 业务错误到 HTTP 状态的映射保留在 `agent/errors.go`。
- 用 `gin.New` 显式选择中间件；入口设置 release 模式，不启用默认请求转储或访问日志。异常恢复不输出请求头、panic 原文或堆栈，已写出的响应不会再覆盖。
- `gin.Context` 仅在 HTTP 适配层使用，业务调用传 `c.Request.Context()` 派生的总超时 context；不能将池化的 Gin 上下文保存到业务对象或异步任务。
- 保留现有严格 JSON 解码，不用默认绑定替代重复键、未知字段、大小写和多段 JSON 等校验。32 KiB 限制也覆盖 JSON 后的尾随空白。
- 关闭尾斜杠自动重定向，不自动重发 POST；未找到接口和错误方法统一使用 JSON 错误响应。不信任转发 IP 请求头，后续接入实际反向代理时再显式配置可信地址。
- Gin 作为标准库 `http.Server.Handler` 使用，沿用读写超时、信号取消和优雅退出；模型与工具的观测继续由原有组件负责，不增加重复 span。

版本依据见 [Gin v1.12.0 发布说明](https://github.com/gin-gonic/gin/releases/tag/v1.12.0)，服务生命周期参考 [官方自定义 HTTP 服务说明](https://gin-gonic.com/en/docs/server-config/custom-http-config/)。上述目录、校验和日志策略是本项目约定。

## 运行边界

默认最多 5 次模型请求、8 次工具分发、8000 个已知累计 token 的调用间软预算，不自动重试。用量缺失时不伪装为零；预算不能证明未知用量没有超限。单次模型输出上限为 512 token，思考模式关闭。

每次运行独立保存历史和统计；取消信号传到模型 HTTP 请求，后续工具和模型调用停止。每次点击开始新运行，不共享或持久化会话。

一个根 span 对应一次运行，模型和工具各有子 span。OpenObserve 上报异步进行，失败提示但不覆盖模型结果；只有服务退出时关闭 provider，退出刷新有界。`TELEMETRY_CAPTURE_CONTENT` 控制观测正文采集；页面仍展示用于学习的脱敏执行记录。展示正文默认最多 32768 字符，截断有标记，不改变实际模型消息。

当前是本机开发演示：管理台登录仍属于模板 Mock，不是 Go 服务的真实鉴权。监听地址默认为本机；本次不增加生产部署、数据库、流式、会话存储或其他 Python 课程。

## 服务退出与清理顺序

`main` 负责信号和最终错误输出；`execute` 解析参数并加载配置；`run` 管理依赖；`server.go` 统一处理信号取消和监听异常。

收到信号或监听失败后，先取消请求上下文并停止放行业务请求，再在共用的 10 秒期限内关闭服务、等待监听循环和正在执行的处理器结束。服务退出期间新进入的请求返回固定 JSON 503。监听失败、正常停止失败及强制关闭失败保留原始错误链，最终由入口统一输出。

监听循环返回、连接被关闭，都不等于处理方法已经返回。入口通过请求计数与停止闸门单独确认完成，避免退出等待开始后又加入业务请求。正常结束后先释放模型客户端，再在独立的 5 秒期限内刷新并关闭观测；观测刷新失败只提示，不覆盖原始业务或退出错误。

若退出超时，会强制关闭 HTTP 连接，并报告仍在运行的请求数。Go 无法强制终止不响应取消的处理方法，此时服务会有界返回错误并继续清理，提示后续观测可能缺失，不声称所有请求已经结束。业务和工具仍需遵守上下文取消约定。

## 从 URL 定位代码

```text
POST /api/agent/chat
  → apps/interfaces/http/router.go：httpx.Adapt(adapter, agentAPI.Chat)
  → infra/httpx/adapter.go：解码、派生上下文
  → apps/interfaces/http/agent/chat.go：Agent.Chat(ctx, req) → (rsp, err)
  → domain/agent/chat.go：Service.Chat
  → infra/deepseek/client.go：Client.Chat
```

结果沿调用链返回后，由适配器使用 `agent/adapter.go` 注入的规则映射错误和 JSON，统一写出响应。

历史消息对应 `messages.go`，工具调用对应 `run.go`。`domain/agent/recording.go` 只包装模型和工具，记录执行步骤与预算统计，不决定运行哪个业务。OpenObserve 通过 `infra/openobserve/agent.go` 实现领域定义的观测接口，领域层不依赖具体观测 SDK。

通用 HTTP 包用法见 [httpx 说明](../../../infra/httpx/README.md)。
