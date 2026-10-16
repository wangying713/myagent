# 项目开发约定

- 本项目用于学习 Go Agent 开发，开发遵循 `docs/development` 中的核心规范、具体规范和最佳实践，注意三者的规范层级。
- 按任务应用该目录的 Go 风格、错误处理、测试和并发技能。中文解释、注释、练习和运行提示；代码标识符保留 Go 惯用写法。
- 手写 Go 文件导入按“标准库 → 本项目包（`saklay/ai`）→ 外部依赖”分组，组间空一行，组内由 `gofmt` 排序；生成文件不手改。具体见 `docs/development/go-decisions.zh-CN.md`。
- 手写 Go 代码（含测试）的非空代码块统一展开，每条语句单独一行，包括函数、方法及 `defer`、`go`、回调中的匿名函数；不把函数体压成单行或用分号拼接语句。具体见 `docs/development/go-decisions.zh-CN.md` 的“代码块与语句换行”。
- 手写 Go 代码（含测试）的复合字面量包含多个元素或字段时，每项单独一行；嵌套展开时外层同步展开，保留尾逗号并由 `gofmt` 格式化。空字面量和简单单项可保留单行。具体见 `docs/development/go-decisions.zh-CN.md` 的“复合字面量换行”。
- 业务操作、角色、结束原因等固定取值使用所属包的具名常量，多次引用时复用定义；业务阈值和可配置默认值集中管理。个别语义明确的局部字面量、协议声明及独立测试样本可保留，不要求全面常量化。具体见 `docs/development/go-decisions.zh-CN.md` 的“具名常量与字面量”。
- `apps/config` 负责启动配置加载、默认值和校验。`apps/interfaces` 负责入口适配：`http` 放后台服务 API，`console` 放管理台，`cli` 继续预留；本次不新增命令行课程。前端构建和运行配置放在 `console` 内。
- `domain/agent` 负责聊天、消息历史、工具调用业务及协议、乘法工具与有限循环；`infra` 负责 DeepSeek、OpenObserve 和可复用的 `httpx` 请求解析、响应编码。各组件配置保留在所属包，由 Go 入口显式传入；领域层不读取环境或使用隐藏的全局依赖，不导入 Gin、OpenTelemetry 或 `infra`；观测能力通过使用方定义的小接口注入。
- Go HTTP 接口统一使用 Gin，`apps/interfaces/http` 顶层只保留 `main.go` 一个 Go 文件，负责启动参数、配置加载、退出信号和最终错误输出。服务实现与测试集中在 `http/app`（包名 `app`），启动入口为 `Run(ctx, cfg)`；`app/run.go` 组装依赖并释放资源，`app/server.go` 管理 HTTP 生命周期，`app/router.go` 声明路由和中间件。Agent 请求准备、接口实现及各自的 Proto 响应组装放在 `app/impl_agent.go`，HTTP JSON 表现及错误映射位于 `app/handler.go`；后续业务使用 `app/impl_xxx.go`。通用 HTTP 能力位于 `infra/httpx`，不依赖业务包。路由逐条声明完整路径并绑定具名方法，禁止用课程字符串动态分发。业务层显式接收 `context.Context`，不依赖 `gin.Context`；保留严格 JSON 校验、稳定错误响应及现有取消和观测契约。
- HTTP API 的协议源文件与生成的请求响应类型统一放在 `http/proto`，手写 HTTP 公共外层与 JSON 表现类型放在 `http/entity`。后续新增接口按业务分文件：`.proto` 与 `.pb.go` 同目录，各接口独立定义响应，按需复用步骤、用量等共用消息；手写共用数据也按业务分文件。`entity` 只承载接口数据，不依赖业务处理包、领域服务、Gin 或基础设施；请求准备及响应组装放在对应业务的 `app/impl_xxx.go`，接口显式返回自己的 Proto 响应；允许重复的字段赋值，不封装统一业务响应处理函数。HTTP JSON 适配和错误映射由 `app/handler.go` 处理。领域层和 `infra/httpx` 不依赖应用的 `entity` 或 `proto` 包。
- API 方法统一为 `(ctx, req) (rsp, error)`，响应通过返回值提供；通过 `handle(implAgent.Chat)` 这类单参数调用接入 Gin，方法内不解析 HTTP 或直接写响应。请求消息统一用 `Req` 后缀，响应消息用 `Rsp` 后缀；Agent 分别使用 `ChatReq/ChatRsp`、`MessagesReq/MessagesRsp`、`RunReq/RunRsp`。`http/app/handler.go` 集中处理请求准备、JSON 适配、业务错误映射和统一 HTTP 写出；超时在路由初始化时通过中间件显式设置，解码及请求准备完成后才开始计时，不使用全局配置，外层结构为 `entity.Rsp[T]` 的 `code/data/traceID/msg`，成功 code 为 `ok`；公共响应码常量位于 `entity/code.go`，应用代码使用具名常量，领域错误判断、HTTP 状态和提示保留在 `app/handler.go`。无结果时 data 为 null，部分结果与错误同时返回时必须保留结果。具体见 `docs/development/go-decisions.zh-CN.md`。
- Agent API 消息在 `http/proto/agent.proto` 定义，通过 `scripts/generate-proto` 生成 `http/proto/agent.pb.go`，禁止手改 `.pb.go`；提交前运行 `scripts/generate-proto --check`。API 方法使用 `proto` 中生成的请求和响应类型，领域层不依赖 Protobuf。HTTP JSON 表现类型位于 `entity/agent_json.go`，业务 data 通过 API 的私有转换保留数字、零值、null 与字段名等契约，不能直接换成标准 ProtoJSON。HTTP trace 在解码前建立，traceID 只放统一外层；领域运行、模型与工具沿用同一链路，关闭观测时为空。
- 当前只实现聊天、消息历史和受控工具循环三课及 OpenObserve Traces。默认问题、提示词与流程已迁入 Go；学习资料以 `docs/learning` 为入口，未实现专题只保留学习路线，不据此扩展业务代码。
- `old` 为废弃的历史归档，不参与 Go 构建，不作为新开发依据；服务只读取显式 `-env-file` 指定的新配置，不自动读取旧凭据。
- 测试默认离线，不读取真实密钥、不连接外部模型。新增或修改行为时验证相应契约；提交前运行格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...`。
- 不提交 `.env` 或真实凭据，不自动覆盖已有配置，不把未知工具或模型生成的文本直接执行。
