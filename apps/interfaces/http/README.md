# 后台服务 API

HTTP 顶层只保留 `main.go` 一个 Go 文件，负责启动参数、配置加载、退出信号和最终错误输出。服务实现与测试集中在 `app` 包，启动入口为 `app.Run(ctx, cfg)`：`app/run.go` 组装 DeepSeek、OpenObserve 和 Agent 业务并管理资源，`app/server.go` 管理 HTTP 生命周期，`app/router.go` 逐条注册完整路由，`app/impl_agent.go` 实现请求准备、领域调用及各接口的 HTTP 响应组装，`app/handler.go` 建立链路并处理 HTTP 解码、错误映射和公共包装。通用 JSON 校验、编码位于 `infra/httpx`，聊天、历史与工具流程位于 `domain/agent`。Gin 锁定 v1.12.0，依赖见根目录 `go.mod` 与 `go.sum`。验证命令见 [修改接口的约定](#修改接口的约定)。

手写请求、响应、HTTP 公共外层与共用数据统一放在 [entity](entity/README.md)，后续新增接口按业务分文件，遵循相同规范。

```text
http/
  main.go
  app/
    run.go
    server.go
    router.go
    handler.go
    impl_agent.go
    *_test.go
  entity/
    response.go
    code.go
    agent.go
    agent_demos.go
```


三课原理、操作练习与通过标准见 [中文学习指南](../../../docs/learning/README.md)。

在项目根目录运行：

```sh
./scripts/go run ./apps/interfaces/http -env-file .env
```

默认监听 `127.0.0.1:8080`。服务不会自动读取任何 `.env`；环境变量优先于显式文件。启动时缺少模型或已启用观测的鉴权会明确报错。

需要在后台运行或修改代码后重启整套服务时，在项目根目录执行：

```sh
./scripts/server
./scripts/server status
./scripts/server stop
tail -f .runtime/http/server.log
```

`./scripts/server` 是前后端统一管理入口，**无参数默认执行整套重启**，也支持显式 `start`、`restart`、`stop` 和 `status`。`start` 补启动缺失的服务，`restart` 同时重启 Go 后台和 Node 管理台；两者默认显式传入项目根目录的 `.env`，指定其他配置可追加 `-env-file 配置文件路径`。`stop` 停止前后端，`status` 同时显示两者状态。Go 重启先编译，成功后发送 `SIGTERM` 并等待旧进程退出，再启动新进程并检查健康接口；未启动时直接启动。Go 二进制、PID 和日志保存在已被 Git 忽略的 `.runtime/http/`，前端 PID 和日志位于 `.runtime/console/`；每次启动重置对应日志。整套启动还会确认管理台页面和后台代理可用，浏览器访问 `http://127.0.0.1:8000/agent-development`。脚本只停止自身管理且启动命令匹配的进程，不按端口批量终止进程；之前手动启动的服务需要先在原终端按 `Ctrl+C`。配置文件只传给 Go 入口解析，不会被当作 Shell 脚本执行，也不会自动创建或覆盖。

| 方法与接口 | 正文 | 含义 |
| --- | --- | --- |
| `GET /api/health` | 无 | 服务存活检查，不调用模型、不检查外部服务 |
| `HEAD /api/health` | 无 | 健康检查的 HEAD 形式，不返回正文 |
| `POST /api/agent/chat` | `{}` 或 `{"question":"问题"}` | 普通聊天 |
| `POST /api/agent/messages` | `{}` 或 `{"questions":["第一轮问题","第二轮问题"]}` | 单次执行两轮，第二轮携带第一轮实际历史 |
| `POST /api/agent/run` | `{}` 或 `{"question":"乘法问题"}` | 只允许乘法工具的有限循环 |

POST 使用 `Content-Type: application/json`。空对象或空字符串问题使用对应接口的默认学习示例；拒绝未知或跨课程字段、重复字段、null、非法类型、空白问题及多段 JSON。每个问题最多 4096 字符，正文最多 32 KiB。

响应的 `data` 包含 `completed`、`answer`、`steps`、耗时、调用数、已知 token、用量完整性。Go 中使用 `Calls []AgentCallRecord` 表示调用记录，HTTP JSON 保留 `steps` 字段名。记录按真实调用顺序保存脱敏后的请求与响应；工具调用记录中保留调用 ID 与 JSON 参数。

统一响应包含四个字段：`code`、`data`、`traceID`、`msg`。成功为 `code: "ok"`、`msg: "成功"`；失败使用下表的字符串错误码和中文说明，同时保留 HTTP 状态。`traceID` 来自 HTTP 请求链路，业务 `data` 不再携带 `trace_id`。解析或输入校验失败时 `data` 为 null，有部分执行结果时保留结果。健康检查也使用相同外层，`data` 为 `{"status":"ok"}`；HEAD 不返回正文。

```json
{"code":"invalid_input","data":null,"traceID":"实际链路 ID","msg":"请求输入无效"}
```

响应码常量定义在 `entity/code.go`，应用使用 `entity.CodeInvalidInput` 等名称；`app/handler.go` 负责领域错误判断、HTTP 状态和具体提示。基础设施独立处理传输错误，不依赖应用 entity。

HTTP 失败码：

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

- `app/router.go` 的 `newRouter` 返回 `*gin.Engine`，显式注册 `/api/agent/chat`、`/api/agent/messages`、`/api/agent/run` 和 `/api/health`。旧 `/api/demos/*` 已移除；不通过循环或课程字符串生成路由。
- `app/impl_agent.go` 中的 `Chat`、`Messages`、`Run` 使用 `(ctx, req) → (rsp, err)` 签名，分别调用 `domain/agent.Client` 对应方法，返回手写的响应及错误。它们不接收 Gin 上下文或直接写 HTTP 响应。
- `app/router.go` 通过 `handle(implAgent.Chat)` 这类单参数调用逐条绑定 API。`app/handler.go` 统一解码、管理超时和取消、映射错误及写出响应，失败时保留已有的部分结果；路由只提供具名业务方法。
- `infra/httpx` 只处理传输契约，不认识问题、模型、工具或 Agent 预算；Agent 业务错误到 HTTP 状态的映射保留在 `app/handler.go`。
- 用 `gin.New` 显式选择中间件；入口设置 release 模式，不启用默认请求转储或访问日志。异常恢复不输出请求头、panic 原文或堆栈，已写出的响应不会再覆盖。
- `gin.Context` 仅在 HTTP 适配层使用，业务调用传 `c.Request.Context()` 派生的总超时 context；不能将池化的 Gin 上下文保存到业务对象或异步任务。
- 保留现有严格 JSON 解码，不用默认绑定替代重复键、未知字段、大小写和多段 JSON 等校验。32 KiB 限制也覆盖 JSON 后的尾随空白。
- 关闭尾斜杠自动重定向，不自动重发 POST；未找到接口和错误方法统一使用 JSON 错误响应。不信任转发 IP 请求头，后续接入实际反向代理时再显式配置可信地址。
- Gin 作为标准库 `http.Server.Handler` 使用，沿用读写超时、信号取消和优雅退出；运行观测由应用层管理，模型观测由 DeepSeek 客户端负责；工具执行记录保留在运行结果中。

版本依据见 [Gin v1.12.0 发布说明](https://github.com/gin-gonic/gin/releases/tag/v1.12.0)，服务生命周期参考 [官方自定义 HTTP 服务说明](https://gin-gonic.com/en/docs/server-config/custom-http-config/)。上述目录、校验和日志策略是本项目约定。

## 运行边界

默认最多 5 次模型请求、8 次工具分发、8000 个已知累计 token 的调用间软预算，不自动重试。用量缺失时不伪装为零；预算不能证明未知用量没有超限。单次模型输出上限为 512 token，思考模式关闭。

每次运行独立保存历史和统计；取消信号传到模型 HTTP 请求，后续工具和模型调用停止。每次点击开始新运行，不共享或持久化会话。

HTTP 请求 span 在解码前通过官方 `otelgin` v0.72.0 中间件建立，显式使用私有 TracerProvider、noop MeterProvider 和空传播器，保持当前服务内链路与仅 Traces 的范围。入口名称只展示 URI 路由模板，例如 `/api/agent/chat`，未匹配路由统一为 `[unmatched]`；方法保留在属性中，屏蔽默认采集的原始路径、User-Agent 与客户端地址；公共响应处理将 4xx 和 5xx 继续标记为失败。领域运行是其子 span，模型请求是领域运行的子 span；工具不创建独立 span。正常聊天、历史和典型工具循环分别为 3、4、4 个 span；解析失败也有 HTTP 请求记录。OpenObserve 上报异步进行，失败提示但不覆盖模型结果；只有服务退出时关闭 provider，退出刷新有界。HTTP 根 span 的“预览”直接展示实际读取的请求 JSON 与写出的响应 JSON，不添加用户、助手或额外的 body 包装；方法、URI 路由模板和状态保留在 span 属性中；请求头和查询字符串不采集。`TELEMETRY_CAPTURE_CONTENT` 控制 HTTP、运行和模型的观测正文采集；页面仍展示用于学习的脱敏执行记录。`TELEMETRY_TRACER_NAME` 设置业务与模型 span 的追踪器名称，默认 `go-demos`，与服务名分别配置。展示正文默认最多 32768 字符，截断有标记，不改变实际模型消息。字符串中内嵌的 JSON 同样使用 `UseNumber` 解码，避免展示副本丢失数值精度。

当前是本机开发演示：管理台登录仍属于模板 Mock，不是 Go 服务的真实鉴权。监听地址默认为本机；本次不增加生产部署、数据库、流式、会话存储或其他 Python 课程。

`agent.run` 在 OpenObserve 的“预览”中展示业务输入与输出：聊天和工具循环显示实际 `question`，历史消息显示两轮 `questions`（包括默认值）；输出为 `completed` 和最终 `answer`，失败时保留已有回答。预览沿用正文采集开关、脱敏和限长设置；模型详细协议查看模型子 span，工具参数、结果和耗时查看运行输出中的 steps。新增预览只作用于服务更新后的新请求。

## 服务退出与清理顺序

`main.go` 的 `main` 负责信号和最终错误输出，`execute` 解析参数并加载配置，再调用 `app.Run`。`app/run.go` 组装依赖、设置 Gin release 模式并管理资源释放；`app/server.go` 统一处理信号取消和监听异常。

收到信号或监听失败后，先取消请求上下文并停止放行业务请求，再在共用的 10 秒期限内关闭服务、等待监听循环和正在执行的处理器结束。服务退出期间新进入的请求返回固定 JSON 503。监听失败、正常停止失败及强制关闭失败保留原始错误链，最终由入口统一输出。

监听循环返回、连接被关闭，都不等于处理方法已经返回。入口通过请求计数与停止闸门单独确认完成，避免退出等待开始后又加入业务请求。正常结束后先释放模型客户端，再在独立的 5 秒期限内刷新并关闭观测；观测刷新失败只提示，不覆盖原始业务或退出错误。

若退出超时，会强制关闭 HTTP 连接，并报告仍在运行的请求数。Go 无法强制终止不响应取消的处理方法，此时服务会有界返回错误并继续清理，提示后续观测可能缺失，不声称所有请求已经结束。业务和工具仍需遵守上下文取消约定。

## 从 URL 定位代码

```text
POST /api/agent/chat
  → apps/interfaces/http/app/router.go：handle(implAgent.Chat)
  → apps/interfaces/http/app/handler.go：调用 httpx.DecodeJSON 解码、派生上下文
  → apps/interfaces/http/app/impl_agent.go：implAgent.Chat(ctx, req) → (rsp, err)
  → domain/agent/chat.go：Client.Chat
  → infra/deepseek/client.go：Client.Chat
```

各接口在 `app/impl_agent.go` 中显式组装自己的 HTTP 响应，随后由 `app/handler.go` 中的 `writeAPIResult` 完成业务错误映射，并包装 `entity.Rsp[T]`，交给 `httpx.Write` 编码并写出。

历史消息对应 `app/impl_agent.go` 的 `Messages`，工具调用对应 `Run`。普通聊天和工具调用入口统一调用领域层 `Client.Chat`，由调用方明确传入问题、提示词和工具；`/api/agent/run` 保留原接口与默认示例。`domain/agent/agent_run.go` 的 `callModel` 和 `callTool` 负责实际调用及执行记录，模型调用前检查预算，工具调用预算由 `Chat` 按整批检查。应用层 `app/agent_service.go` 管理每次执行的运行观测与结果脱敏，`infra/openobserve/client.go` 管理观测客户端，`run.go` 管理单次运行，`preview.go` 负责观测输入与输出转换，具体分工见 [OpenObserve 组件说明](../../../infra/openobserve/README.md)；工具由调用方直接传入并执行，不增加观测包装。领域层不定义观测接口，也不依赖观测 SDK；领域返回原始执行记录及实际输入，HTTP 响应和观测预览仍保持脱敏。

通用 HTTP 包用法见 [httpx 说明](../../../infra/httpx/README.md)。

## API 方法写法

三个 API 统一使用 `(ctx, req) → (rsp, err)`：上下文和请求是入参，响应与错误是返回值。例如当前消息历史入口：

```go
func (a *ImplAgent) Messages(ctx context.Context, req *entity.MessagesReq) (*entity.MessagesRsp, error) {
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{
		Questions: req.Questions,
	})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &entity.MessagesRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       result.ModelCalls,
		ToolCalls:        result.ToolCalls,
		KnownTokens:      result.KnownTokens,
		UsageComplete:    result.UsageComplete,
		Calls:            agentCalls(result.Calls),
	}, err
}
```

各接口显式处理领域返回值并组装自己的响应，允许重复字段赋值。步骤与用量是实际复用的数据，保留其共用转换。

请求与响应分别命名为 `ChatReq/ChatRsp`、`MessagesReq/MessagesRsp`、`RunReq/RunRsp`，都手写在 `entity`。

`Chat` 接口在调用前补齐默认学习问题和提示词，再通过领域 `ChatReq` 传给 Agent 客户端；通用 `Client.Chat` 不内置示例问题或提示词，空问题由领域校验拒绝，不传提示词时不添加系统消息。`Run` 同样在 API 方法内补齐工具案例的默认问题，并显式传入工具与提示词。API 方法不接收 `gin.Context`，不解析 HTTP 正文、不设置超时、不直接写响应。

`app/router.go` 创建 `newImplAgent(agentCli)`，通过 `requestTimeout(timeout)` 中间件统一设置当前路由的超时；路由只传入业务方法：

```go
router.POST("/api/agent/chat", handle(implAgent.Chat))
router.POST("/api/agent/messages", handle(implAgent.Messages))
router.POST("/api/agent/run", handle(implAgent.Run))
```

`app/handler.go` 的 `handle` 为每个请求创建新的请求对象，调用 `httpx.DecodeJSON` 严格解码，保留省略与显式空数组后，从 HTTP 请求上下文派生总超时，调用具名 API，最后统一处理响应并释放上下文。超时设置由中间件保存在当前 Gin 请求中，业务上下文只接收派生出的截止时间；没有全局配置或额外业务协程。业务方法签名由 Go 编译器检查；请求字段从显式 JSON 标签提取，不通过反射寻找业务方法或按课程字符串分发。

`writeAPIResult` 在 HTTP 处理层映射业务错误并统一包装手写响应，随后直接编码，不维护第二套响应表现及按类型分发的转换函数。响应和错误可以同时非空：例如预算中止、模型失败时仍返回已完成的步骤。响应处理不能看到错误就丢弃响应，API 也不能在已有部分结果时提前返回 `nil, err`。解析失败或没有结果时统一使用 `data: null`。观测关闭时 `traceID` 为空；服务退出闸门直接拒绝、尚未进入路由的 503 也使用相同四字段且 trace ID 为空。

## HTTP JSON 兼容边界

请求由 `httpx.DecodeJSON` 按手写结构体的显式 JSON 标签严格解码。保留 32 KiB 正文限制、内容类型检查，以及对未知字段、重复字段、大小写与驼峰别名、显式 null、多段 JSON 的拒绝；不使用 Gin 默认绑定替代这些检查。字段名从标签派生，不在处理器里维护另一份允许字段列表。嵌套对象和数组元素也执行严格检查。

- `question` 使用 `string`，省略和空字符串均由对应 API 补齐默认问题，纯空白或超长问题仍然无效。
- `questions` 使用 `[]string`；省略保持 nil，显式空数组保持非 nil 空切片，不需要额外字段存在性映射或请求准备函数。
- `model_calls` 使用 `*int` 区分省略与明确零值；为保持现有输入兼容，整数允许 JSON 数字、数字字符串和整数值的指数形式，拒绝小数值和溢出。
- 未知用量使用 `*int` 表示，nil 输出 null，明确零值输出数字 0。

响应直接编码 `entity` 中定义的类型，外层统一为 `code/data/traceID/msg`；保留原字段名、JSON 数字、零值、null、可省略的步骤属性，以及 `steps: null` 与 `steps: []` 的区别。原 API 各自定义响应，按需复用步骤和用量等共用类型；Demo 复用执行记录。领域结果只转换一次，不再经过中间协议类型。后续规则见 [HTTP 共用数据规范](entity/README.md)。

Go `int64` 在服务端完整保留；浏览器中的数字仍受 JavaScript 安全整数范围限制，与改造前相同。

## 修改接口的约定

1. 在 `entity` 中修改对应业务的请求或响应，字段写清楚 JSON 标签与中文含义。
2. 在对应 `app/impl_xxx.go` 中准备领域输入并组装响应，不把 HTTP 类型引入领域层。
3. 在 `router.go` 中逐条声明完整路径并绑定具名业务方法。
4. 数据类型不替代业务校验；问题数量、长度、工具权限和预算继续由业务处理。
5. 新增或修改字段时同步管理台需要的类型与接口契约测试。
6. 运行格式检查、`./scripts/go vet ./...`、`./scripts/go test ./...` 与 `./scripts/go test -race ./...`；全程使用假模型和内存观测器。

## Agent Demos 接口

新增 `POST /api/agent-demos/overview`、`chat`、`messages`、`tools`、`structured`、`multiple-tools`、`stream`、`reliability`、`retrieval`、`workflow`、`observation`、`evaluation`、`delivery`，对应管理台的 13 个章节。路由逐条声明在 `app/router.go`，入口实现在 `app/impl_agent_demos.go`。原接口保留。

普通响应仍为 `code/data/traceID/msg`；`data.execution` 是执行记录，概览分别返回 `direct/withTools`，评估返回 `cases/passed/total`。请求示例与实际演示边界见 `docs/learning` 各章的“当前可以运行的 Demo”。所有工具和资料都在本地固定注册，不执行外部动作。

`stream` 返回 SSE；每条 `data:` 都是统一响应外层，末条 `data.done=true`。需检查 `code` 与 `execution.completed` 判断是否成功，收到部分内容或 HTTP 200 不能代表完成。取消请求会取消模型连接。离线测试用本机模拟服务，不读取真实密钥。
