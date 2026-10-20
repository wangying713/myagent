# 通用 HTTP 请求与响应

此包供 Gin HTTP 入口复用，只提供传输机制，不导入任何业务包。

- `DecodeJSON`：根据手写结构体的准确 JSON 标签解码，限制 32 KiB 正文，要求 `application/json` 和单个 JSON 对象，拒绝未知字段、重复字段、字段名别名及 null，嵌套对象和数组元素同样检查。省略切片保持 nil，显式空数组保持非 nil 空切片。可选字段用指针区分省略与零值；整数输入兼容数字、数字字符串和整数值指数形式，避免浮点精度损失。不依赖任何应用类型。
- `CaptureBodies`：旁路记录处理器实际读取的请求体与写出的响应体，副本有字节上限与截断标记；不提前读请求，不改变状态、正文或底层读写错误。调用方负责正文开关、脱敏与展示适配。
- `Write`：先编码后写出，禁止缓存；编码失败返回 error，尚不写出任何响应头或正文，由调用方生成其统一 JSON 500。
- `RequestError` / `ClassifyReqError`：内容类型、JSON 结构、正文上限等传输错误。业务错误的 HTTP 映射由具体 API 完成。

请求字段是否必填、允许什么范围和数量，由业务契约决定。本包不定义 Agent、模型、工具或其他后台业务。

HTTP 外层由应用的 `entity.Rsp[T]` 定义，`http/app/handler.go` 统一包装 `code/data/traceID/msg`。本包既不定义应用响应外层，也不导入应用的 `entity`。

## 与入口处理层的分工

路由使用 `handle(implAgent.Chat)` 绑定业务方法；`handle` 位于 `apps/interfaces/http/app/handler.go`，负责请求分配、调用本包严格解码、超时取消及响应处理。路由初始化只设置一次当前路由的超时，不分别创建 Adapter 或注入每个接口的回调。

总超时从成功解码后开始，父上下文来自 HTTP 请求。处理函数同步调用 API，等待返回后写出响应并释放上下文，保留同时返回的部分结果与错误。HTTP 读取正文的超时仍由入口的 `http.Server` 管理。

`httpx_test.go` 验证响应编码及失败回退，`json_test.go` 和 `json_integer_test.go` 使用手写数据验证严格解码、切片存在性及整数输入兼容。入口的 `app/handle_test.go`、`app/impl_agent_test.go` 与 `app/router_test.go` 验证响应兼容、默认输入、取消、超时、上下文释放及并发隔离。业务字段映射和响应外层留在入口；领域层不导入本包。
