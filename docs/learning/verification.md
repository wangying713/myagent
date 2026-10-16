# 验证与验收

## 离线检查

在项目根目录执行，测试不读取真实密钥、不连接模型或 OpenObserve：

```sh
./scripts/generate-proto --check
./scripts/go fmt ./...
./scripts/go vet ./...
./scripts/go test ./...
./scripts/go test -race ./...
```

格式命令会修改有格式问题的 Go 文件，完成后检查 diff。前端在 `apps/interfaces/console` 中执行：

```sh
npx --no-install biome check src/pages/agent-development
npm run tsc
npm run build
```

构建前停止前端开发服务，完成后重新 `npm start`。静态构建与开发服务会使用共享生成目录。

| 测试文件 | 当前验证契约 |
| --- | --- |
| [领域业务测试](../../domain/agent/service_test.go) | 三个业务方法、默认问题、历史、部分结果、取消、脱敏与观测事件 |
| [领域运行预览测试](../../infra/openobserve/agent_test.go) | `agent.run` 输入输出预览、失败保留回答、正文开关、脱敏与限长 |
| [通用 HTTP 测试](../../infra/httpx/httpx_test.go) | 非 Agent 请求复用、严格 JSON、正文上限、响应编码失败 |
| [Proto HTTP 测试](../../infra/httpx/proto_test.go) | 描述符字段名校验、严格解码、数组存在性与正文上限 |
| [HTTP 统一处理测试](../../apps/interfaces/http/app/handle_test.go) | 部分结果、路由超时隔离、上下文释放与未适配响应保护 |
| [HTTP 包装测试](../../apps/interfaces/http/app/handler_test.go) | code/data/traceID/msg、无结果为 null、解码前链路、URI 名称、HTTP 预览、正文开关、脱敏、截断及编码回退 |
| [HTTP 正文旁路采集测试](../../infra/httpx/capture_test.go) | 读写副本有界、不预读请求、不改变实际正文、状态与请求体所有权 |
| [响应兼容测试](../../apps/interfaces/http/app/impl_agent_test.go) | 下划线字段、数字整数、零值、未知用量与空步骤的 JSON 表现 |
| [循环测试](../../domain/agent/loop_test.go) | 完整 assistant、多工具回填、非法参数、预算、ID 与取消 |
| [HTTP 测试](../../apps/interfaces/http/app/router_test.go) | Gin 路由、健康检查、405/404、异常恢复、三课、父子 span、输入拒绝、部分失败、超时、观测故障隔离与并发历史隔离 |
| [服务生命周期测试](../../apps/interfaces/http/app/server_test.go) | 信号与监听异常、等待处理器结束、强制关闭、有界退出、原始错误链和退出前结束 span |
| [配置测试](../../apps/config/config_test.go) | 显式文件、环境优先级及非法配置 |
| [脱敏测试](../../infra/openobserve/telemetry_test.go) | 敏感字段、长度限制、不修改原始正文 |

## 管理台人工验收

按 [学习入口](README.md) 启动两个服务。先请求 `GET /api/health` 确认存活，再依次运行：

| 课程 | 业务检查 | 正常观测 |
| --- | --- | --- |
| 聊天 | 一次请求，非空回答，正常结束 | HTTP 根 + 领域运行 + 模型，共 3 个 |
| 消息历史 | 第二次请求包含真实第一轮 assistant，回答“豆包” | HTTP 根 + 领域运行 + 两次模型，共 4 个 |
| 工具循环 | 参数 17、23，实际结果 391，ID 配对，最终回答一致 | HTTP 根 + 领域运行 + 两次模型 + 一次工具，共 5 个 |

同时检查编辑问题、切换课程、取消按钮和失败提示；取消后应能重新执行，旧请求不能覆盖新结果。模板登录只提供演示流程，不是 Go API 身份校验。

每次复制 trace ID 查询 OpenObserve，核对同一 trace、父子关系、实际正文、状态与用量。数量按真实调用轨迹检查，不能只查到一条就认为完整入库。只有查询到相应记录，才判定该次观测验收通过。

## 2026-10-16：agent.run 输入输出预览

领域运行显式传入补齐默认值并通过校验的问题，`infra/openobserve` 将聊天和工具循环的 `question`、历史消息的两轮 `questions` 适配为输入预览，将 `completed/answer` 适配为输出预览。失败仍保留已有回答，不记录底层错误正文；正文开关、脱敏和截断规则保持一致。

