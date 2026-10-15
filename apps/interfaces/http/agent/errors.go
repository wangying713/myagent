package agentapi

import (
	"context"
	"errors"
	"net/http"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/httpx"
)

// Agent 业务错误的 HTTP 映射留在本接口包，不能进入通用 httpx。
func classify(err error) (int, *httpx.Error) {
	if err == nil {
		return http.StatusOK, nil
	}
	switch {
	case errors.Is(err, agent.ErrInput):
		return http.StatusBadRequest, &httpx.Error{Code: "invalid_input", Message: "聊天与工具调用需要一个问题；历史消息需要两个非空问题，每个问题最多 4096 字符"}
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, &httpx.Error{Code: "timeout", Message: "执行超时，已停止后续调用"}
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, &httpx.Error{Code: "canceled", Message: "执行已取消"}
	case errors.Is(err, agent.ErrBudgetExceeded):
		return http.StatusConflict, &httpx.Error{Code: "budget_exceeded", Message: "达到运行预算，任务未完成"}
	case errors.Is(err, agent.ErrInvalidResponse):
		return http.StatusBadGateway, &httpx.Error{Code: "invalid_model_response", Message: "模型未返回完整回答或工具协议无效"}
	default:
		return http.StatusBadGateway, &httpx.Error{Code: "model_request_failed", Message: "模型服务调用失败，请通过执行记录和 trace ID 排查"}
	}
}
