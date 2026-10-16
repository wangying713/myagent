# 本项目 Go 具体约定

本文件记录项目自己的工程约定，是对 Google Go 规范的补充，不代表 Google 官方规则。核心原则、具体语言规则和辅助建议的层级仍按 [规范阅读顺序](README.md#go-规范阅读顺序) 执行。

## 导入分组与顺序

手写 Go 文件的 `import` 按以下顺序分为三组，组间空一行：

1. **标准库（系统）**：Go 标准库，例如 `context`、`net/http`、`time`。
2. **本项目包（本地）**：当前模块 `saklay/ai` 及其子包，例如 `saklay/ai/apps/config`、`saklay/ai/domain/agent`。
3. **外部依赖（第三方）**：当前模块以外的依赖，例如 `github.com/gin-gonic/gin`、`google.golang.org/protobuf/proto`。Go 官方维护但不属于标准库的模块，例如 `golang.org/x/sync`，也属于这一组。

```go
import (
	"context"
	"net/http"
	"time"

	"saklay/ai/apps/config"
	"saklay/ai/domain/agent"

	"github.com/gin-gonic/gin"
)
```

- 没有导入项的组直接省略，不保留空组；只有一个导入项时可使用单行 `import`。
- 组内按导入路径排序，交给 `gofmt` 处理；保留确有必要的导入别名，不根据别名或业务功能再拆组。
- `gofmt` 会保留组间空行，不会自动按上述类别重新分组，添加导入时需要确认分组和顺序。
- 工具生成的文件沿用生成器输出，不手工修改 `.pb.go` 等生成文件的导入分组。

“标准库 → 本项目包 → 外部依赖”是本项目统一选择的顺序。Google 规范中的一般导入要求见 [Google Go 具体规范：导入](google-go-decisions.zh-CN.md#三导入)。

## HTTP API 方法签名

HTTP 顶层只保留 `main.go` 一个 Go 文件，负责参数、配置、信号与最终错误输出。服务实现和测试放在 `http/app`（包名 `app`），对外只暴露 `Run(ctx, cfg)`，由 `app/run.go` 组装依赖并管理资源。Agent 的处理方法、请求准备和各接口的 Proto 响应组装位于 `http/app/impl_agent.go`，HTTP JSON 适配与错误映射位于 `http/app/handler.go`；后续业务使用 `app/impl_xxx.go`。API 的协议源文件与生成的请求响应类型统一放在 `http/proto`；`http/proto/agent.proto` 定义消息，通过生成脚本输出同目录的 `agent.pb.go`。手写 HTTP 公共外层与 JSON 表现类型放在 `http/entity`。路由与中间件位于 `app/router.go`，服务生命周期位于 `app/server.go`，通用传输能力留在 `infra/httpx`。

后续新增接口继续遵循这一分工：协议源文件与生成的请求响应类型直接放在 `http/proto`，手写 HTTP 公共外层与 JSON 表现类型放在 `entity`，均按业务分文件；接口按需复用步骤、用量等共用消息，响应由各接口独立定义。`entity` 只负责字段与序列化约定，不依赖业务处理包、领域服务、Gin 或基础设施；请求准备和 Proto 响应组装放在对应业务的 `app/impl_xxx.go`；HTTP JSON 适配和错误映射位于 `app/handler.go`。领域层与 `infra/httpx` 不依赖应用的 `entity` 或 `proto` 包。具体见 [HTTP 共用数据规范](../../apps/interfaces/http/entity/README.md)。

业务 API 统一使用 `(ctx, req) → (rsp, err)`：`context.Context` 和 `proto` 中生成的请求消息指针作为入参，`proto` 中生成的响应消息指针与 `error` 作为返回值。

```go
func (a *agentAPI) Chat(ctx context.Context, req *proto.ChatReq) (*proto.ChatRsp, error)
```

- API 方法负责参数转换、调用领域服务和返回结果，不接收 `gin.Context`，不解析 HTTP 或直接写出响应。
- `app/router.go` 逐条声明完整路径，通过 `httpx.Adapt` 绑定具体方法；不通过反射扫描方法或解析注释生成路由。
- 解析、超时、取消和写出由通用适配器负责；具体 JSON 数据格式、业务错误映射和响应写出由 `http/app/handler.go` 注入，通用包不依赖业务或 `entity`。
- 接口方法显式组装并返回自己的 Proto 响应，允许重复字段赋值，不封装统一业务返回函数。输入拒绝返回 nil 数据，其他失败保留已经完成的结果，错误原样返回。
- 响应和错误可以同时非空，已有的步骤和用量必须保留，不能因错误而统一丢弃响应。
- 领域层使用自己的输入输出，不依赖接口生成类型；解码后需要保留的字段存在性通过显式准备函数转换到请求对象，不藏在上下文中。

请求消息统一使用 `Req` 后缀，响应消息使用 `Rsp` 后缀。三个 Agent 接口分别返回 Proto 定义的 `ChatRsp`、`MessagesRsp`、`RunRsp`；业务方法不返回 HTTP 外层结构。

公共响应码常量定义在 `entity/code.go`，接口适配使用 `entity.CodeInvalidInput` 等具名常量。领域错误判断、HTTP 状态及具体提示保留在 `app/handler.go`，`entity` 不依赖领域或基础设施。

HTTP 统一外层手写于 `entity/response.go`：`Rsp[T]` 包含 `code`、`data`、`traceID`、`msg`。成功为 `code: "ok"`、`msg: "成功"`；失败沿用稳定的字符串错误码及中文说明，同时保留原有 HTTP 状态。解码失败、输入拒绝及其他尚无业务结果的响应使用 `data: null`，部分结果与错误同时返回时保留数据。HTTP handler 负责包装，`httpx.Write` 只编码并写出；编码失败不能先写出成功响应。

HTTP 入口在解码前使用官方 `otelgin` 中间件建立请求 span，显式注入私有 TracerProvider、noop MeterProvider 和空传播器，保持服务内链路且不依赖全局观测配置。领域运行作为其子 span，模型和工具仍是领域运行的子 span。固定使用 `http.request` 名称，保留路由模板，屏蔽默认属性中的原始路径、User-Agent 与客户端地址。公共响应处理继续将 4xx 和 5xx 标记为失败。外层 `traceID` 来自实际请求链路，业务 Proto 响应不再重复携带；关闭观测或服务退出闸门尚未进入路由时为空，不伪造 ID。

实现与使用说明见 [HTTP API](../../apps/interfaces/http/README.md) 和 [通用适配器](../../infra/httpx/README.md)。

## 第三方能力选择

通用框架能力优先复用当前依赖，新增依赖须能减少维护成本并保留现有契约。HTTP 请求链路使用 `otelgin`，Proto 解码使用官方 `protojson`；严格字段检查和 HTTP JSON 表现继续由入口适配。

配置文件已使用 `godotenv.Read`。当前环境解析保留显式实现：显式空值优先于文件与默认值，布尔值仅接受规范化后的 true/false，整数沿用本机 int 范围。`caarlos0/env/v11` 的默认值与类型转换语义不同，当前规模下接入还需自定义解析及错误翻译，暂不增加该依赖。服务生命周期继续使用标准库，模型客户端保留原始 assistant 消息、取消与无重试语义。