格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 及 Proto 生成检查通过。离线测试覆盖三课默认输入、成功输出、部分失败、脱敏、不修改业务数据及关闭正文采集和截断。

另外使用现有配置向本机 OpenObserve 发送假模型验证 trace `8da23db6f0381df13f366dc015ec6e1c`，按 trace ID 查询确认 `agent.run` 已入库，`gen_ai_input_messages` 为问题 JSON，`gen_ai_output_messages` 包含验证回答和 `completed: true`。未调用外部模型，未输出鉴权；本轮验证了预览数据入库，未进行浏览器布局验收。已有 trace 不会补写新增预览。

## 2026-10-16：输入输出直接预览 JSON

HTTP 预览改为直接展示实际读取的请求体和写出的响应体，移除额外 `body/method/uri/status` 外层。HTTP、模型和工具共用的预览适配直接传递脱敏限长后的正文，不再生成 `role/parts/content` 消息包装；实际 JSON 内自带的消息字段继续保留。HTTP 方法、路由和状态仍在 span 属性中。

格式、vet、全量离线测试、竞态检查、协议生成检查、服务构建及差异检查通过，现有成功、错误回退、正文开关、脱敏、精度和截断契约继续通过。本机服务已重启。通过管理台代理提交 `{"question":""}`，按预期返回 HTTP 400，未调用真实模型；trace `e398fb97270347b38c5d9244cd457fab` 已入库。OpenObserve 页面及截图确认输入直接显示该 JSON，输出直接显示接口的 `code/data/traceID/msg`，没有额外 body 外层或“用户／助手”卡片。

## 2026-10-16：模型 HTTP 名称简化为 URI

按需求将模型 Client span 名称简化为 URI 路径，例如 `/v1/chat/completions`，避免与 OpenObserve 客户端行左侧的目标主机重复；`url.full` 继续保留完整地址，HTTP 方法、目标主机、模型名、预览与链路关系沿用原有上报。

格式、vet、全量离线测试、竞态检查、协议生成检查、构建及差异检查通过。三课的离线模型替身验证 URI 名称与完整地址属性相符。本机服务已重启，健康检查通过；本轮未调用真实模型或进行新模型链路的入库复验。

## 2026-10-16：模型 HTTP 完整地址展示

模型 Client span 名称由 `chat 模型名` 改为实际请求完整地址，例如 `https://api.deepseek.com/v1/chat/completions`，同时记录 `url.full`。HTTP 方法、目标主机、模型名称、操作、请求响应预览及父子链路继续保留。

格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、协议生成检查、服务构建及差异检查通过。离线 HTTP 替身验证三课中的 Client span 名称与完整地址一致，并保留目标主机、POST 方法、模型属性及原有链路关系。本机服务已重启，健康检查通过；本轮未调用真实模型，也未将旧链路视为新显示的验收证据。

## 2026-10-16：HTTP URI 名称与请求响应预览

随后按需求将默认、示例及本机的 `OTEL_SERVICE_NAME` 统一为 `agent` 并重启服务。新校验请求 trace `e175d42304cec00459b0fd98ee8ee20d` 已从 OpenObserve 查回，确认 `service_name=agent`；未调用真实模型。配置调整后格式、vet、全量测试、竞态检查、协议生成检查及构建通过。

HTTP 根 span 名称只保留注册的 URI 路由模板，例如 `/api/agent/chat`；HTTP 方法继续保存在属性中，未匹配路由显示 `[unmatched]`。根 span 增加实际读写的 HTTP 请求与响应正文，复用内容开关、脱敏、字符上限及输入输出预览字段。请求头和查询字符串不采集，采集不会预读或改变业务正文。

Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、`scripts/generate-proto --check`、服务构建及 `git diff --check` 均通过。离线测试覆盖成功、解码及领域输入拒绝、404/405、编码失败回退、数值精度、关闭正文、脱敏、截断、读写副本上限与不预读请求。

本机服务已重启。通过管理台代理提交未知字段，接口按预期返回 HTTP 400，未调用真实模型。trace `73b0e6f1df39f960f321d38cf1c1d743` 已从 OpenObserve 查回，HTTP span 为 `bcbab9be55548338`，名称为 `/api/agent/chat`。浏览器确认“预览”位于“属性”前，并实际显示请求 `unknown: true`、方法、URI 及最终错误响应的 `code/data/traceID/msg` 和 HTTP 400；该错误标记来自有意提交的非法参数。本轮未发起真实模型成功请求，成功与部分失败响应由离线测试覆盖。

