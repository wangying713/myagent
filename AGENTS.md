# 项目开发约定

- 本项目用于学习 Go Agent 开发，开发遵循 `docs/development` 中的核心规范、具体规范和最佳实践，注意三者的规范层级。
- 按任务应用该目录的 Go 风格、错误处理、测试和并发技能。中文解释、注释、练习和运行提示；代码标识符保留 Go 惯用写法。
- 手写 Go 文件导入按“标准库 → 本项目包（`saklay/ai`）→ 外部依赖”分组，组间空一行，组内由 `gofmt` 排序；生成文件不手改。具体见 `docs/development/go-decisions.zh-CN.md`。
- `apps/config` 负责启动配置加载、默认值和校验。`apps/interfaces` 负责入口适配：`http` 放后台服务 API，`console` 放管理台，`cli` 继续预留；本次不新增命令行课程。前端构建和运行配置放在 `console` 内。
- `domain/agent` 负责聊天、消息历史、工具调用业务及协议、乘法工具与有限循环；`infra` 负责 DeepSeek、OpenObserve 和可复用的 `httpx` 请求解析、响应编码。各组件配置保留在所属包，由 Go 入口显式传入；领域层不读取环境或使用隐藏的全局依赖，不导入 Gin、OpenTelemetry 或 `infra`；观测能力通过使用方定义的小接口注入。
- Go HTTP 接口统一使用 Gin，路由与中间件位于 `apps/interfaces/http`。HTTP 入口下按业务组织接口适配，Agent 处理方法、请求响应转换及业务错误映射位于 `http/agent`（包名 `agentapi`）。通用 HTTP 能力位于 `infra/httpx`，不依赖业务包。路由逐条声明完整路径并绑定具名方法，禁止用课程字符串动态分发。业务层显式接收 `context.Context`，不依赖 `gin.Context`；保留严格 JSON 校验、稳定错误响应及现有取消和观测契约。
- HTTP 手写共用数据定义放在 `http/entity`，协议定义和生成文件放在 `http/proto`，后续新增接口沿用该规范：手写数据类型按业务分文件，Proto 消息直接放在 `http/proto` 下，按业务分文件，多个接口复用类型。`entity` 只承载接口数据，不依赖业务处理包、领域服务、Gin 或基础设施；转换、字段准备及错误映射留在业务适配包。领域层和 `infra/httpx` 不依赖 `entity`。
- API 方法统一为 `(ctx, req) (rsp, error)`，响应通过返回值提供；通过 `httpx.Adapt` 接入 Gin，方法内不解析 HTTP 或直接写响应。JSON 格式与业务错误映射由 API 注入，部分结果与错误同时返回时必须保留结果。具体见 `docs/development/go-decisions.zh-CN.md`。
- Agent API 消息在 `http/proto/agent.proto` 定义，通过 `scripts/generate-proto` 生成，禁止手改 `.pb.go`；提交前运行 `scripts/generate-proto --check`。生成类型只用于接口适配层，领域层不依赖 Protobuf。响应通过 API 的私有 JSON 适配保留数字、零值、null 与字段名等现有契约，不能直接换成标准 ProtoJSON。
- 当前只实现聊天、消息历史和受控工具循环三课及 OpenObserve Traces。默认问题、提示词与流程已迁入 Go；学习资料以 `docs/learning` 为入口，未实现专题只保留学习路线，不据此扩展业务代码。
- `old` 为废弃的历史归档，不参与 Go 构建，不作为新开发依据；服务只读取显式 `-env-file` 指定的新配置，不自动读取旧凭据。
- 测试默认离线，不读取真实密钥、不连接外部模型。新增或修改行为时验证相应契约；提交前运行格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...`。
- 不提交 `.env` 或真实凭据，不自动覆盖已有配置，不把未知工具或模型生成的文本直接执行。
