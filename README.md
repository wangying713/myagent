# Go Agent 后台与管理台

根据用户指定的三个 Python demo 迁移：普通聊天、携带历史的两轮对话、乘法工具调用循环。管理台「API」页保留这三个接口的调试能力；「Agent Demos」按学习路线提供 13 个最小可运行案例、说明文档与相关原文。

HTTP 接口统一使用 Gin，当前锁定 v1.12.0；请求链路由官方 `otelgin` 中间件管理，显式注入私有观测配置；标准库 `http.Server` 负责超时和优雅退出。模型客户端继续使用标准库 HTTP/JSON，Agent 循环保持显式实现。

## 启动

Go 工具链为 1.27.2，模块为 `saklay/ai`。`./scripts/go` 使用项目现有工具链，不覆盖系统安装。

准备根目录 `.env`：可从 [.env.example](.env.example) 创建并填写模型及观测鉴权；已有文件不要覆盖。服务通过 `-env-file` 显式加载，环境变量优先，不自动读取 `old` 中的配置。真实凭据已被 Git 忽略。

```sh
# 同时启动或重启 Go 后台（8080）和 Node 管理台（8000）。
./scripts/server
```

打开 [Agent Demos](http://127.0.0.1:8000/agent-demos/overview) 阅读学习资料；需要调试原有接口时，打开 [API](http://127.0.0.1:8000/api-explorer)。开发环境预填演示账号 `admin` / `ant.design`。API 页保留参数编辑、发送与取消请求、响应和 trace ID 展示。其他官方 Demos、个人页、欢迎页和 AI 助手保留。

直接执行 `./scripts/server` 即可，默认整套重启；在 `scripts` 目录执行 `./server` 也可以。默认显式传入项目根目录的 `.env`，也可用 `./scripts/server restart -env-file 配置文件路径` 指定其他文件。Go 后台先编译最新代码，再优雅停止旧进程并启动；Node 前端复用 `npm start`，停止时一并通知其子进程退出。未运行的服务直接启动，成功前检查 Go 健康接口、管理台页面和前端到后台的代理。

前端依赖需要先在 `apps/interfaces/console` 执行一次 `npm ci`；脚本不会自动安装依赖。缺少依赖或前端端口被其他进程占用时，会在重启 Go 前报错。日志分别位于 `.runtime/http/server.log` 和 `.runtime/console/server.log`。

前后端统一通过同一个脚本管理：

```sh
./scripts/server start
./scripts/server restart
./scripts/server status
./scripts/server stop
```

`start` 启动缺失的服务，已运行的服务保持运行；`restart` 重新编译 Go 并重启前后端；`stop` 停止前后端；`status` 同时显示两者状态，任一未运行时返回非零退出码。脚本只管理自己记录并核对过启动命令的进程；若此前在终端通过 `go run` 或 `npm start` 启动，请先在那个终端按 `Ctrl+C`，再用脚本启动。需要手动调试时仍可分别使用 `./scripts/go run ./apps/interfaces/http -env-file .env` 和前端目录中的 `npm start`。

## 三课与原始文件

| 课程 | 原始 Python demo | Go 流程 | HTTP 接口 |
| --- | --- | --- | --- |
| 普通聊天 | `old/agent-learning/demos/demo_01_chat.py` | system + user → 一次模型请求 → 完整回答 | `POST /api/agent/chat` |
| 消息历史 | `old/agent-learning/demos/demo_02_messages.py` | 第一轮保存实际 assistant → 第二轮回传完整历史 | `POST /api/agent/messages` |
| 工具循环 | `old/agent-learning/demos/demo_03_agent_loop.py` | 模型提出 multiply → Go 校验并执行 → 按调用 ID 回填 → 模型回答 | `POST /api/agent/run` |

默认问题和提示词保留原 demo 内容。原三课工具为本地乘法；新增 Demo 另有内置商品查询。不引入 Agent 框架，不执行模型生成的代码。`old` 不参与 Go 构建。

## 目录

```text
apps/
  config/            # 显式文件与环境变量配置
  interfaces/
    http/            # 启动入口、显式路由；app 放接口适配，entity 放手写请求响应类型
    console/         # 通用管理台；agent-development 为 13 章 Demo 页面
    cli/             # 预留，本次不新增 CLI 课程
domain/agent/        # 基础流程、13 章最小演示、执行记录、协议和有限工具循环
infra/
  httpx/             # 通用严格 JSON 解析、响应编码与传输错误
  deepseek/          # 标准库 HTTP 模型客户端
  openobserve/       # OTLP/HTTP Traces、脱敏、生命周期
docs/
  development/       # 中文 Go 和前端规范
  learning/          # Go 学习教程、后续路线与历史记录
```

学习从 [中文学习指南](docs/learning/README.md) 开始，13 章包含当前演示的操作和边界；章节大纲保留进一步学习方向。HTTP 顶层只有 `main.go`，加载配置后调用 `app.Run(ctx, cfg)`。阅读顺序：`apps/interfaces/http/main.go` → `apps/interfaces/http/app/run.go` → `app/router.go` → `app/impl_agent.go` → `domain/agent/chat.go`、`messages.go`、`demos.go` → `infra/deepseek/client.go`。工具协议与循环见 `domain/agent/tools.go`、`chat.go`，模型与工具的执行记录见 `agent_run.go`。API、错误码和边界见 [HTTP 接口说明](apps/interfaces/http/README.md)；页面运行说明见 [管理台说明](apps/interfaces/console/README.md)。

旧学习资料已迁入 `docs/learning`，对应关系见 [迁移说明](docs/learning/migration.md)。`old` 为废弃的历史归档，学习与运行不再依赖它。

HTTP 请求、响应和公共外层统一定义在 [entity](apps/interfaces/http/entity/README.md)，API 方法使用手写的 `ChatReq/ChatRsp`、`MessagesReq/MessagesRsp`、`RunReq/RunRsp`，各自显式组装响应；HTTP handler 统一包装 `code/data/traceID/msg`，trace ID 只位于外层。请求按 JSON 标签严格解码，响应直接编码，保留数字、零值、null 与空数组等表现。具体见 [HTTP 接口说明](apps/interfaces/http/README.md)。

## 观测与边界

每次 HTTP 请求建立根 span，领域运行是其子 span，每次模型请求再创建领域运行的子 span；工具直接执行，参数、结果和耗时保留在运行结果的 steps 中，不创建独立工具 span。观测异步上报；页面给出 trace ID，但不声称已入库。默认 OpenObserve 本机界面为 `http://localhost:5080/web/`，组织与 stream 由 `.env` 配置确定。

默认总运行超时 120s、单次模型超时 60s；最多 5 次模型请求、8 次工具分发、8000 个累计已知 token 的软预算。真实演示会消耗模型 token；测试只使用假模型与内存观测器，不读取真实密钥。

当前只做本机演示，包含真实文本流式，不增加生产鉴权、数据库或持久化会话。每次 API 请求是独立运行，消息历史接口的一次请求包含两轮。

## 验证

```sh
./scripts/go vet ./...
./scripts/go test ./...
./scripts/go test -race ./...

cd apps/interfaces/console
npx --no-install biome check src/pages/agent-development
npm run tsc
npm run build
```

格式由 `gofmt` 和现有 Biome 统一处理。构建前停止前端开发服务，构建结束后重新执行 `npm start`。规范见 [中文开发规范](docs/development/README.md)。

管理台 **Agent Demos** 按学习路线提供 13 个二级章节，每章包含文档、相关原文链接及可编辑请求。Demo 接口为 `/api/agent-demos/*`，已有 API 页面和 `/api/agent/*` 继续保留。各章的实际演示范围见 `docs/learning`。