## 2026-10-16：HTTP 入口名称及 traceID 查询验收

移除固定的 HTTP span 名称配置，使用 otelgin 默认的“方法 + 注册路由模板”，例如 `POST /api/agent/chat`；未知路由仅保留方法，参数路径和查询参数仍不写入名称。测试按 Server span 识别 HTTP 入口，并逐个验证三课的入口名称、响应 traceID、父子关系、失败标记和元数据脱敏。Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、Proto 生成一致性及 `git diff --check` 均通过。

使用现有本机观测配置按管理台显示的 traceID 精确查询，确认旧请求已完整入库但早于界面默认的“过去 15 分钟”；扩大到一天并使用 trace_id 字段筛选后，页面成功显示完整三层链路。随后重启后台，通过管理台代理发送一个会被严格校验拒绝的请求，返回 ID 与 OpenObserve 的实际记录一致，页面入口名称为 `POST /api/agent/chat`，HTTP 400 正确标记为失败。该验收未新增模型调用，未输出鉴权或模型正文。

## 2026-10-16：单参数 HTTP 处理验证

三条路由统一为 `handle(implAgent.Chat)`、`handle(implAgent.Messages)`、`handle(implAgent.Run)`，移除 `infra/httpx` 中的 Adapter 配置对象与多回调适配器。`app/handler.go` 直接分配请求、调用严格解码、准备消息字段、派生有界上下文并同步调用业务方法；超时在路由初始化时显式注入当前 Gin 请求，不使用全局配置，计时仍从解码及准备完成后开始。响应类型与 JSON 表现的对应关系集中在 HTTP 处理层，保留各接口独立的 Proto 响应、原有 JSON 数据形状、错误映射和部分结果。

通用解析与写出继续位于 `infra/httpx`，原适配器测试中相关契约由入口的统一处理测试、响应兼容测试及路由集成测试覆盖。新增检查验证同一个 handle 在不同路由实例间使用独立超时、返回后释放上下文、部分结果与错误并存、缺少配置或未适配响应时返回统一错误；现有测试继续验证严格校验、显式空数组、取消、并发隔离和父子链路。Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、Proto 生成一致性及 `git diff --check` 均通过。本轮只运行离线测试。

## 2026-10-16：数值精度、协议命名与 HTTP 观测依赖验证

观测脱敏在解析字符串中的 JSON 时统一使用 `Decoder.UseNumber`，并确认单个 JSON 值完整结束，展示副本保留大整数、高精度小数与极大指数；敏感字段仍被脱敏，实际正文保持原样。Proto 命名空间统一为 `saklay.http.agent`，生成脚本的临时目录同步清理旧项目名，生成文件通过脚本更新。

HTTP 请求链路改用官方 `otelgin` v0.72.0，显式注入私有 TracerProvider、noop MeterProvider 和空传播器；保留固定 span 名称、路由模板、服务内父子链路和公共响应中的实际 trace ID。原始路径、User-Agent 和客户端地址属性替换为脱敏标记，公共响应处理继续标记 4xx/5xx 和编码失败。`go.mod` 与 `go.sum` 已整理并锁定相应依赖。配置库经过评估后保留 `godotenv.Read` 与标准库解析，原因见开发约定。

Proto 生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...` 和 `git diff --check` 均通过。新增回归覆盖数值精度与原始正文保护、请求元数据脱敏及请求内容保持、编码失败链路状态，以及关闭观测时携带外部 traceparent 仍返回空 trace ID；现有测试继续验证三课的 3/4/5 个 span、部分结果、取消和退出等待。本轮仅运行离线测试，未调用真实模型或连接 OpenObserve。

## 2026-10-15：接口独立响应与显式返回验证

三个接口分别定义并返回 `ChatRsp`、`MessagesRsp`、`RunRsp`，在 `app/impl_agent.go` 内显式处理领域返回值和组装响应，移除了统一业务响应类型及返回包装函数。各接口的字段赋值允许重复，步骤与用量继续复用实际共用的消息。路由为三种响应分别配置 HTTP 适配器，JSON 表现、错误映射及 `code/data/traceID/msg` 公共外层保持原有契约。项目约定和当前示例已同步。

Proto 生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...` 及 `git diff --check` 均通过。响应兼容测试分别覆盖三种独立响应的 nil、零值、空步骤、完整字段、未知及零用量和大整数；现有路由测试继续验证三个业务流程、输入拒绝、预算、部分失败、取消及并发隔离。本轮未修改管理台或调用真实模型。

