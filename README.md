# Go Agent 后台与管理台

根据用户指定的三个 Python demo 迁移：普通聊天、携带历史的两轮对话、乘法工具调用循环。管理台点击后请求 Go HTTP 接口，展示实际消息、模型响应、工具结果和 OpenObserve trace ID。

HTTP 接口统一使用 Gin，当前锁定 v1.12.0；标准库 `http.Server` 负责超时和优雅退出。模型客户端继续使用标准库 HTTP/JSON，Agent 循环保持显式实现。

## 启动

Go 工具链为 1.27.2，模块为 `saklay/ai`。`./scripts/go` 使用项目现有工具链，不覆盖系统安装。

准备根目录 `.env`：可从 [.env.example](.env.example) 创建并填写模型及观测鉴权；已有文件不要覆盖。服务通过 `-env-file` 显式加载，环境变量优先，不自动读取 `old` 中的配置。真实凭据已被 Git 忽略。

```sh
# 终端一：启动 Go HTTP 服务，默认 127.0.0.1:8080。
./scripts/go run ./apps/interfaces/http -env-file .env

# 终端二：启动管理台，默认 127.0.0.1:8000。
cd apps/interfaces/console
npm start
```

打开 [Agent 开发](http://127.0.0.1:8000/agent-development)。开发环境预填演示账号 `admin` / `ant.design`，点击登录即可。侧边栏“Agent 开发”提供三个直接运行按钮，分别请求三课接口；可编辑问题并查看回答、真实步骤与 trace ID。其他官方 Demos、个人页、欢迎页和 AI 助手保留。

## 三课与原始文件

| 课程 | 原始 Python demo | Go 流程 | HTTP 接口 |
| --- | --- | --- | --- |
| 普通聊天 | `old/agent-learning/demos/demo_01_chat.py` | system + user → 一次模型请求 → 完整回答 | `POST /api/agent/chat` |
| 消息历史 | `old/agent-learning/demos/demo_02_messages.py` | 第一轮保存实际 assistant → 第二轮回传完整历史 | `POST /api/agent/messages` |
| 工具循环 | `old/agent-learning/demos/demo_03_agent_loop.py` | 模型提出 multiply → Go 校验并执行 → 按调用 ID 回填 → 模型回答 | `POST /api/agent/run` |

默认问题和提示词保留原 demo 内容。工具仅有本地乘法；不引入 Agent 框架，不执行模型生成的代码。`old` 不参与 Go 构建。

## 目录

```text
apps/
  config/            # 显式文件与环境变量配置
  interfaces/
    http/            # 启动入口、显式路由；agent 放接口适配，entity 放请求响应类型，proto 放协议源文件
    console/         # 通用管理台；agent-development 为三课独立页面
    cli/             # 预留，本次不新增 CLI 课程
domain/agent/        # 三个业务流程、执行记录、协议、乘法工具和有限循环
infra/
  httpx/             # 通用严格 JSON 解析、响应编码与传输错误
  deepseek/          # 标准库 HTTP 模型客户端
  openobserve/       # OTLP/HTTP Traces、脱敏、生命周期
docs/
  development/       # 中文 Go 和前端规范
  learning/          # Go 学习教程、后续路线与历史记录
```

学习从 [中文学习指南](docs/learning/README.md) 开始，前三课包含操作、代码阅读、练习和通过标准；后续专题明确标注尚未实现。阅读顺序：`apps/interfaces/http/router.go` → `apps/interfaces/http/agent/chat.go` → `domain/agent/chat.go`、`messages.go`、`run.go` → `infra/deepseek/client.go`。工具协议与循环见 `domain/agent/tools.go`、`loop.go`，执行记录见 `recording.go`。API、错误码和边界见 [HTTP 接口说明](apps/interfaces/http/README.md)；页面运行说明见 [管理台说明](apps/interfaces/console/README.md)。

旧学习资料已迁入 `docs/learning`，对应关系见 [迁移说明](docs/learning/migration.md)。`old` 为废弃的历史归档，学习与运行不再依赖它。

HTTP 请求与响应消息定义在 [agent.proto](apps/interfaces/http/proto/agent.proto)，生成到 [entity/agent.pb.go](apps/interfaces/http/entity/agent.pb.go)，API 方法统一使用 `entity` 中的请求与响应类型。修改协议后执行 `./scripts/generate-proto`，使用 `./scripts/generate-proto --check` 检查生成一致性；已检入生成文件，普通运行无需安装编译器。工具版本和现有 JSON 兼容说明见 [Proto 消息说明](apps/interfaces/http/agent/README.md)。

## 观测与边界

每次运行一个根 span，每次模型请求和工具分发一个子 span。观测异步上报；页面给出 trace ID，但不声称已入库。默认 OpenObserve 本机界面为 `http://localhost:5080/web/`，组织与 stream 由 `.env` 配置确定。

默认总运行超时 120s、单次模型超时 60s；最多 5 次模型请求、8 次工具分发、8000 个累计已知 token 的软预算。真实演示会消耗模型 token；测试只使用假模型与内存观测器，不读取真实密钥。

当前只做本机演示，不增加生产鉴权、流式、数据库或持久化会话。每次点击是独立运行，消息历史课程的一次运行包含两轮。

## 验证

```sh
./scripts/generate-proto --check
./scripts/go vet ./...
./scripts/go test ./...
./scripts/go test -race ./...

cd apps/interfaces/console
npx --no-install biome check src/pages/agent-development
npm run tsc
npm run build
```

格式由 `gofmt` 和现有 Biome 统一处理。构建前停止前端开发服务，构建结束后重新执行 `npm start`。规范见 [中文开发规范](docs/development/README.md)。
