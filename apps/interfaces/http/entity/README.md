# HTTP 手写共用数据定义

本目录只保存手写的 HTTP 公共外层与 JSON 表现类型。协议源文件和生成的请求响应类型放在 `http/proto`，`.proto` 与 `.pb.go` 同目录。业务处理方法、请求准备与 Proto 响应组装放在 `http/app` 下对应业务的 `impl_xxx.go`，HTTP JSON 适配和错误映射位于 `app/handler.go`，通用解析、超时和响应编码由 `infra/httpx` 提供。

```text
http/proto/
  agent.proto               # 协议源文件
  agent.pb.go               # 自动生成的请求、响应、步骤和用量

http/entity/
  agent_json.go             # 手写的 HTTP JSON 结果、步骤和用量
  response.go               # HTTP 统一外层 Rsp[T]：code/data/traceID/msg
  code.go                   # HTTP 公开响应码常量
```

后续新增接口遵循以下规则：

- 请求消息使用 `Req` 后缀，响应消息使用 `Rsp` 后缀，例如 `proto.ChatReq` 和 `proto.ChatRsp`；各接口的响应独立定义。API 方法直接返回生成的业务响应；HTTP handler 使用 `entity.Rsp[T]` 包装，trace ID 只放外层。
- Proto 源文件直接放在 `http/proto` 下，按业务分文件，`go_package` 指向同目录的 Go 包；修改后通过项目生成脚本更新 `.pb.go`，并运行生成一致性检查，不手改生成文件。
- 手写 JSON 表现类型按业务分文件，并使用清楚的名称，例如 `AgentExecutionResult`；它们用于保留当前 HTTP JSON 契约，多个接口复用同一类型。不在 `entity` 中手写重复的 Proto 请求响应定义，也不通过类型别名转出生成类型。
- 公共响应码集中在 `code.go`，使用 `entity.CodeInvalidInput` 等常量；领域错误判断、HTTP 状态和具体提示保留在 `app/handler.go`。`infra/httpx` 独立定义传输错误，不反向导入本目录。
- 数据定义只承载接口字段与序列化约定；请求准备与响应组装放在对应业务的 `app/impl_xxx.go`，例如 `http/app/impl_agent.go`；HTTP JSON 适配与错误映射位于 `http/app/handler.go`。
- `entity` 不依赖业务处理包、领域服务、Gin、基础设施或生成的协议包。领域层和 `infra/httpx` 不依赖应用的 `entity` 或 `proto` 包；传输错误由 `httpx` 定义，handler 将其与公共外层组合。
- JSON 字段名、数字、零值、null、空数组和可选字段都属于接口契约，新增或修改字段时同步转换代码及契约测试。

Agent 的生成命令和 JSON 兼容规则见 [接口说明](../README.md)。
