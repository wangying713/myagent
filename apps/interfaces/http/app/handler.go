package app

import (
	"context"
	"errors"
	"net/http"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// httpObserver 由入口显式注入，HTTP 处理不依赖具体的观测实现。
type httpObserver interface {
	TracerProvider() trace.TracerProvider
}

func traceHTTP(observer httpObserver) gin.HandlerFunc {
	return otelgin.Middleware("http",
		otelgin.WithTracerProvider(observer.TracerProvider()),
		otelgin.WithMeterProvider(noop.NewMeterProvider()),
		// 沿用当前服务内链路，不读取全局传播器或引入外部请求头中的链路。
		otelgin.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		otelgin.WithSpanNameFormatter(func(*gin.Context) string { return "http.request" }),
		// 保留路由模板，屏蔽 otelgin 默认采集的原始路径与请求头派生值。
		otelgin.WithSpanStartOptions(trace.WithAttributes(
			attribute.String("url.path", "[REDACTED]"),
			attribute.String("user_agent.original", "[REDACTED]"),
			attribute.String("client.address", "[REDACTED]"),
		)),
	)
}

// writeHTTPRsp 统一包装业务结果与错误，业务失败时保留已有的部分结果。
func writeHTTPRsp(c *gin.Context, status int, data any, failure *httpx.Error) {
	span := trace.SpanFromContext(c.Request.Context())
	var traceID string
	if span.IsRecording() {
		traceID = span.SpanContext().TraceID().String()
	}
	rsp := entity.Rsp[any]{Code: entity.CodeOK, Data: data, TraceID: traceID, Msg: "成功"}
	if failure != nil {
		rsp.Code, rsp.Msg = failure.Code, failure.Message
	}
	// otelgin 按标准语义将 4xx 视为 Unset；本项目继续标记所有失败响应。
	if status >= http.StatusBadRequest {
		span.SetStatus(codes.Error, "HTTP 请求未成功")
	}
	if err := httpx.Write(c, status, rsp); err != nil {
		span.SetStatus(codes.Error, "响应编码失败")
		// 回退响应仅含固定字符串、nil 和已获取的 trace ID，保证可编码且不泄漏底层错误。
		_ = httpx.Write(c, http.StatusInternalServerError, entity.Rsp[any]{
			Code: entity.CodeInternalError, Data: nil, TraceID: traceID, Msg: "响应编码失败",
		})
	}
}

// chatRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func chatRspJSON(rsp *proto.ChatRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson: rsp.GetLesson(), Completed: rsp.GetCompleted(), Answer: rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(), ElapsedMS: rsp.GetElapsedMs(),
		ModelCalls: rsp.GetModelCalls(), ToolCalls: rsp.GetToolCalls(), KnownTokens: rsp.GetKnownTokens(),
		UsageComplete: rsp.GetUsageComplete(), Steps: agentStepsJSON(rsp.GetSteps()),
	}
}

// messagesRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func messagesRspJSON(rsp *proto.MessagesRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson: rsp.GetLesson(), Completed: rsp.GetCompleted(), Answer: rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(), ElapsedMS: rsp.GetElapsedMs(),
		ModelCalls: rsp.GetModelCalls(), ToolCalls: rsp.GetToolCalls(), KnownTokens: rsp.GetKnownTokens(),
		UsageComplete: rsp.GetUsageComplete(), Steps: agentStepsJSON(rsp.GetSteps()),
	}
}

// runRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func runRspJSON(rsp *proto.RunRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson: rsp.GetLesson(), Completed: rsp.GetCompleted(), Answer: rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(), ElapsedMS: rsp.GetElapsedMs(),
		ModelCalls: rsp.GetModelCalls(), ToolCalls: rsp.GetToolCalls(), KnownTokens: rsp.GetKnownTokens(),
		UsageComplete: rsp.GetUsageComplete(), Steps: agentStepsJSON(rsp.GetSteps()),
	}
}

// agentStepsJSON 保留共享步骤的 null、空数组、可选字段与未知用量语义。
func agentStepsJSON(steps []*proto.ExecutionStep) []entity.AgentExecutionStep {
	var data []entity.AgentExecutionStep
	if steps != nil {
		data = make([]entity.AgentExecutionStep, 0, len(steps))
	}
	for _, step := range steps {
		var usage *entity.AgentUsage
		if step.GetUsage() != nil {
			usage = &entity.AgentUsage{
				PromptTokens: step.Usage.PromptTokens, CompletionTokens: step.Usage.CompletionTokens,
				TotalTokens: step.Usage.TotalTokens,
			}
		}
		data = append(data, entity.AgentExecutionStep{
			Kind: step.GetKind(), Name: step.GetName(), Request: step.GetRequest(), Response: step.GetResponse(),
			ElapsedMS: step.GetElapsedMs(), FinishReason: step.GetFinishReason(), Usage: usage, Error: step.GetError(),
		})
	}
	return data
}

// classifyAgentError 在响应处理侧将 Agent 业务错误映射为 HTTP 错误。
func classifyAgentError(err error) (int, *httpx.Error) {
	if err == nil {
		return http.StatusOK, nil
	}
	switch {
	case errors.Is(err, agent.ErrInput):
		return http.StatusBadRequest, &httpx.Error{Code: entity.CodeInvalidInput, Message: "聊天与工具调用需要一个问题；历史消息需要两个非空问题，每个问题最多 4096 字符"}
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, &httpx.Error{Code: entity.CodeTimeout, Message: "执行超时，已停止后续调用"}
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, &httpx.Error{Code: entity.CodeCanceled, Message: "执行已取消"}
	case errors.Is(err, agent.ErrBudgetExceeded):
		return http.StatusConflict, &httpx.Error{Code: entity.CodeBudgetExceeded, Message: "达到运行预算，任务未完成"}
	case errors.Is(err, agent.ErrInvalidResponse):
		return http.StatusBadGateway, &httpx.Error{Code: entity.CodeInvalidModelResponse, Message: "模型未返回完整回答或工具协议无效"}
	default:
		return http.StatusBadGateway, &httpx.Error{Code: entity.CodeModelRequestFailed, Message: "模型服务调用失败，请通过执行记录和 trace ID 排查"}
	}
}
