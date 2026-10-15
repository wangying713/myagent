# Agent HTTP API 与 Proto 消息

三个 Agent 接口使用 Proto 定义请求和响应数据，继续通过 Gin 提供 HTTP JSON。接口路径、管理台调用方式、错误码、取消与观测行为保持原有约定。

## 目录与阅读顺序

```text
http/agent/                 # Go 包名 agentapi
  agent.go                  # API 业务依赖
  chat.go                   # 普通聊天入口
  messages.go               # 两轮消息历史入口
  run.go                    # 工具循环入口
  mapping.go                # 请求、领域结果和 HTTP JSON 数据之间的转换
  adapter.go                # 注入编码、错误映射与解码后的字段准备
  errors.go                 # 业务错误映射

http/entity/                # API 方法统一使用的 Go 数据包
  agent.pb.go               # 工具生成的请求、响应、步骤与用量类型
  agent_json.go             # 共用的 HTTP JSON 表现类型

http/proto/
  agent.proto               # 请求、响应、步骤与用量的消息定义
```

从 `http/router.go` 找完整 URL，进入 `http/agent` 对应处理方法，再看 `http/proto/agent.proto` 的输入输出。业务实现仍在 `domain/agent`，不导入生成包。

协议源文件直接放在 `http/proto`，通过 `go_package` 指定生成到 `http/entity`，均按业务分文件。API 方法只引用 `entity` 中的请求和响应类型，消息字段统一由 `.proto` 定义，不再手写一套同名请求响应。本次只生成消息，不生成路由、RPC 服务、前端客户端或接口文档；三个完整路由仍逐条绑定具名方法。公共 `data` / `error` 外层继续由 `infra/httpx` 定义，不与 Agent 消息绑定。修改消息时，同时检查 Proto、HTTP JSON、业务语义和管理台调用的兼容性。

## API 方法写法

三个 API 统一使用 `(ctx, req) → (rsp, err)`：上下文和请求是入参，响应与错误是返回值。例如当前消息历史入口：

```go
func (a *Agent) Messages(ctx context.Context, req *entity.MessagesRequest) (*entity.ExecutionResult, error) {
	result, err := a.service.Messages(ctx, agent.MessagesInput{Questions: req.GetQuestions()})
	return resultMessage(result), err
}
```

`Chat` 和 `Run` 先转换并校验问题的省略与空字符串，再按同样方式返回结果。API 方法不接收 `gin.Context`，不解析 HTTP 正文、不设置超时、不直接写响应。

`router.go` 创建 `agentapi.NewAgent(service)` 和 `agentapi.NewAgentAdapter(timeout)`，再逐条绑定：

```go
router.POST("/api/agent/chat", httpx.Adapt(adapter, agentAPI.Chat))
router.POST("/api/agent/messages", httpx.Adapt(adapter, agentAPI.Messages, agentapi.PrepareMessages))
router.POST("/api/agent/run", httpx.Adapt(adapter, agentAPI.Run))
```

`infra/httpx/adapter.go` 为每个请求创建新的请求对象，严格解码后运行显式注入的字段准备函数，再从 HTTP 请求上下文派生总超时，调用具名 API，最后统一编码返回并释放上下文。泛型约束在编译期检查请求指针实现 Proto 消息接口，不通过反射寻找方法或按课程字符串分发。

`agent/adapter.go` 只注入本业务的 JSON 格式和错误映射。响应和错误可以同时非空：例如预算中止、模型失败时仍返回已完成的步骤。适配器不能看到错误就丢弃响应，API 也不能在已有部分结果时提前返回 `nil, err`。解析失败或没有结果时，API 编码规则保留原有零值数据结构。

## 生成与一致性检查

在项目根目录执行：

```sh
# 修改 agent.proto 后，重新生成 Go 消息。
./scripts/generate-proto

# 只比较生成结果，不修改已检入的生成文件；不一致时返回非零退出码。
./scripts/generate-proto --check
```

工具版本：

| 工具 | 固定版本与来源 |
| --- | --- |
| Go | 项目工具链 1.27.2，通过 `scripts/go` 使用 |
| protoc | 29.3，脚本校验版本 |
| protoc-gen-go 与 Go Protobuf 运行时 | 同为 `go.mod` 锁定的 v1.36.12 |

