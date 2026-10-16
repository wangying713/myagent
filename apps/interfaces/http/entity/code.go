package entity

// HTTP 响应码是公开接口契约；HTTP 状态和具体提示由接口适配层决定。
const (
	// CodeOK 表示请求成功。
	CodeOK = "ok"
	// CodeInvalidInput 表示 JSON 结构或业务输入无效。
	CodeInvalidInput = "invalid_input"
	// CodeNotFound 表示请求路径未注册。
	CodeNotFound = "not_found"
	// CodeMethodNotAllowed 表示路径存在但不支持当前请求方法。
	CodeMethodNotAllowed = "method_not_allowed"
	// CodeRequestTooLarge 表示请求正文超过接口上限。
	CodeRequestTooLarge = "request_too_large"
	// CodeInvalidContentType 表示请求未使用受支持的 JSON 内容类型。
	CodeInvalidContentType = "invalid_content_type"
	// CodeBudgetExceeded 表示执行达到调用次数或已知用量预算。
	CodeBudgetExceeded = "budget_exceeded"
	// CodeCanceled 表示执行已被取消。
	CodeCanceled = "canceled"
	// CodeTimeout 表示执行超过截止时间。
	CodeTimeout = "timeout"
	// CodeInvalidModelResponse 表示模型响应或工具调用协议无效。
	CodeInvalidModelResponse = "invalid_model_response"
	// CodeModelRequestFailed 表示模型服务调用失败。
	CodeModelRequestFailed = "model_request_failed"
	// CodeInternalError 表示接口内部处理或响应编码失败。
	CodeInternalError = "internal_error"
	// CodeServiceUnavailable 表示服务退出期间拒绝新请求。
	CodeServiceUnavailable = "service_unavailable"
)