## 2026-10-15：HTTP 入口收敛与 app 包迁移验证

HTTP 顶层只保留 `main.go` 一个 Go 文件，负责参数、配置、退出信号及最终错误输出，随后调用 `app.Run(ctx, cfg)`。依赖组装及资源释放迁至 `http/app/run.go`，路由、接口实现、响应处理、服务生命周期及测试统一迁入 `http/app`（包名 `app`）；应用对外函数只有 `Run`，`entity` 和 `proto` 仍为同级目录。服务实现及测试在迁移时仅调整包名，启动命令、HTTP 协议与资源清理顺序保持原有契约。当前阅读路径、项目约定及测试命令已同步，下方记录保留此前目录演进的历史。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、文档本地链接与 `git diff --check` 均通过。`go run ./apps/interfaces/http -h` 正常输出启动参数，目录检查确认顶层只有 `main.go`。本轮未启动真实后台、未调用模型，也未修改管理台。

## 2026-10-15：Proto 源文件与生成类型归并验证

`agent.proto` 与生成的 `agent.pb.go` 统一放在 `http/proto`，Go 包名为 `proto`；`entity` 只保存手写的 HTTP 公共外层及 JSON 表现类型。生成脚本按新的 `go_package` 输出到同目录，旧的 `entity/agent.pb.go` 已移除。接口、转换和测试直接使用 `proto.ChatReq`、`proto.MessagesReq`、`proto.RunReq` 与 `proto.AgentRsp`，项目约定及当前阅读路径已同步；下方记录保留此前目录演进的历史。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、文档本地链接、依赖边界和 `git diff --check` 均通过。本轮只调整生成位置与 Go 导入，HTTP JSON、公共外层和领域行为保持既有契约；未修改管理台或调用真实模型。

## 2026-10-15：Req/Rsp 与 HTTP 统一响应验证

Proto 请求消息统一为 `ChatReq`、`MessagesReq`、`RunReq`，三个接口共用业务响应 `AgentRsp`。生成文件通过脚本更新，移除的业务 `trace_id` 字段编号与名称已保留。接口方法继续返回 `(rsp, error)`；`http/handler.go` 统一包装 `entity.Rsp[T]` 的 `code/data/traceID/msg`，成功 code 为 `ok`，失败保留原有 HTTP 状态与字符串错误码，无结果使用 `data: null`，有部分执行结果则保留。

HTTP 解码前建立请求 span，领域运行挂到 HTTP 请求下，模型和工具仍挂到领域运行下。正常聊天、历史与典型工具流程分别为 3、4、5 个 span；外层 trace ID 对应实际链路，关闭观测时为空。退出闸门直接返回的 503 尚未进入路由，也使用四字段外层和空 trace ID。管理台已适配新外层、无结果错误及部分结果展示。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、前端三文件 Biome、TypeScript 类型检查及生产构建均通过。新增和更新的离线测试验证四字段外层、成功、解码与领域校验拒绝、404/405、真实 trace 关联、HTTP 与领域父子关系、关闭观测、编码回退；现有部分失败、预算、取消、并发隔离和退出等待测试继续通过。本轮未调用真实模型、未验证 OpenObserve 入库，也未进行浏览器交互验收。

## 2026-10-15：Agent 接口实现合并验证

原 `http/agent` 包的处理方法、参数转换、响应映射、JSON 适配及业务错误映射已合并到 `http/impl_agent.go`，与 HTTP 入口同属 `main` 包。内部类型使用 `agentAPI`，辅助函数保持私有并带 Agent 业务含义。响应兼容测试迁至 `http/impl_agent_test.go`；`router.go` 继续逐条声明完整路由并通过 `httpx.Adapt` 绑定方法。