生成脚本优先使用项目内 `.tools/protoc-29.3/bin/protoc`，否则检查 `PATH` 中的 `protoc`。已有 29.3 版可直接运行。其他机器可从 [Protobuf v29.3 官方发布页](https://github.com/protocolbuffers/protobuf/releases/tag/v29.3) 下载对应操作系统与架构的编译器压缩包，将其中的 `bin`、`include` 解压到 `.tools/protoc-29.3`；不需要覆盖系统安装。只构建、运行已检入的 Go 代码不需要 protoc。

脚本使用项目 Go 工具链，将生成器构建到被忽略的 `.tools/protobuf`，不需要全局安装插件；第一次运行如果 Go 模块未缓存，需要下载锁定依赖。`go_package` 指向 `saklay/ai/apps/interfaces/http/entity`，生成器使用模块输出模式，把 `proto/agent.proto` 生成到 `entity/agent.pb.go`。生成文件由临时目录中的完整结果覆盖，检查模式不改源文件。提交时同时包含 `.proto` 和 `.pb.go`，并运行生成一致性检查。

生成文件开头可能将编译器显示为 `v5.29.3`，这是该工具的版本表示；命令行检查仍是 `libprotoc 29.3`。生成器自动附带的英文机械注释保留，业务消息注释来自中文 `.proto`。

## HTTP JSON 兼容边界

请求先由 `httpx.DecodeProtoJSON` 按消息描述符校验顶层字段名，再由官方 `protojson` 解码。保留 32 KiB 正文限制、内容类型检查，以及对顶层未知字段、重复字段、大小写与驼峰别名、显式 null、多段 JSON 的拒绝；不使用 Gin 默认绑定替代这些检查。字段名从 `.proto` 派生，不在处理器里维护第二份允许字段列表。

- `question` 使用 `optional string`，区分省略与显式空字符串；省略使用默认问题，空字符串或空白问题无效。
- `questions` 使用 `repeated string`。这类字段本身不提供存在性，因此通用解码器另行返回实际出现的顶层字段。路由为消息接口显式绑定 `agentapi.PrepareMessages`，在成功解码后将显式空数组保留为非 nil 的空切片，省略则保持 nil。领域方法仍按原规则处理，不从上下文读取隐藏参数。
- 用量使用 `optional int64`，区分未知、已知为零和其他数值。

**响应使用现有项目的 HTTP JSON 格式，不是标准 ProtoJSON 响应。** 官方 ProtoJSON 默认使用驼峰字段名、将 64 位整数输出为字符串，并有自己的零值和 null 规则。管理台当前使用下划线字段名、数字整数、未知用量的 null、可省略的步骤属性，以及 `steps: null` 与 `steps: []` 的区别。直接替换编码器会改变这些契约。

因此 `agent/mapping.go` 的 `resultMessage` 先将领域结果转换为 `entity` 中生成的响应消息，`resultJSON` 再转换为 `entity/agent_json.go` 定义的共用 JSON 数据，最终交给 `httpx` 统一编码和写出。这些手写结构体只负责现有 JSON 表现；转换函数留在 `http/agent`，不直接对生成类型调用 `encoding/json`，也不修改生成文件的标签。新增响应字段时，要同时补充共用数据定义、领域映射、JSON 映射和响应契约测试。后续接口的数据放置规则见 [HTTP 共用数据规范](../entity/README.md)。

当前管理台仍使用手写 TypeScript HTTP 类型，不能直接用标准 ProtoJSON 响应解码器替代现有请求封装。Go `int64` 在服务端完整保留；浏览器中的数字仍受 JavaScript 安全整数范围限制，与改造前相同。

## 修改消息的约定

1. 先修改 `.proto`，字段编号在发布后不重用；删除字段时保留其编号与名称。
2. 修改生成文件必须通过生成命令完成，不复制同步对象、不手写生成器内部结构。
3. 接口类型停留在 HTTP 适配层，通过明确转换调用领域服务。
4. Proto 类型不替代业务校验；问题数量、长度、工具权限和预算继续由现有业务处理。
5. 运行生成检查、格式检查、`go vet`、普通测试与竞态测试；全程使用假模型和内存观测器。

原理参考：[Go 消息生成说明](https://protobuf.dev/reference/go/go-generated/)、[ProtoJSON 格式](https://protobuf.dev/programming-guides/json/)、[字段存在性](https://protobuf.dev/programming-guides/field_presence/)。这些官方页面是来源资料，项目内使用说明以本中文文档为准。
