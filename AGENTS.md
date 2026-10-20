# 项目开发约定

- 本项目用于学习 Go Agent 开发，开发遵循 `docs/development` 中的核心规范、具体规范和最佳实践，注意三者的规范层级。
- 按任务应用该目录的 Go 风格、错误处理、测试和并发技能。中文解释、注释、练习和运行提示；代码标识符保留 Go 惯用写法。
- 注释先按声明类型和是否成组确定位置，再判断信息量：包、类型、函数、方法（包括接口方法）及独立单条 `const`/`var` 声明的注释均独立放在上方，仅含一项的括号声明同样处理；结构体字段和成组常量、变量的简短说明放行尾，内容较多时放上方。独立注释可占一行或多行，不以注释原有行数或固定字符数判断，也不为行尾注释硬凑声明分组。编译指令和工具读取的注释保持原位。具体见 `docs/development/go-decisions.zh-CN.md` 的“注释位置与简洁性”。
- 手写 Go 结构体的字段补齐中文说明，包括内部辅助类型、匿名结构体和测试数据；说明业务含义、单位及必要的零值或 nil 语义，简短注释放行尾，生成文件与历史归档不手改。具体见 `docs/development/go-decisions.zh-CN.md` 的“结构体字段说明”。
- 手写 Go 函数和方法的参数最多 4 个，包括接口方法、函数类型、匿名函数和测试辅助函数；`ctx` 计入，接收者与返回值不计，`a, b string` 按两个参数计算。达到 5 个时，先检查职责，再把相关输入合并为具名结构体；`ctx` 仍单独作为首参。外部接口要求固定签名的实现和回调沿用外部约定。具体见 `docs/development/go-decisions.zh-CN.md` 的“函数与方法参数数量”。
- 新增或修改的手写 Go 函数和方法最多 90 行，匿名函数、测试和测试辅助函数同样遵循。按 `gofmt` 后从函数签名到结束大括号计算，包含空行、内部注释和嵌套代码，上方文档注释不计；按职责拆分，不压缩代码或删注释凑行数。存量超限函数在后续涉及该函数时治理，不据此进行无关的全仓重写。具体见 `docs/development/go-decisions.zh-CN.md` 的“函数与方法行数”。
- 函数、方法只有一个业务请求或执行输入对象时，该参数统一命名为 `req`，如 `Run(ctx, req)`；领域方法、模型和工具调用、接口声明及相关测试保持一致。`ctx` 仍为首参，配置、依赖、响应和普通辅助值继续按含义命名，不把它们一律改成 `req`，也不因此修改输入类型名。具体见 `docs/development/go-decisions.zh-CN.md` 的“请求参数命名”。
- 手写 Go 代码（含测试）中的客户端字段、函数参数和局部变量统一按“用途或服务名 + `Cli`”命名，如 `agentCli`、`chatCli`、`deepseekCli`、`httpCli`；导出字段使用 `ChatCli` 这类大写形式，同一用途的字段、构造参数和调用处保持一致。类型名 `Client`、`ChatClient` 和构造函数 `NewClient` 保持原有命名；模型名称、配置、观测元数据等数据字段不因同名而改为 `Cli`。生成文件、第三方代码及 `old` 历史归档不手改。具体见 `docs/development/go-decisions.zh-CN.md` 的“客户端变量命名”。
- 手写 Go 文件导入按“标准库 → 本项目包（`saklay/ai`）→ 外部依赖”分组，组间空一行，组内由 `gofmt` 排序；生成文件不手改。具体见 `docs/development/go-decisions.zh-CN.md`。
- 手写 Go 代码（含测试）的非空代码块统一展开，每条语句单独一行，包括函数、方法及 `defer`、`go`、回调中的匿名函数；不把函数体压成单行或用分号拼接语句。具体见 `docs/development/go-decisions.zh-CN.md` 的“代码块与语句换行”。
- 手写 Go 代码（含测试）的非空结构体字面量统一展开，每个字段单独一行，即使只有一个字段也不写成单行；切片、数组和映射包含多个元素时，每项单独一行。嵌套展开时外层同步展开，保留尾逗号并由 `gofmt` 格式化。空字面量可保留单行，切片、数组和映射的简单单元素也可保留单行。具体见 `docs/development/go-decisions.zh-CN.md` 的“复合字面量换行”。
- 业务操作、角色、结束原因等固定取值使用所属包的具名常量，多次引用时复用定义；业务阈值和可配置默认值集中管理。个别语义明确的局部字面量、协议声明及独立测试样本可保留，不要求全面常量化。具体见 `docs/development/go-decisions.zh-CN.md` 的“具名常量与字面量”。
- `apps/config` 负责启动配置加载、默认值和校验。`apps/interfaces` 负责入口适配：`http` 放后台服务 API，`console` 放管理台，`cli` 继续预留；本次不新增命令行课程。前端构建和运行配置放在 `console` 内。
- `domain/agent` 负责聊天、消息历史、工具调用业务及协议、乘法工具与有限循环；`infra` 负责 DeepSeek、OpenObserve 和可复用的 `httpx` 请求解析、响应编码。各组件配置保留在所属包，由 Go 入口显式传入；领域层不读取环境或使用隐藏的全局依赖，不导入 Gin、OpenTelemetry 或 `infra`；观测能力通过使用方定义的小接口注入。
- Go HTTP 接口统一使用 Gin，`apps/interfaces/http` 顶层只保留 `main.go` 一个 Go 文件，负责启动参数、配置加载、退出信号和最终错误输出。服务实现与测试集中在 `http/app`（包名 `app`），启动入口为 `Run(ctx, cfg)`；`app/run.go` 组装依赖并释放资源，`app/server.go` 管理 HTTP 生命周期，`app/router.go` 声明路由和中间件。Agent 请求准备、接口实现及各自的 HTTP 响应组装放在 `app/impl_agent.go`，HTTP JSON 表现及错误映射位于 `app/handler.go`；后续业务使用 `app/impl_xxx.go`。通用 HTTP 能力位于 `infra/httpx`，不依赖业务包。路由逐条声明完整路径并绑定具名方法，禁止用课程字符串动态分发。业务层显式接收 `context.Context`，不依赖 `gin.Context`；保留严格 JSON 校验、稳定错误响应及现有取消和观测契约。
- HTTP API 的请求、响应、公共外层及共用数据统一手写在 `http/entity`，按业务分文件，各接口独立定义响应，按需复用步骤、用量等共用类型。`entity` 只承载接口数据和 JSON 标签，不依赖业务处理包、领域服务、Gin 或基础设施；请求准备及响应组装放在对应业务的 `app/impl_xxx.go`，接口显式返回自己的响应；允许重复字段赋值，不封装统一业务响应处理函数。HTTP 解码和错误映射由 `app/handler.go` 处理。领域层和 `infra/httpx` 不依赖应用的 `entity` 包。
- API 方法统一为 `(ctx, req) (rsp, error)`，响应通过返回值提供；通过 `handle(implAgent.Chat)` 这类单参数调用接入 Gin，方法内不解析 HTTP 或直接写响应。请求消息统一用 `Req` 后缀，响应消息用 `Rsp` 后缀；Agent 分别使用 `ChatReq/ChatRsp`、`MessagesReq/MessagesRsp`、`RunReq/RunRsp`。`http/app/handler.go` 集中处理请求解码、业务错误映射和统一 HTTP 写出；超时在路由初始化时通过中间件显式设置，解码完成后才开始计时，不使用全局配置，外层结构为 `entity.Rsp[T]` 的 `code/data/traceID/msg`，成功 code 为 `ok`；公共响应码常量位于 `entity/code.go`，应用代码使用具名常量，领域错误判断、HTTP 状态和提示保留在 `app/handler.go`。无结果时 data 为 null，部分结果与错误同时返回时必须保留结果。具体见 `docs/development/go-decisions.zh-CN.md`。
- Agent API 使用 `http/entity` 中手写的请求和响应类型，领域层使用自己的类型。请求通过 `httpx.DecodeJSON` 按准确 JSON 标签严格解码，保留省略、空数组、数字类型及 null 拒绝规则；响应直接编码，保留数字、零值、null 与字段名等契约，不维护第二套中间协议类型。HTTP trace 在解码前建立，traceID 只放统一外层；领域运行、模型与工具沿用同一链路，关闭观测时为空。
- 保留聊天、消息历史、受控工具循环原 API 及 OpenObserve Traces；经用户明确安排，管理台 Agent Demos 按学习路线提供 13 个最小可运行案例。学习资料以 `docs/learning` 为入口，未在案例中实现的扩展主题不自动新增业务。
- `old` 为废弃的历史归档，不参与 Go 构建，不作为新开发依据；服务只读取显式 `-env-file` 指定的新配置，不自动读取旧凭据。
- 测试默认离线，不读取真实密钥、不连接外部模型。新增或修改行为时验证相应契约；提交前运行格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...`。
- 不提交 `.env` 或真实凭据，不自动覆盖已有配置，不把未知工具或模型生成的文本直接执行。

- 管理台「Agent Demos」按现有学习章节组织二级菜单，支持文档、原文链接和页内接口请求；入口实现放在 `app/impl_agent_demos.go`，请求和响应放在 `entity/agent_demos.go`。原「API」页及后台继续保留。纯文档任务仍只处理文档及阅读入口，不据此增删已有业务功能。

- 文本流式 Demo 采用明确的 SSE 适配：API 方法 `(ctx, req)` 返回同步响应序列，传输层负责逐条写出统一外层；最终事件 `done=true`，成功与否由 `code` 和执行状态决定。取消沿当前上下文传播，不启动脱离请求的生成协程。普通 JSON 接口沿用现有签名与响应规则。
