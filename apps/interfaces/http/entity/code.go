package entity

// HTTP 响应码是公开接口契约；HTTP 状态和具体提示由接口适配层决定。
const (
	CodeOK                   = "ok"
	CodeInvalidInput         = "invalid_input"
	CodeNotFound             = "not_found"
	CodeMethodNotAllowed     = "method_not_allowed"
	CodeRequestTooLarge      = "request_too_large"
	CodeInvalidContentType   = "invalid_content_type"
	CodeBudgetExceeded       = "budget_exceeded"
	CodeCanceled             = "canceled"
	CodeTimeout              = "timeout"
	CodeInvalidModelResponse = "invalid_model_response"
	CodeModelRequestFailed   = "model_request_failed"
	CodeInternalError        = "internal_error"
	CodeServiceUnavailable   = "service_unavailable"
)
