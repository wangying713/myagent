package entity

// HTTP 响应码是公开接口契约；HTTP 状态和具体提示由接口适配层决定。
const (
	CodeOK                   = "ok"                     // 请求成功。
	CodeInvalidInput         = "invalid_input"          // JSON 结构或业务输入无效。
	CodeNotFound             = "not_found"              // 请求路径未注册。
	CodeMethodNotAllowed     = "method_not_allowed"     // 路径存在但不支持当前请求方法。
	CodeRequestTooLarge      = "request_too_large"      // 请求正文超过接口上限。
	CodeInvalidContentType   = "invalid_content_type"   // 请求未使用受支持的 JSON 内容类型。
	CodeBudgetExceeded       = "budget_exceeded"        // 执行达到调用次数或已知用量预算。
	CodeCanceled             = "canceled"               // 执行已被取消。
	CodeTimeout              = "timeout"                // 执行超过截止时间。
	CodeInvalidModelResponse = "invalid_model_response" // 模型响应或工具调用协议无效。
	CodeModelRequestFailed   = "model_request_failed"   // 模型服务调用失败。
	CodeInternalError        = "internal_error"         // 接口内部处理或响应编码失败。
	CodeServiceUnavailable   = "service_unavailable"    // 服务退出期间拒绝新请求。
)
