# 通用 HTTP 请求与响应

此包供 Gin HTTP 入口复用，只提供传输机制，不导入任何业务包。

- `DecodeJSON`：限制 32 KiB 正文，要求 `application/json` 和单个 JSON 对象，按调用方声明的字段精确校验顶层名称，拒绝顶层的未知字段、重复字段、大小写别名和显式 null；按目标请求类型验证值的类型。
- `DecodeProtoJSON`：复用相同的正文与顶层检查，从 Proto 描述符提取原始字段名，再用官方 `protojson` 解码；返回实际出现的字段，供 API 保留 repeated 字段的省略与空数组区别。不认识具体 Agent 消息。
- `CaptureBodies`：旁路记录处理器实际读取的请求体与写出的响应体，副本有字节上限与截断标记；不提前读请求，不改变状态、正文或底层读写错误。调用方负责正文开关、脱敏与展示适配。
- `Write`：先编码后写出，禁止缓存；编码失败返回 error，尚不写出任何响应头或正文，由调用方生成其统一 JSON 500。
- `RequestError` / `ClassifyReqError`：内容类型、JSON 结构、正文上限等传输错误。业务错误的 HTTP 映射由具体 API 完成。

请求字段是否必填、允许什么范围和数量，由业务契约决定。本包不定义 Agent、模型、工具或其他后台业务。

HTTP 外层由应用的 `entity.Rsp[T]` 定义，`http/app/handler.go` 统一包装 `code/data/traceID/msg`。本包既不定义应用响应外层，也不导入应用的 `entity`。

## 与入口处理层的分工

路由使用 `handle(implAgent.Chat)` 绑定业务方法；`handle` 位于 `apps/interfaces/http/app/handler.go`，负责请求分配、调用本包严格解码、字段准备、超时取消及响应处理。路由初始化只设置一次当前路由的超时，不分别创建 Adapter 或注入每个接口的回调。

总超时从成功解码及准备完成后开始，父上下文来自 HTTP 请求。处理函数同步调用 API，等待返回后写出响应并释放上下文，保留同时返回的部分结果与错误。HTTP 读取正文的超时仍由入口的 `http.Server` 管理。

`httpx_test.go` 使用与 Agent 无关的目录请求验证复用契约，`proto_test.go` 使用标准生成消息验证 Proto 解码。入口的 `app/handle_test.go`、`app/impl_agent_test.go` 与 `app/router_test.go` 验证响应兼容、请求准备、取消、超时、上下文释放及并发隔离。业务字段映射和响应外层留在入口；领域层不导入本包。
