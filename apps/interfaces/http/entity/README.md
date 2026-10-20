# HTTP 请求、响应与共用数据

本目录统一保存手写的 HTTP 请求、响应、公共外层及共用 JSON 数据。业务处理方法、请求准备与响应组装放在 `http/app` 下对应业务的 `impl_xxx.go`，HTTP 解码和错误映射位于 `app/handler.go`，通用严格解析和响应编码由 `infra/httpx` 提供。

```text
http/entity/
  agent.go                  # Agent 请求、独立响应、共享步骤和用量
  agent_demos.go            # 各 Demo 请求、独立响应、执行记录和共用数据
  response.go               # HTTP 统一外层 Rsp[T]：code/data/traceID/msg
  code.go                   # HTTP 公开响应码常量
```

后续新增接口遵循以下规则：

- 请求使用 `Req` 后缀，响应使用 `Rsp` 后缀，例如 `entity.ChatReq` 和 `entity.ChatRsp`；各接口独立定义响应，按需复用步骤、用量等共用数据。
- 类型按业务分文件，同一接口的请求与响应相邻排列；Demo 专用执行记录 `DemoExecution` 放在 `agent_demos.go`，步骤和用量由两类接口复用。每个字段显式设置准确的 JSON 标签并补充中文含义、单位和必要的 nil 或零值语义。
- API 方法直接返回手写响应；handler 使用 `entity.Rsp[T]` 包装，trace ID 只放外层，不再维护另一套中间协议类型与 JSON 转换。
- 公共响应码集中在 `code.go`；领域错误判断、HTTP 状态和具体提示保留在 `app/handler.go`。`infra/httpx` 独立定义传输错误，不反向导入本目录。
- 数据类型只承载接口字段和 JSON 标签；请求准备与响应组装位于对应 `app/impl_xxx.go`。`entity` 不依赖业务处理包、领域服务、Gin 或基础设施；领域层及 `infra/httpx` 不依赖应用的 `entity` 包。
- `question` 省略和空字符串使用默认问题；`questions` 的 nil 表示省略，非 nil 空切片表示显式空数组；可选数值用指针区分省略与零值，显式 null 仍由解码器拒绝。
- JSON 字段名、数字、零值、null、空数组和可选字段都属于接口契约，修改时同步响应组装及契约测试。

严格解码与 JSON 兼容规则见 [接口说明](../README.md)。
