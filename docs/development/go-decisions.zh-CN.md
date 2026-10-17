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

## 注释位置与简洁性

### 术语与判断顺序

这里按注释的位置区分两种写法，均使用 `//`：

- **独立注释**：独占一行或多行，紧邻对应声明的上方。
- **行尾注释**：与被说明的代码写在同一行，放在代码之后。

“注释内容只有一行”不代表应该使用行尾注释。手写 Go 代码（包括测试）先按声明类型和是否成组确定位置，再按信息量决定是否需要独立说明。

| 被说明的对象 | 注释位置 | 判断要点 |
| --- | --- | --- |
| 包、结构体及其他类型的用途 | 声明上方的独立注释 | 类型用途与字段含义分别说明 |
| 函数、方法，包括接口方法 | 声明上方的独立注释 | 即使说明很短，也不放在方法签名行尾 |
| 独立的一条 `const` 或 `var` 声明 | 声明上方的独立注释 | 即使说明很短，也独占一行；仅含一项的括号声明同样处理 |
| 成组 `const`、`var` 中的各项 | 简短说明放行尾，较多说明放该项上方 | 同组各项可按各自说明的信息量选择位置 |
| 结构体字段，包括匿名结构体和测试字段 | 简短说明放行尾，较多说明放字段上方 | 只有一个字段时也遵循字段规则 |

分组依据职责与语义；不要为了使用行尾注释，把独立声明硬凑成一组。结构体类型的用途说明放在上方，不随字段注释移动到类型声明行尾。

### 说明内容与长度

- 简短说明通常只解释一个含义，适合与代码同行阅读。内容较多、包含多项条件或约束，或与代码挤在一起影响阅读时，改用独立注释。
- 独立注释可按完整意思写成一行，也可按约束和段落分成多行；不强行合并成长行，不设固定字符数或代码行宽阈值。
- 包、类型、函数、方法及独立声明的文档注释通常以对应标识符名称开头；行尾注释直接说明含义，省略重复的标识符及“表示”“标识”“是”等引导词。对齐交给 `gofmt`。
- 移动或精简注释时保留原有含义，尤其不能删掉单位、nil 和零值语义、取消、并发、资源释放及部分结果等约束。
- 编译指令（如 `//go:embed`）和工具读取的注释保持所需位置与格式；生成文件不手改，`old` 历史归档不纳入整改。

### 示例

独立声明，即使说明很短也使用上方的独立注释：

```go
// MaxQuestionChars 是单个问题允许的最大 Unicode 字符数。
const MaxQuestionChars = 4096
```

成组声明中，各项的简短说明放在行尾：

```go
const (
	RoleSystem = "system" // 系统提示消息。
	RoleUser   = "user"   // 用户输入消息。
)
```

接口方法与普通函数、方法一样，说明独立放在声明上方：

```go
// ToolObservation 保存一次工具调用的观测句柄。
type ToolObservation interface {
	// End 记录结果和错误并结束观测，每次 StartTool 对应一次调用。
	End(ToolResult, error)
}
```

结构体的用途说明放在上方；字段说明较多时也独立放在字段上方：

```go
// Message 保存对话消息及可选的原始 JSON。
type Message struct {
	Role string // 消息角色。
	// Raw 保存完整的 assistant JSON，非空时优先用于编码。
	// 原始内容用于保留扩展字段和 null，不能用重组后的字段替代。
	Raw json.RawMessage
}
```

