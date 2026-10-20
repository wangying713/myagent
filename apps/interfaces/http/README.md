# 后台服务 API

HTTP 顶层只保留 `main.go` 一个 Go 文件，负责启动参数、配置加载、退出信号和最终错误输出。服务实现与测试集中在 `app` 包，启动入口为 `app.Run(ctx, cfg)`：`app/run.go` 组装 DeepSeek、OpenObserve 和 Agent 业务并管理资源，`app/server.go` 管理 HTTP 生命周期，`app/router.go` 逐条注册完整路由，`app/impl_agent.go` 实现请求准备、领域调用及各接口的 Proto 响应组装，`app/handler.go` 建立链路并处理 HTTP JSON 适配、错误映射和公共包装。通用 JSON 校验、编码位于 `infra/httpx`，聊天、历史与工具流程位于 `domain/agent`。Gin 锁定 v1.12.0，依赖见根目录 `go.mod` 与 `go.sum`。生成命令见 [生成与一致性检查](#生成与一致性检查)。

协议源文件和生成的请求响应类型放在 `proto`，手写 HTTP 公共外层与 JSON 表现类型放在 [entity](entity/README.md)，后续新增接口按业务分文件，遵循相同规范。

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
    agent_json.go
  proto/
    agent.proto
    agent.pb.go
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

响应的 `data` 包含 `completed`、`answer`、`steps`、耗时、调用数、已知 token、用量完整性。步骤按真实执行顺序保存脱敏后的请求与响应；工具步骤的请求中保留调用 ID 与 JSON 参数。

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
- `app/impl_agent.go` 中的 `Chat`、`Messages`、`Run` 使用 `(ctx, req) → (rsp, err)` 签名，分别调用 `domain/agent.Client` 对应方法，返回生成的响应消息及错误。它们不接收 Gin 上下文或直接写 HTTP 响应。
- `app/router.go` 通过 `handle(implAgent.Chat)` 这类单参数调用逐条绑定 API。`app/handler.go` 统一解码、准备请求、管理超时和取消、转换响应及映射错误，失败时保留已有的部分结果；路由只提供具名业务方法。
- `infra/httpx` 只处理传输契约，不认识问题、模型、工具或 Agent 预算；Agent 业务错误到 HTTP 状态的映射保留在 `app/handler.go`。
- 用 `gin.New` 显式选择中间件；入口设置 release 模式，不启用默认请求转储或访问日志。异常恢复不输出请求头、panic 原文或堆栈，已写出的响应不会再覆盖。
- `gin.Context` 仅在 HTTP 适配层使用，业务调用传 `c.Request.Context()` 派生的总超时 context；不能将池化的 Gin 上下文保存到业务对象或异步任务。
- 保留现有严格 JSON 解码，不用默认绑定替代重复键、未知字段、大小写和多段 JSON 等校验。32 KiB 限制也覆盖 JSON 后的尾随空白。
- 关闭尾斜杠自动重定向，不自动重发 POST；未找到接口和错误方法统一使用 JSON 错误响应。不信任转发 IP 请求头，后续接入实际反向代理时再显式配置可信地址。
- Gin 作为标准库 `http.Server.Handler` 使用，沿用读写超时、信号取消和优雅退出；模型与工具的观测继续由原有组件负责，不增加重复 span。

版本依据见 [Gin v1.12.0 发布说明](https://github.com/gin-gonic/gin/releases/tag/v1.12.0)，服务生命周期参考 [官方自定义 HTTP 服务说明](https://gin-gonic.com/en/docs/server-config/custom-http-config/)。上述目录、校验和日志策略是本项目约定。

## 运行边界

默认最多 5 次模型请求、8 次工具分发、8000 个已知累计 token 的调用间软预算，不自动重试。用量缺失时不伪装为零；预算不能证明未知用量没有超限。单次模型输出上限为 512 token，思考模式关闭。

每次运行独立保存历史和统计；取消信号传到模型 HTTP 请求，后续工具和模型调用停止。每次点击开始新运行，不共享或持久化会话。

HTTP 请求 span 在解码前通过官方 `otelgin` v0.72.0 中间件建立，显式使用私有 TracerProvider、noop MeterProvider 和空传播器，保持当前服务内链路与仅 Traces 的范围。入口名称只展示 URI 路由模板，例如 `/api/agent/chat`，未匹配路由统一为 `[unmatched]`；方法保留在属性中，屏蔽默认采集的原始路径、User-Agent 与客户端地址；公共响应处理将 4xx 和 5xx 继续标记为失败。领域运行是其子 span，模型和工具继续作为领域运行的子 span。正常聊天、历史和典型工具循环分别为 3、4、5 个 span；解析失败也有 HTTP 请求记录。OpenObserve 上报异步进行，失败提示但不覆盖模型结果；只有服务退出时关闭 provider，退出刷新有界。HTTP 根 span 的“预览”直接展示实际读取的请求 JSON 与写出的响应 JSON，不添加用户、助手或额外的 body 包装；方法、URI 路由模板和状态保留在 span 属性中；请求头和查询字符串不采集。`TELEMETRY_CAPTURE_CONTENT` 控制 HTTP、模型和工具的观测正文采集；页面仍展示用于学习的脱敏执行记录。展示正文默认最多 32768 字符，截断有标记，不改变实际模型消息。字符串中内嵌的 JSON 同样使用 `UseNumber` 解码，避免展示副本丢失数值精度。

当前是本机开发演示：管理台登录仍属于模板 Mock，不是 Go 服务的真实鉴权。监听地址默认为本机；本次不增加生产部署、数据库、流式、会话存储或其他 Python 课程。

`agent.run` 在 OpenObserve 的“预览”中展示业务输入与输出：聊天和工具循环显示实际 `question`，历史消息显示两轮 `questions`（包括默认值）；输出为 `completed` 和最终 `answer`，失败时保留已有回答。预览沿用正文采集开关、脱敏和限长设置；模型与工具的详细协议继续查看子 span。新增预览只作用于服务更新后的新请求。

## 服务退出与清理顺序

`main.go` 的 `main` 负责信号和最终错误输出，`execute` 解析参数并加载配置，再调用 `app.Run`。`app/run.go` 组装依赖、设置 Gin release 模式并管理资源释放；`app/server.go` 统一处理信号取消和监听异常。

收到信号或监听失败后，先取消请求上下文并停止放行业务请求，再在共用的 10 秒期限内关闭服务、等待监听循环和正在执行的处理器结束。服务退出期间新进入的请求返回固定 JSON 503。监听失败、正常停止失败及强制关闭失败保留原始错误链，最终由入口统一输出。

监听循环返回、连接被关闭，都不等于处理方法已经返回。入口通过请求计数与停止闸门单独确认完成，避免退出等待开始后又加入业务请求。正常结束后先释放模型客户端，再在独立的 5 秒期限内刷新并关闭观测；观测刷新失败只提示，不覆盖原始业务或退出错误。

若退出超时，会强制关闭 HTTP 连接，并报告仍在运行的请求数。Go 无法强制终止不响应取消的处理方法，此时服务会有界返回错误并继续清理，提示后续观测可能缺失，不声称所有请求已经结束。业务和工具仍需遵守上下文取消约定。

## 从 URL 定位代码

```text
POST /api/agent/chat
  → apps/interfaces/http/app/router.go：handle(implAgent.Chat)
  → apps/interfaces/http/app/handler.go：调用 httpx.DecodeProtoJSON 解码、准备请求、派生上下文
  → apps/interfaces/http/app/impl_agent.go：implAgent.Chat(ctx, req) → (rsp, err)
  → domain/agent/chat.go：Client.Chat
  → infra/deepseek/client.go：Client.Chat
```

各接口在 `app/impl_agent.go` 中显式组装自己的 Proto 响应，随后由 `app/handler.go` 中的 `writeAPIResult` 完成业务错误及 JSON 映射，并包装 `entity.Rsp[T]`，交给 `httpx.Write` 编码并写出。

历史消息对应 `app/impl_agent.go` 的 `Messages`，工具调用对应 `Run`。`domain/agent/agent_run.go` 的 `callModel` 和 `callTool` 负责实际调用及执行记录，模型调用前检查预算，工具调用预算由 `Chat` 按整批检查。OpenObserve 通过 `infra/openobserve/agent.go` 实现领域定义的观测接口，领域层不依赖具体观测 SDK。

通用 HTTP 包用法见 [httpx 说明](../../../infra/httpx/README.md)。

## API 方法写法

三个 API 统一使用 `(ctx, req) → (rsp, err)`：上下文和请求是入参，响应与错误是返回值。例如当前消息历史入口：

```go
func (a *ImplAgent) Messages(ctx context.Context, req *proto.MessagesReq) (*proto.MessagesRsp, error) {
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{Questions: req.GetQuestions()})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.MessagesRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMs:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}
```

各接口显式处理领域返回值并组装自己的响应，允许重复字段赋值。步骤与用量是实际复用的消息，保留其共用数据转换。

请求与响应分别命名为 `ChatReq/ChatRsp`、`MessagesReq/MessagesRsp`、`RunReq/RunRsp`，都由 Proto 生成到 `proto`。

`Chat` 接口在调用前补齐默认学习问题和提示词，再通过领域 `ChatReq` 传给 Agent 客户端；通用 `Client.Chat` 不内置示例问题或提示词，空问题由领域校验拒绝，不传提示词时不添加系统消息。`Run` 直接传入 `req.GetQuestion()`，由工具案例入口补齐自己的默认问题。API 方法不接收 `gin.Context`，不解析 HTTP 正文、不设置超时、不直接写响应。

`app/router.go` 创建 `newImplAgent(agentCli)`，通过 `requestTimeout(timeout)` 中间件统一设置当前路由的超时；路由只传入业务方法：

```go
router.POST("/api/agent/chat", handle(implAgent.Chat))
router.POST("/api/agent/messages", handle(implAgent.Messages))
router.POST("/api/agent/run", handle(implAgent.Run))
```

`app/handler.go` 的 `handle` 为每个请求创建新的请求对象，调用 `httpx.DecodeProtoJSON` 严格解码，按请求类型完成必要的字段准备，再从 HTTP 请求上下文派生总超时，调用具名 API，最后统一处理响应并释放上下文。超时设置由中间件保存在当前 Gin 请求中，业务上下文只接收派生出的截止时间；没有全局配置或额外业务协程。泛型约束在编译期检查请求指针实现 Proto 消息接口，不通过反射寻找方法或按课程字符串分发。

`writeAPIResult` 在 HTTP 处理层按实际响应类型选择 `chatRspJSON`、`messagesRspJSON` 或 `runRspJSON`，再映射业务错误并统一包装；新增响应时在这里显式增加对应的 JSON 表现，未配置的类型返回统一 500，避免意外使用生成标签或标准 ProtoJSON 改变契约。响应和错误可以同时非空：例如预算中止、模型失败时仍返回已完成的步骤。响应处理不能看到错误就丢弃响应，API 也不能在已有部分结果时提前返回 `nil, err`。解析失败或没有结果时统一使用 `data: null`。观测关闭时 `traceID` 为空；服务退出闸门直接拒绝、尚未进入路由的 503 也使用相同四字段且 trace ID 为空。

## 生成与一致性检查

在项目根目录执行：

```sh
# 修改 agent.proto 后，重新生成 Go 消息。
./scripts/generate-proto

# 只比较生成结果，不修改已检入的生成文件；不一致时返回非零退出码。
./scripts/generate-proto --check
```

工具版本：

| 工具 | 固定版本与来源 |
| --- | --- |
| Go | 项目工具链 1.27.2，通过 `scripts/go` 使用 |
| protoc | 29.3，脚本校验版本 |
| protoc-gen-go 与 Go Protobuf 运行时 | 同为 `go.mod` 锁定的 v1.36.12 |

生成脚本优先使用项目内 `.tools/protoc-29.3/bin/protoc`，否则检查 `PATH` 中的 `protoc`。已有 29.3 版可直接运行。其他机器可从 [Protobuf v29.3 官方发布页](https://github.com/protocolbuffers/protobuf/releases/tag/v29.3) 下载对应操作系统与架构的编译器压缩包，将其中的 `bin`、`include` 解压到 `.tools/protoc-29.3`；不需要覆盖系统安装。只构建、运行已检入的 Go 代码不需要 protoc。

脚本使用项目 Go 工具链，将生成器构建到被忽略的 `.tools/protobuf`，不需要全局安装插件；第一次运行如果 Go 模块未缓存，需要下载锁定依赖。协议命名空间为 `saklay.http.agent`，`go_package` 指向 `saklay/ai/apps/interfaces/http/proto`，生成器使用模块输出模式，把 `proto/agent.proto` 生成到同目录的 `proto/agent.pb.go`。生成文件由临时目录中的完整结果覆盖，检查模式不改源文件。提交时同时包含 `.proto` 和 `.pb.go`，并运行生成一致性检查。

生成文件开头可能将编译器显示为 `v5.29.3`，这是该工具的版本表示；命令行检查仍是 `libprotoc 29.3`。生成器自动附带的英文机械注释保留，业务消息注释来自中文 `.proto`。

## HTTP JSON 兼容边界

请求先由 `httpx.DecodeProtoJSON` 按消息描述符校验顶层字段名，再由官方 `protojson` 解码。保留 32 KiB 正文限制、内容类型检查，以及对顶层未知字段、重复字段、大小写与驼峰别名、显式 null、多段 JSON 的拒绝；不使用 Gin 默认绑定替代这些检查。字段名从 `.proto` 派生，不在处理器里维护第二份允许字段列表。

- `question` 保留 `optional string` 协议，接口直接传递 `GetQuestion()` 返回的字符串；省略和空字符串均由对应案例的调用入口补齐默认问题，纯空白或超长问题仍然无效。
- `questions` 使用 `repeated string`。这类字段本身不提供存在性，因此通用解码器另行返回实际出现的顶层字段。路由为消息接口显式绑定 `prepareAgentMessages`，在成功解码后将显式空数组保留为非 nil 的空切片，省略则保持 nil。领域方法仍按原规则处理，不从上下文读取隐藏参数。
- 用量使用 `optional int64`，区分未知、已知为零和其他数值。

**业务 data 保留项目的 HTTP JSON 表现，不是标准 ProtoJSON 响应；公共外层统一为 code/data/traceID/msg。** 官方 ProtoJSON 默认使用驼峰字段名、将 64 位整数输出为字符串，并有自己的零值和 null 规则。管理台当前使用下划线字段名、数字整数、未知用量的 null、可省略的步骤属性，以及 `steps: null` 与 `steps: []` 的区别。直接替换编码器会改变这些契约。

各接口在 `app/impl_agent.go` 中直接组装自己的 Proto 响应，再由 `app/handler.go` 中对应的 JSON 适配函数转换为 `entity/agent_json.go` 的 HTTP 数据，最后包装公共外层并交给 `httpx` 编码和写出。当前三个接口的 HTTP JSON 形状相同，因此复用表现类型；各接口的 Proto 响应及适配函数保持独立，后续可分别演进。共享的步骤转换保留 null、空数组和未知用量语义，不直接对生成类型调用 `encoding/json`，也不修改生成文件标签。新增响应字段时，同时更新本接口消息、组装、JSON 表现及契约测试。后续规则见 [HTTP 共用数据规范](entity/README.md)。

当前管理台仍使用手写 TypeScript HTTP 类型，不能直接用标准 ProtoJSON 响应解码器替代现有请求封装。Go `int64` 在服务端完整保留；浏览器中的数字仍受 JavaScript 安全整数范围限制，与改造前相同。

## 修改消息的约定

1. 先修改 `.proto`，字段编号在发布后不重用；删除字段时保留其编号与名称。
2. 修改生成文件必须通过生成命令完成，不复制同步对象、不手写生成器内部结构。
3. 接口类型停留在 HTTP 适配层，通过明确转换调用领域服务。
4. Proto 类型不替代业务校验；问题数量、长度、工具权限和预算继续由现有业务处理。
5. 运行生成检查、格式检查、`go vet`、普通测试与竞态测试；全程使用假模型和内存观测器。

原理参考：[Go 消息生成说明](https://protobuf.dev/reference/go/go-generated/)、[ProtoJSON 格式](https://protobuf.dev/programming-guides/json/)、[字段存在性](https://protobuf.dev/programming-guides/field_presence/)。这些官方页面是来源资料，项目内使用说明以本中文文档为准。

## Agent Demos 接口

新增 `POST /api/agent-demos/overview`、`chat`、`messages`、`tools`、`structured`、`multiple-tools`、`stream`、`reliability`、`retrieval`、`workflow`、`observation`、`evaluation`、`delivery`，对应管理台的 13 个章节。路由逐条声明在 `app/router.go`，入口实现在 `app/impl_agent_demos.go`。原接口保留。

普通响应仍为 `code/data/traceID/msg`；`data.execution` 是执行记录，概览分别返回 `direct/withTools`，评估返回 `cases/passed/total`。请求示例与实际演示边界见 `docs/learning` 各章的“当前可以运行的 Demo”。所有工具和资料都在本地固定注册，不执行外部动作。

`stream` 返回 SSE；每条 `data:` 都是统一响应外层，末条 `data.done=true`。需检查 `code` 与 `execution.completed` 判断是否成功，收到部分内容或 HTTP 200 不能代表完成。取消请求会取消模型连接。离线测试用本机模拟服务，不读取真实密钥。
