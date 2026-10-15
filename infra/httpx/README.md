# 通用 HTTP 请求与响应

此包供 Gin HTTP 入口复用，只提供传输机制，不导入任何业务包。

- `DecodeJSON`：限制 32 KiB 正文，要求 `application/json` 和单个 JSON 对象，按调用方声明的字段精确校验顶层名称，拒绝顶层的未知字段、重复字段、大小写别名和显式 null；按目标请求类型验证值的类型。
- `DecodeProtoJSON`：复用相同的正文与顶层检查，从 Proto 描述符提取原始字段名，再用官方 `protojson` 解码；返回实际出现的字段，供 API 保留 repeated 字段的省略与空数组区别。不认识具体 Agent 消息。
- `NewAdapter` / `Adapt`：将 `(context.Context, *请求) (*响应, error)` 方法适配为 Gin 处理器，统一严格解码、超时、取消和响应写出。API 注入具体数据编码与业务错误映射，错误不会导致返回的数据被丢弃。
- `Response[T]`：公共 `data` / `error` 响应结构；业务响应数据由各 API 定义。
- `WriteResult` / `Write`：先编码后写出，禁止缓存；编码失败返回固定 JSON 500，不能泄漏底层错误或写出半个成功响应。
- `RequestError` / `WriteRequestError`：内容类型、JSON 结构、正文上限等传输错误。业务错误的 HTTP 映射由具体 API 完成。

请求字段是否必填、允许什么范围和数量，由业务契约决定。本包不定义 Agent、模型、工具或其他后台业务。

以下是独立的数据目录示例，不表示项目已经实现该业务：

```go
type CreateRequest struct {
	Name string `json:"name"`
}

func Create(c *gin.Context) {
	var request CreateRequest
	if err := httpx.DecodeJSON(c, &request, "name"); err != nil {
		httpx.WriteRequestError(c, err, nil)
		return
	}
	// 这里调用所属领域的业务方法，再转换为 API 响应数据。
	httpx.WriteResult(c, http.StatusOK, request, nil)
}
```

## 统一适配器

`NewAdapter(timeout, encode, classify)` 在启动时校验正数超时、编码回调和错误映射，构造后可以供同类响应的多个接口共用。`encode` 接收 API 返回的响应，包括解析失败时响应类型的零值；它决定具体 JSON 数据格式。`classify` 接收 API 返回的错误，包括成功时的 nil，决定 HTTP 状态及公共错误。回调应保持配置不变，不共享请求状态，以支持并发调用。

路由逐条使用 `Adapt(adapter, apiMethod)` 绑定具名方法。请求必须是生成的 Proto 消息指针，由适配器为每次请求单独分配；响应通过方法返回，不传入待填充的响应对象，也不复制生成消息的内部状态。

需要保留字段存在性时，可在路由上显式传入解码后准备函数：`Adapt(adapter, apiMethod, prepare)`。准备函数接收请求和实际出现的顶层字段，只在严格解码成功后执行；业务不需要从上下文取回这些参数。现有消息接口用它区分省略与显式空数组。

总超时从成功解码及准备完成后开始，父上下文来自 HTTP 请求。适配器同步调用 API，等待方法返回后写出响应，不额外启动后台业务协程；方法需要沿调用链响应上下文取消。HTTP 读取正文的超时仍由入口的 `http.Server` 管理。

`httpx_test.go` 使用与 Agent 无关的目录请求测试复用契约，`proto_test.go` 使用标准生成消息验证 Proto 解码，`adapter_test.go` 验证部分结果、错误、取消、超时、字段准备、上下文释放与并发隔离。响应如何适配现有 JSON 由 API 负责，不能把业务字段映射移入本包。领域层不应导入本包。