这是项目自己的注释位置约定；函数和类型等声明仍遵循 [注释和文档规范](google-go-decisions.zh-CN.md#二注释和文档)。

## 结构体字段说明

- 手写 Go 结构体的字段统一补充中文注释，包括导出数据、内部辅助类型、匿名结构体和测试数据；生成文件与 `old` 历史归档沿用原有内容。
- 字段注释说明具体含义，按需写明时间与大小单位、计数范围、nil 或零值语义、并发保护及关联关系；不只重复字段名称或 Go 类型。
- 简短说明放在字段行尾；内容较多或包含多项约束的说明放在字段上方，可写成一行或多行 `//`。具名结构体的用途说明放在类型声明上方，空结构体不添加无意义的字段说明。
- 同一行声明的多个字段可以共用能明确对应各字段的说明；含义不同、需要分别解释时拆成独立字段声明，保持字段顺序与类型不变。

```go
// ExecutionResult 保存本次业务执行的结果。
type ExecutionResult struct {
	Completed bool  // 本次业务是否成功完成。
	ElapsedMS int64 // 本次运行耗时，单位为毫秒。
}
```

这是为项目学习用途补充的说明约定，不是 Google Go 对所有字段的通用强制要求。

## 函数与方法参数数量

- 手写 Go 函数和方法的参数**最多 4 个，不得达到 5 个**。接口方法、函数类型、匿名函数及测试辅助函数同样遵循；生成文件和 `old` 历史归档不手改。
- 按实际参数个数计算：`ctx` 算一个，`question, prompt string` 算两个，可变参数 `args ...string` 算一个；方法接收者、泛型类型参数和返回值不计入。
- 参数达到 5 个时，先看是否把多件事放进了同一个函数；职责合理时，把相关输入合并为具名结构体。调用处写明字段名，让读者直接看出每个值的用途。
- `context.Context` 仍单独作为首参；同一次操作要用的模型、工具等依赖也可以放进输入结构体，通过具名字段明确传入。不把无关数据塞进大结构体，不通过全局变量、`map[string]any` 或可变参数绕过数量限制。
- 实现标准库、第三方接口或使用其规定的回调签名时，按外部要求保留签名；只有确实需要匹配外部约定时才适用此例外。

例如工具循环把模型、工具、问题、提示词和调用限制放在一起，调用时只传 `ctx` 和这份输入：

```go
// LoopInput 放这次工具循环要用的模型、工具、问题和调用限制。
type LoopInput struct {
	Model    ChatClient // 这次使用的模型，不能为 nil。
	Tools    Tools      // 模型可以调用的工具，不能为 nil。
	Question string     // 用户要问的问题。
	Prompt   string     // 系统提示词。
	Limits   Limits     // 模型、工具和 token 的使用上限。
}

// Run 根据输入调用模型和工具，返回最终回答。
func Run(ctx context.Context, req LoopInput) (string, error)
```

这是本项目为方便阅读设定的数量限制，不是 Google Go 的通用要求；分组思路见 [函数参数与可选配置](google-go-best-practices.zh-CN.md#六函数参数与可选配置)。

## 请求参数命名

- 函数、方法只有一个业务请求或执行输入对象时，参数统一命名为 `req`；例如 `Run(ctx context.Context, req LoopInput)`、`Chat(ctx context.Context, req Request)`。
- `ctx` 仍单独作为首参；另有模型、工具等依赖参数时，唯一的业务请求参数仍叫 `req`。领域方法、接口声明、模型与工具实现、相关测试替身和回调保持一致。
- 这项约定针对请求或执行输入对象。配置仍可叫 `cfg`，响应叫 `rsp`，依赖及普通辅助值按含义命名，例如 `tools`、`question`、`timeout`、`err`；只有 `ctx` 的方法仍使用 `ctx`。同一函数里的 HTTP 请求和业务请求需要区分时，HTTP 请求使用 `httpReq`。
- 只统一参数变量名，不因此修改 `ChatInput`、`LoopInput` 等已有类型名，不把多个含义不同的输入硬合并。生成文件和 `old` 历史归档不手改。

这是本项目的命名偏好，目的是让同类方法使用相同写法，不是 Google Go 对所有单参数函数的要求。

## 代码块与语句换行

- 手写 Go 代码（包括测试）的非空代码块统一展开：左花括号后换行，块内每条语句单独一行，右花括号另起一行；即使只有一条语句，也不压成单行。
- 函数、方法、匿名函数以及 `if`、`for`、`switch`、`select` 等代码块均遵循此规则，包括 `defer`、`go`、排序回调和测试回调中的匿名函数。`case`、`default` 后的语句也另起一行，不用分号把多条语句挤在一行。
- `if`、`for`、`switch` 头部所需的分号保留惯用写法，例如 `if err := validate(input); err != nil {`；空代码块可以保留 `{}`。生成文件与 `old` 历史归档不纳入整改。
- 换行只调整书写方式，保留原有求值时机与执行顺序。例如下面的闭包需要在函数返回时读取最终的 `err`，不能为了简写替换成 `defer execution.end(err)`。

```go
defer func() {
	execution.end(err)
}()

go func() {
	handler.ServeHTTP(w, req)
	close(done)
}()
```

这是项目自己的可读性约定，缩进和其他格式交给 `gofmt`。

## 复合字面量换行

- 手写 Go 代码（包括测试）中的切片、数组、映射和结构体复合字面量，只要包含多个元素或字段，就采用多行写法，每个元素或字段独占一行；不因内容较短而挤在同一行。
- 左花括号后换行，最后一项保留尾逗号，右花括号另起一行；缩进和字段对齐交给 `gofmt`。
- 嵌套字面量逐层遵循同一规则；内层展开后，外层即使只有一个元素或字段，也展开书写，保持层次清晰。
- 空字面量和仅含一个简单元素或字段的字面量可以保留单行。字符串中的 JSON 等协议样本不按 Go 字面量拆分；生成文件（如 `.pb.go`）沿用生成器输出，`old` 历史归档不纳入整改。

```go
questions := []string{
	FirstQuestion,
	SecondQuestion,
}

message := Message{
	Role:    RoleUser,
	Content: question,
}
```

这是项目自己的可读性约定，不是 Google Go 的固定行宽要求。

## 具名常量与字面量

- 业务操作标识、消息角色、结束原因、步骤类型、工具名称等固定取值，应在所属业务或协议包定义具名常量；同一含义在多处引用时必须复用定义，避免散落 `"messages"`、`"stop"` 等魔法字符串。
- 业务阈值使用表达含义的具名常量；可配置的默认值集中由所属包提供，例如问题长度用 `agent.MaxQuestionChars`，预算默认值复用 `agent.DefaultLimits()` 的字段，校验、配置和提示保持一致。
- 常量按职责就近分组，名称使用 Go 驼峰；跨包协议需要的定义可导出，局部实现细节保持私有。不为了收集常量新建全局 `common/constants` 包，也不因字符串取值相同就混用不同概念，如业务操作、服务名称与观测操作。
- 个别字面量可以保留：空串、直观的 0/1 和索引、局部结构的固定数量、只出现一次且语义明确的文案或局部值，以及 JSON 标签、JSON Schema、显式路由等协议声明。是否提取以含义、复用和维护成本判断，不要求消除所有字面量。
- 测试中的独立协议样本、边界输入和预期值可保留字面量，以验证线上取值不变；不要把所有预期都改成生产常量，导致常量改错时测试也同步放行。生成文件继续交给生成器处理。

例如 `s.newResult(OperationMessages)` 表达消息历史操作；其他包引用该标识时使用 `agent.OperationMessages`。这项约定是项目工程选择，不能据此把每个字符串、每个数字都视为错误。

## HTTP API 方法签名

HTTP 顶层只保留 `main.go` 一个 Go 文件，负责参数、配置、信号与最终错误输出。服务实现和测试放在 `http/app`（包名 `app`），启动入口为 `Run(ctx, cfg)`，由 `app/run.go` 组装依赖并管理资源。Agent 的处理方法、请求准备和各接口的 Proto 响应组装位于 `http/app/impl_agent.go`，HTTP JSON 适配与错误映射位于 `http/app/handler.go`；后续业务使用 `app/impl_xxx.go`。API 的协议源文件与生成的请求响应类型统一放在 `http/proto`；`http/proto/agent.proto` 定义消息，通过生成脚本输出同目录的 `agent.pb.go`。手写 HTTP 公共外层与 JSON 表现类型放在 `http/entity`。路由与中间件位于 `app/router.go`，服务生命周期位于 `app/server.go`，通用传输能力留在 `infra/httpx`。

后续新增接口继续遵循这一分工：协议源文件与生成的请求响应类型直接放在 `http/proto`，手写 HTTP 公共外层与 JSON 表现类型放在 `entity`，均按业务分文件；接口按需复用步骤、用量等共用消息，响应由各接口独立定义。`entity` 只负责字段与序列化约定，不依赖业务处理包、领域服务、Gin 或基础设施；请求准备和 Proto 响应组装放在对应业务的 `app/impl_xxx.go`；HTTP JSON 适配和错误映射位于 `app/handler.go`。领域层与 `infra/httpx` 不依赖应用的 `entity` 或 `proto` 包。具体见 [HTTP 共用数据规范](../../apps/interfaces/http/entity/README.md)。

业务 API 统一使用 `(ctx, req) → (rsp, err)`：`context.Context` 和 `proto` 中生成的请求消息指针作为入参，`proto` 中生成的响应消息指针与 `error` 作为返回值。

```go
func (a *ImplAgent) Chat(ctx context.Context, req *proto.ChatReq) (*proto.ChatRsp, error)
```

- API 方法负责参数转换、调用领域服务和返回结果，不接收 `gin.Context`，不解析 HTTP 或直接写出响应。
- `app/router.go` 逐条声明完整路径，通过 `handle(implAgent.Chat)` 这类单参数调用绑定具体方法；不通过反射扫描方法或解析注释生成路由。
- 解码、请求准备、超时、取消及响应处理由 `http/app/handler.go` 集中完成。路由初始化通过 `requestTimeout(timeout)` 中间件显式设置当前路由的超时，计时仍从解码及准备完成后开始；配置不放入全局变量。`infra/httpx` 只保留独立的严格解析、传输错误和 JSON 写出，不依赖业务或 `entity`。
- 接口方法显式组装并返回自己的 Proto 响应，允许重复字段赋值，不封装统一业务返回函数。输入拒绝返回 nil 数据，其他失败保留已经完成的结果，错误原样返回。
- 响应和错误可以同时非空，已有的步骤和用量必须保留，不能因错误而统一丢弃响应。
- 领域层使用自己的输入输出，不依赖接口生成类型；解码后需要保留的字段存在性通过显式准备函数转换到请求对象，不藏在上下文中。

请求消息统一使用 `Req` 后缀，响应消息使用 `Rsp` 后缀。三个 Agent 接口分别返回 Proto 定义的 `ChatRsp`、`MessagesRsp`、`RunRsp`；业务方法不返回 HTTP 外层结构。

公共响应码常量定义在 `entity/code.go`，接口适配使用 `entity.CodeInvalidInput` 等具名常量。领域错误判断、HTTP 状态及具体提示保留在 `app/handler.go`，`entity` 不依赖领域或基础设施。

HTTP 统一外层手写于 `entity/response.go`：`Rsp[T]` 包含 `code`、`data`、`traceID`、`msg`。成功为 `code: "ok"`、`msg: "成功"`；失败沿用稳定的字符串错误码及中文说明，同时保留原有 HTTP 状态。解码失败、输入拒绝及其他尚无业务结果的响应使用 `data: null`，部分结果与错误同时返回时保留数据。HTTP handler 负责包装，`httpx.Write` 只编码并写出；编码失败不能先写出成功响应。

HTTP 入口在解码前使用官方 `otelgin` 中间件建立请求 span，显式注入私有 TracerProvider、noop MeterProvider 和空传播器，保持服务内链路且不依赖全局观测配置。领域运行作为其子 span，模型和工具仍是领域运行的子 span。通过 otelgin 的名称格式器只展示 URI 路由模板，例如 `/api/agent/chat`；未匹配路由统一为 `[unmatched]`。HTTP 方法保留在属性中，屏蔽默认属性中的原始路径、User-Agent 与客户端地址。启用正文采集时，在根 span 记录实际读写的请求体与最终 HTTP 响应，预览直接显示正文 JSON，不添加角色、消息或 body 包装；方法、路由模板和响应状态保留在 span 属性中，正文复用脱敏、限长及 OpenObserve 原生预览适配；不采集请求头或查询字符串，不预读请求。公共响应处理继续将 4xx 和 5xx 标记为失败。外层 `traceID` 来自实际请求链路，业务 Proto 响应不再重复携带；关闭观测或服务退出闸门尚未进入路由时为空，不伪造 ID。

实现与使用说明见 [HTTP API](../../apps/interfaces/http/README.md) 和 [通用 HTTP 能力](../../infra/httpx/README.md)。

领域运行 `agent.run` 的观测输入通过使用方定义的 `Observer.StartRun` 显式传入已补齐默认值并校验的问题；领域层不接触 OpenTelemetry 属性或 JSON 展示协议。`infra/openobserve` 组装输入问题与输出的 `completed/answer` 预览，复用正文采集开关、脱敏、限长及原生预览适配，失败保留已有回答，不记录底层错误正文。

## 第三方能力选择

通用框架能力优先复用当前依赖，新增依赖须能减少维护成本并保留现有契约。HTTP 请求链路使用 `otelgin`，Proto 解码使用官方 `protojson`；严格字段检查和 HTTP JSON 表现继续由入口适配。

配置文件已使用 `godotenv.Read`。当前环境解析保留显式实现：显式空值优先于文件与默认值，布尔值仅接受规范化后的 true/false，整数沿用本机 int 范围。`caarlos0/env/v11` 的默认值与类型转换语义不同，当前规模下接入还需自定义解析及错误翻译，暂不增加该依赖。服务生命周期继续使用标准库，模型客户端保留原始 assistant 消息、取消与无重试语义。
