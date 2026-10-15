# 通用 HTTP 请求与响应

此包供 Gin HTTP 入口复用，只提供传输机制，不导入任何业务包。

- `DecodeJSON`：限制 32 KiB 正文，要求 `application/json` 和单个 JSON 对象，按调用方声明的字段精确校验顶层名称，拒绝顶层的未知字段、重复字段、大小写别名和显式 null；按目标请求类型验证值的类型。
- `DecodeProtoJSON`：复用相同的正文与顶层检查，从 Proto 描述符提取原始字段名，再用官方 `protojson` 解码；返回实际出现的字段，供 API 保留 repeated 字段的省略与空数组区别。不认识具体 Agent 消息。
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

`httpx_test.go` 使用与 Agent 无关的目录请求测试复用契约，`proto_test.go` 使用标准生成消息验证 Proto 解码。响应如何适配现有 JSON 由 API 负责，不能把业务字段映射移入本包。领域层不应导入本包。