请求响应数据仍在 `entity`，协议源文件仍在 `proto`。原业务包的使用说明合并到 HTTP 入口 README，项目约定、生成脚本提示、学习路径和测试命令同步更新。下方保留此前迁移时的目录记录，当前组织方式以本节及项目约定为准。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...` 和 `git diff --check` 均通过。现有离线契约测试覆盖严格 JSON、空数组、零值、未知用量、部分结果、取消及并发隔离；本轮未调用真实模型或进行浏览器验收。

## 2026-10-15：HTTP 请求响应统一归属验证

`http/proto/agent.proto` 作为消息定义来源，生成脚本按 `go_package` 和模块输出模式生成 `http/entity/agent.pb.go`。三个 API 方法统一使用 `entity` 中生成的请求和响应类型，手写 HTTP JSON 表现类型位于 `entity/agent_json.go`；请求、领域结果及 JSON 数据之间的转换合并至 `agent/mapping.go`。字段准备及业务错误映射继续由 `http/agent` 负责。生成脚本、导入、阅读路径和项目约定已同步，后续新增接口沿用这一分工。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、相关 Markdown 本地链接、目录分工检查及 `git diff --check` 均通过。包依赖检查确认 `entity` 不依赖领域、Gin 或基础设施，领域与 `httpx` 也不依赖 `entity`。手写 JSON 数据定义与迁移前逐字一致，现有响应兼容测试继续覆盖数字整数、零值、未知用量、null 与空数组；本轮未调用真实模型或进行浏览器验收。

## 2026-10-15：Agent 接口目录整理验证

Agent 的处理方法、请求响应转换、JSON 适配、业务错误映射及响应契约测试统一迁入 `apps/interfaces/http/agent`，Go 包名为 `agentapi`；原 `http.go` 更名为 `adapter.go`。移除中间的 `api` 目录，`http/proto` 保存第一版 Proto 消息，完整路由仍由 `http/router.go` 显式绑定。导入、学习资料、生成工具的说明路径和项目约定已同步。

生成一致性、Go 格式、`go vet ./...`、`go test ./...`、`go test -race ./...`、相关 Markdown 本地链接及 `git diff --check` 均通过。URL、消息定义、JSON 格式及领域业务保持现有契约；本轮仅做目录与包组织调整，未调用真实模型或进行浏览器验收。

## 2026-10-15：评审收尾与退出流程验证

入口已拆分为参数与配置加载、依赖组装、HTTP 生命周期三个职责。`server.go` 在退出信号和监听异常时统一取消请求、停止接收业务、等待处理器结束，并在共用的 10 秒期限内清理；超时强制关闭连接、报告仍在运行的请求，不将连接关闭当作业务完成。底层监听、停止和关闭错误通过错误链保留。正常请求结束后才释放模型客户端，并用独立 5 秒期限刷新观测；未结束请求的强制退出会明确提示观测可能缺失。

新增离线生命周期测试验证了信号取消、监听异常、保留原始错误、处理器不响应取消时有界退出、连接先关闭时仍等待处理器、退出期间拒绝新业务请求，以及退出返回前运行与模型 span 均已结束。普通取消测试使用较长截止时间，和 50ms 超时用例分开。旧文件的三组导入顺序与配置注释已同步。

项目手写 Go 文件的导入分组检查、生成一致性检查、格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 与 `git diff --check` 均通过。本轮未启动实际后台或管理台，也未调用真实模型或验证 OpenObserve 入库；网络测试仅使用本机临时监听与假模型。

## 2026-10-15：API 方法签名与统一适配验证

聊天、消息历史和工具循环 API 已统一为 `(ctx, req) → (rsp, err)`，响应通过返回值提供。路由逐条使用 `httpx.Adapt` 绑定具名方法，适配器负责解析、超时、取消、响应写出和上下文释放；JSON 编码及业务错误映射由 API 显式注入。领域方法和三个 HTTP URL 保留现有行为。

新增通用适配器测试使用与 Agent 无关的标准生成请求消息，验证成功响应、响应与错误同时返回、nil 响应、解码失败提前拒绝、显式字段准备、超时、调用方取消、返回后释放上下文和并发隔离。原有响应 JSON 契约测试改为通过适配器执行；三个接口的默认与自定义问题、显式空数组拒绝、预算中止后的部分记录、脱敏、取消和观测父子关系测试继续通过。

生成一致性检查、Go 格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 和 `git diff --check` 均通过。本轮测试使用假模型及内存观测器，未修改管理台、未启动服务，也未进行真实模型或浏览器验收。

## 2026-10-15：Proto 消息接入验证

三个接口的请求、响应、步骤与用量已在 `apps/interfaces/http/proto/agent.proto` 定义并生成 Go 消息。Gin 路由、HTTP URL、公共响应和领域业务保持原有分工，领域层不依赖生成包。请求字段名从消息描述符提取，响应通过 API 私有格式适配保留现有 JSON；具体边界见 [Proto 使用说明](../../apps/interfaces/http/README.md)。

`scripts/generate-proto --check`、Go 格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 和 `git diff --check` 均通过。编译器固定为 protoc 29.3，生成器与运行时共用现有 v1.36.12 依赖，未升级其他依赖。

新增与扩展的离线测试验证了三个接口的默认与自定义问题、历史问题数组的省略和显式空数组、未知字段与别名、重复字段、null、类型错误和正文超限。响应契约测试核对下划线字段名、必需零值、数字整数、未知与零用量、步骤可选字段及 null/空数组；原有预算、取消、错误脱敏、观测父子关系和并发隔离测试继续通过。

本轮未修改管理台源码，未启动服务或进行真实模型调用、浏览器点击与 OpenObserve 入库复验。通过离线契约测试确认现有调用格式，不将其表述为真实环境验收通过。

## 2026-10-15：HTTP 与领域分层改造验证

三个接口已改为 `/api/agent/chat`、`/api/agent/messages`、`/api/agent/run`；健康检查为 `/api/health`。`router.go` 逐条声明完整路径，`http/agent` 定义请求、业务响应和具名处理方法，三个流程移入 `domain/agent`。`apps/demos` 已移除。

通用 HTTP 解析、公共响应和传输错误已提取到 `infra/httpx`。领域层不依赖 Gin、OpenTelemetry 或 `infra`，通过领域定义的观测接口接收 OpenObserve 适配。API 保留现有业务响应字段（包括 `lesson`），路由级错误的 `data` 为 null，不绑定 Agent 响应类型。

Go 格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 通过；前端本次三个修改文件的 Biome、TypeScript 类型检查和构建通过。离线模型 HTTP 服务及内存观测器验证了三个流程、父子 span、历史和工具回填、预算、取消、部分失败、脱敏、观测失败隔离与并发隔离。新增领域测试直接使用内存替身，不导入基础设施；`httpx` 测试用非 Agent 的目录请求验证复用及编码失败行为。

本机前后端已启动：页面 HTTP 200，健康检查代理正常；通过管理台代理请求三个新接口，非法 JSON 值均被拒绝为 HTTP 400；旧 Go 接口返回 HTTP 404。这些请求不调用模型。本轮未进行真实模型成功路径复验或浏览器点击、视觉验收。

## 2026-10-15：Gin 迁移验证

全部现有接口已使用 Gin v1.12.0。格式检查、`go vet ./...`、`go test ./...`、`go test -race ./...` 与服务构建通过。三课的离线契约、历史与工具回填、取消、预算、观测父子关系及并发隔离通过；新增路由、健康检查、请求方法、内容类型、超限尾随空白和异常恢复检查通过。

本机服务已重启，管理台健康检查代理通过。通过代理两次尝试真实聊天，均在 60 秒单次模型超时处返回 HTTP 504，未得到完整模型回答；DNS、TCP 与 TLS 连通检查通过，但不足以确定模型请求超时原因。本轮未完成三课真实成功路径复验，不沿用之前结果判定其通过。

超时 trace `16899d95d9f46f8409ca1d61840b0b4c` 已从 OpenObserve 查回 2 个 span，运行根节点与实际模型子节点均记录失败，父子关系正确。该次 1 次模型请求、0 次工具调用，用量未知；这只证明该次失败路径与观测入库，不证明业务成功。

本次未修改前端页面，也未进行新的浏览器点击验收。

## 2026-10-14：Go 初次迁移证据

2026-10-14 迁移时，Go 格式、vet、离线测试和竞态检查通过，前端 Biome、类型检查及构建通过。真实模型三课通过，下面三个 trace 已查回并核对 span 数量、根节点和父子关系：

| 课程 | 模型 / 工具 | span 数 | trace ID |
| --- | --- | --- | --- |
| 聊天 | 1 / 0 | 2 | `8515e3359e8e9018c4f249b996664bca` |
| 消息历史 | 2 / 0 | 3 | `282556fff1ee28ff2af045fb35854691` |
| 工具循环 | 2 / 1 | 4 | `738c6a2dae1ee5e6a950c5b88ca18def` |

最终服务重启后，管理台路由访问和代理真实工具请求通过，工具结果为 391；该次 trace 为 `6b27e70f96f5965d9eb59734704a614c`，这里不额外声称已完成该次入库查询。

浏览器控制通道当时无法启动，未完成自动点击或视觉验收。HTTP 访问、代理与构建通过不能替代浏览器交互检查。以上是迁移时的证据，不代表后续修改自动通过；观测记录也可能因保留策略过期。

旧 Python 结果见 [历史记录](history/README.md)，与 Go 结果分开。流式、RAG、持久会话、思考模式和批量评估尚未实现，不能依据旧记录判定 Go 功能完成。
