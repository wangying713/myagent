# Go Agent 开发学习

面向已有 Go 基础的学习者，从 HTTP 和消息协议开始，手写一个能够调用受控工具的 Agent，并在 OpenObserve 中查看实际请求、响应和工具结果。

第一阶段提供三课：普通聊天、消息历史、工具调用循环。学习路径见[中文课程入口](docs/learning/README.md)，开发约定见[Go 开发规范](docs/development/README.md)。

## 快速开始

项目工具链为 Go 1.27.2，OpenTelemetry 依赖锁定为 1.47.0。现有 Go 支持自动工具链下载时，根目录执行命令会选择项目版本，不覆盖系统安装。

本机自动下载受到代理和网络影响，已从官方源码构建 Go 1.27.2 并保存在 Git 忽略的 `.tools/` 中。当前机器可把下文命令中的 `go` 换成 `./scripts/go`；这个入口只为当前命令设置对应的 `GOROOT`，不修改系统 Go 或全局配置。其他机器没有本地工具链时，它会调用普通 `go`。

先把 [.env.example](.env.example) 复制为根目录 `.env`，填写 DeepSeek API key 和 OpenObserve 鉴权；已有文件时保留。程序只读取 `-env-file` 指定的文件，环境变量优先。不会自动读取 `old` 的配置。

```sh
go mod download

# 不调用模型，先验证观测发送与入库查询。
go run ./apps/interfaces/learning -lesson observe -env-file .env

# 第一课：一次 HTTP 模型调用。
go run ./apps/interfaces/learning -lesson chat -env-file .env

# 第二课：显式保存并传回消息历史。
go run ./apps/interfaces/learning -lesson messages -env-file .env

# 第三课：模型提出工具调用，程序执行，再回填结果。
go run ./apps/interfaces/learning -lesson agent -env-file .env
```

真实课程会调用模型并消耗 token；离线测试不会调用外部服务。若只体验模型、暂不连接 OpenObserve，明确设置 `TELEMETRY_ENABLED=false`。`observe` 和观测验收模式必须启用观测。

每次课程打印 `trace_id`。默认本机界面为 `http://localhost:5080/web/`，在 Traces 中选择配置的 stream，再查这个 ID。端点、组织与凭据都可以通过配置更换。

## 目录与职责

```text
apps/interfaces/
  config/          # 显式文件与环境配置
  learning/        # 命令入口和三个学习示例
domain/agent/
  entity/          # 消息、工具协议、响应与运行结果
  service/         # 工具注册与有限 Agent 循环
infra/
  deepseek/        # 标准 HTTP 模型客户端
  openobserve/    # trace、脱敏、OTLP 导出与查询
docs/
  development/    # 中文 Go 开发规范和技能
  learning/       # 中文课程、练习与验收
old/               # Python 课程参考，不参与 Go 构建
```

其余空目录是预留骨架，第一阶段没有加入数据库、Kafka、流式、RAG 或持久化会话。

## 验收

本次测试和真实链路结果见[验收记录](docs/learning/verification.md)。

```sh
go vet ./...
go test ./...
go test -race ./...

# 真实课程同时检查业务结果、实际调用数和父子 span 入库。
go run ./apps/interfaces/learning -lesson chat -env-file .env -verify
go run ./apps/interfaces/learning -lesson messages -env-file .env -verify
go run ./apps/interfaces/learning -lesson agent -env-file .env -verify

# 只查询已有记录，不调用模型、不新建 span。
go run ./apps/interfaces/learning -lesson verify -env-file .env -trace-id 实际32位trace_id -expect 4
```

格式检查可运行 `gofmt -l apps/interfaces domain/agent infra/deepseek infra/openobserve`；无输出表示格式一致。配置、观测失败、用量缺失与验收边界见[OpenObserve 说明](docs/learning/openobserve.md)。
