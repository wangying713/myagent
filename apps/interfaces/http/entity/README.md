# HTTP 请求、响应与共用数据

本目录保存手写的 HTTP 请求、响应、公共外层及接口专用 JSON 数据；字段与语义一致的调用记录、用量直接复用领域类型。业务处理方法、请求准备与响应组装放在 `http/app` 下对应业务的 `impl_xxx.go`，HTTP 解码和错误映射位于 `app/handler.go`，通用严格解析和响应编码由 `infra/httpx` 提供。

```text
http/entity/
  agent.go                  # Agent 请求、独立响应与共用执行结果
  agent_demos.go            # 各 Demo 请求、独立响应和专用数据
  response.go               # HTTP 统一外层 Rsp[T]：code/data/traceID/msg
  code.go                   # HTTP 公开响应码常量
```

后续新增接口遵循以下规则：

- 请求使用 `Req` 后缀，响应使用 `Rsp` 后缀，例如 `entity.ChatReq` 和 `entity.ChatRsp`；各接口独立定义响应，按需复用步骤、用量等共用数据。
- 类型按业务分文件，同一接口的请求与响应相邻排列；共用执行结果 `AgentResult` 放在 `agent.go`，Demo 响应按需引用；调用记录与用量直接使用 `domain/agent.CallRecord`、`Usage`，不定义对应的 HTTP 副本。每个字段显式设置准确的 JSON 标签并补充中文含义、单位和必要的 nil 或零值语义。
- API 方法直接返回手写响应；handler 使用 `entity.Rsp[T]` 包装，trace ID 只放外层，不再维护另一套中间协议类型与 JSON 转换。
- 公共响应码集中在 `code.go`；领域错误判断、HTTP 状态和具体提示保留在 `app/handler.go`。`infra/httpx` 独立定义传输错误，不反向导入本目录。
- 数据类型只承载接口字段和 JSON 标签；请求准备与响应组装位于对应 `app/impl_xxx.go`。`entity` 允许引用上述领域数据类型，不调用领域服务，不依赖业务处理包、Gin 或基础设施；领域层及 `infra/httpx` 不依赖应用的 `entity` 包。
- `question` 省略和空字符串使用默认问题；`questions` 的 nil 表示省略，非 nil 空切片表示显式空数组；可选数值用指针区分省略与零值，显式 null 仍由解码器拒绝。
- 调用记录在 Go 中使用 `Calls []agent.CallRecord`，JSON 标签仍为 `steps`；其字段标签由领域类型提供。应用层先完成脱敏，再直接赋值给响应，保留 nil、空数组和未知用量的表现。
- JSON 字段名、数字、零值、null、空数组和可选字段都属于接口契约，修改时同步响应组装及契约测试。

严格解码与 JSON 兼容规则见 [接口说明](../README.md)。

顶层 HTTP 响应含观测启用状态，领域结果含内部执行输入，因此继续独立定义。Trace 预览会把正文字符串展开成 JSON 对象，与 HTTP 调用记录的字符串字段不同，保留专用转换。
