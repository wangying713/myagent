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

HTTP 入口下按业务划分包。Agent 的处理方法、请求响应转换与错误映射放在 `http/agent`（Go 包名 `agentapi`）。API 使用的请求、响应及共用数据类型统一放在 `http/entity`；`http/proto/agent.proto` 定义消息，通过生成脚本输出 `http/entity/agent.pb.go`。路由和服务生命周期留在 HTTP 入口，通用传输能力留在 `infra/httpx`。

后续新增接口继续遵循这一分工：协议源文件直接放在 `http/proto`，生成的请求响应类型与手写 HTTP JSON 表现类型放在 `entity`，均按业务分文件；多个接口复用同一数据定义。`entity` 只负责字段与序列化约定，不依赖业务处理包、领域服务、Gin 或基础设施；转换函数放在业务适配包的 `mapping.go`，字段准备和错误映射也留在业务适配包。领域层与 `infra/httpx` 不依赖 `entity`。具体见 [HTTP 共用数据规范](../../apps/interfaces/http/entity/README.md)。

业务 API 统一使用 `(ctx, req) → (rsp, err)`：`context.Context` 和 `entity` 中生成的请求消息指针作为入参，`entity` 中生成的响应消息指针与 `error` 作为返回值。

```go
func (a *Agent) Chat(ctx context.Context, req *entity.ChatRequest) (*entity.ExecutionResult, error)
```

- API 方法负责参数转换、调用领域服务和返回结果，不接收 `gin.Context`，不解析 HTTP 或直接写出响应。
- `router.go` 逐条声明完整路径，通过 `httpx.Adapt` 绑定具体方法；不通过反射扫描方法或解析注释生成路由。
- 解析、超时、取消和写出由通用适配器负责；具体 JSON 数据格式与业务错误映射由 API 注入，通用包不依赖业务。
- 响应和错误可以同时非空，已有的步骤和用量必须保留，不能因错误而统一丢弃响应。
- 领域层使用自己的输入输出，不依赖接口生成类型；解码后需要保留的字段存在性通过显式准备函数转换到请求对象，不藏在上下文中。

实现与使用说明见 [HTTP API](../../apps/interfaces/http/agent/README.md) 和 [通用适配器](../../infra/httpx/README.md)。
