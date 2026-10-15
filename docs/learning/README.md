# Go Agent 学习指南

从一次模型请求开始，逐步理解历史消息、受控工具和循环停止。已有 Go 基础即可；示例通过管理台调用 Go HTTP 接口，全部学习步骤在当前项目完成。

## 先运行，再读代码

1. 在项目根目录准备 `.env`，字段见 [配置示例](../../.env.example)。已有配置不要覆盖；密钥与观测鉴权只保存在本机。
2. 在两个终端分别启动后端与管理台：

```sh
# 终端一，在项目根目录执行。
./scripts/go run ./apps/interfaces/http -env-file .env
```

```sh
# 终端二，在项目根目录执行。
cd apps/interfaces/console
npm start
```

3. 打开 [Agent 开发](http://127.0.0.1:8000/agent-development)。演示账号为 `admin` / `ant.design`，开发环境已预填。点击侧边栏“Agent 开发”，页面三个运行按钮分别调用普通聊天、消息历史和工具循环接口，可先编辑默认问题。
4. 展开执行步骤，对照实际请求、响应和工具结果；复制 trace ID，到 OpenObserve 检查入库与父子关系。

服务显式加载 `-env-file`，环境变量优先，不自动读取旧配置。健康检查 `GET /api/demos/health` 只验证 Go 服务存活，不代表模型与观测可用。真实点击会消耗模型 token。

## 课程状态与顺序

| 课程 | 内容 | 当前状态 |
| --- | --- | --- |
| [01 普通聊天](01-chat.md) | 请求、响应、结束原因、用量 | 已实现，可运行 |
| [02 消息历史](02-messages.md) | 保存实际 assistant、第二轮回传历史 | 已实现，可运行 |
| [03 工具与 Agent 循环](03-agent.md) | 工具描述、参数校验、调用 ID、结果回填 | 已实现，可运行 |
| [04 结构化输出](04-json.md) | JSON 语法、结构和业务校验 | 后续课程，尚未实现 |
| [05 流式与思考模式](05-stream-thinking.md) | 增量重组、取消、模式比较 | 后续课程，尚未实现 |
| [06 失败、预算与权限](06-reliability.md) | 错误、取消、软预算、受控执行 | 当前三课已实现相应边界；重试为后续方向 |
| [07 检索、记忆与状态](07-retrieval-memory.md) | 最小 RAG、来源、会话持久化 | 后续课程，尚未实现 |
| [08 评估与交付](08-evaluation.md) | 工具证据、固定用例、综合项目 | 当前有契约测试；自动质量评估与综合项目尚未实现 |

按阶段学习：[初级](levels/beginner.md) → [中级路线](levels/intermediate.md) → [高级路线](levels/advanced.md)。专题与检查：[OpenObserve](openobserve.md)、[验收说明](verification.md)、[代码与课程评审](review.md)。Go 和前端规范统一见 [开发规范](../development/README.md)。

## 对照代码

| 位置 | 学习重点 |
| --- | --- |
| [三课执行器](../../apps/demos/runner.go) | 默认题目、提示词、消息历史、步骤记录、每次运行独立状态 |
| [协议](../../domain/agent/protocol.go) | 类型明确的消息、请求、响应与用量，保留完整 assistant |
| [工具](../../domain/agent/tools.go) | 乘法工具描述、严格参数校验、稳定错误码 |
| [循环](../../domain/agent/loop.go) | 模型 → 工具 → 回填 → 模型，预算与取消 |
| [模型客户端](../../infra/deepseek/client.go) | 标准库 HTTP/JSON、请求超时、响应验证 |
| [观测](../../infra/openobserve/telemetry.go) | 私有 provider、脱敏副本、OTLP/HTTP、生命周期 |
| [HTTP 适配](../../apps/interfaces/http/api/handler.go) | Gin 路由组、严格输入边界、总超时、HTTP 状态与部分结果 |
| [启动入口](../../apps/interfaces/http/main.go) | 配置和依赖组装、信号取消、服务退出 |
| [管理台页面](../../apps/interfaces/console/src/pages/agent-development/index.tsx) | 发请求、取消、展示回答和真实步骤 |

模型 HTTP 客户端在服务启动时创建并复用；历史和统计在每次运行中创建。观测 provider 也属于整个服务，不能在一次请求结束时关闭。`context.Context` 从 HTTP 请求传入循环、模型和工具；请求结束后通过返回值和错误报告实际状态。

后台接口使用 Gin；阅读 HTTP 代码时先看 `api.New` 的路由组，再进入课程处理函数。Gin 上下文只负责入口适配，业务继续使用标准 `context.Context`，模型客户端仍直接发送标准库 HTTP 请求。具体约定见 [HTTP 接口说明](../../apps/interfaces/http/README.md)。

## 迁移范围

本目录已接替旧 Python 学习资料；对应关系与未实现范围见 [迁移说明](migration.md)。学习和运行无需进入 `old`。旧 Python 验收与评审保存在 [历史索引](history/README.md)，不作为 Go 版本证据。

当前只提供三课 HTTP 演示，不提供学习 CLI、流式接口、数据库或持久化会话。每次点击是独立运行，第二课的一次点击执行两轮。
