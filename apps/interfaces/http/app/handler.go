package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"
	"unicode/utf8"

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
	pb "google.golang.org/protobuf/proto"
)

// requestTimeoutKey 隔离 Gin 请求作用域中的业务超时配置。
type requestTimeoutKey struct{}

// requestTimeout 将当前 HTTP 实例的超时设置写入 Gin 请求作用域，此时尚不开始计时。
func requestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(requestTimeoutKey{}, timeout)
		c.Next()
	}
}

// protoRequest 限定可独立分配的 Protobuf 消息指针，避免在请求间复用消息状态。
type protoRequest[Req any] interface {
	*Req
	pb.Message
}

// handle 将业务方法接入 Gin，须配合 requestTimeout 设置业务超时。
// 解码及请求准备完成后开始计时，同步等待 API 返回并释放上下文；API 须响应上下文取消。
// 响应与错误可以同时返回，已有结果会继续写入 HTTP data。
func handle[Req, Rsp any, PReq protoRequest[Req]](api func(context.Context, PReq) (Rsp, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := c.GetDuration(requestTimeoutKey{})
		if timeout <= 0 {
			writeHTTPRsp(c, http.StatusInternalServerError, nil, &httpx.Error{
				Code:    entity.CodeInternalError,
				Message: "服务处理异常，请稍后重试",
			})
			return
		}
		req := PReq(new(Req))
		present, err := httpx.DecodeProtoJSON(c, req)
		if err != nil {
			status, failure := httpx.ClassifyReqError(err)
			writeHTTPRsp(c, status, nil, failure)
			return
		}
		if messages, ok := any(req).(*proto.MessagesReq); ok {
			prepareAgentMessages(messages, present)
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		rsp, err := api(ctx, req)
		writeAPIResult(c, rsp, err)
	}
}

// writeAPIResult 集中选择各接口的 HTTP JSON 表现，并保留错误同时返回的部分结果。
func writeAPIResult(c *gin.Context, rsp any, err error) {
	var data any
	switch rsp := rsp.(type) {
	case nil:
	case *proto.ChatRsp:
		data = chatRspJSON(rsp)
	case *proto.MessagesRsp:
		data = messagesRspJSON(rsp)
	case *proto.RunRsp:
		data = runRspJSON(rsp)
	default:
		// 新增协议须显式配置 HTTP 表现，避免标准 ProtoJSON 或生成标签改变契约。
		writeHTTPRsp(c, http.StatusInternalServerError, nil, &httpx.Error{
			Code:    entity.CodeInternalError,
			Message: "响应编码失败",
		})
		return
	}
	status, failure := classifyAgentError(err)
	writeHTTPRsp(c, status, data, failure)
}

// httpObserver 由入口显式注入，HTTP 处理不依赖具体的观测实现。
type httpObserver interface {
	TracerProvider() trace.TracerProvider
	ContentLimit() int
	Payload(trace.Span, string, []byte)
}

// traceHTTP 在解码前建立请求链路，显式使用私有观测配置并屏蔽敏感请求属性。
func traceHTTP(observer httpObserver) gin.HandlerFunc {
	return otelgin.Middleware("http",
		otelgin.WithTracerProvider(observer.TracerProvider()),
		otelgin.WithSpanNameFormatter(func(c *gin.Context) string {
			if route := c.FullPath(); route != "" {
				return route
			}
			return "[unmatched]"
		}),
		otelgin.WithMeterProvider(noop.NewMeterProvider()),
		// 沿用当前服务内链路，不读取全局传播器或引入外部请求头中的链路。
		otelgin.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		// 保留路由模板，屏蔽 otelgin 默认采集的原始路径与请求头派生值。
		otelgin.WithSpanStartOptions(trace.WithAttributes(
			attribute.String("url.path", "[REDACTED]"),
			attribute.String("user_agent.original", "[REDACTED]"),
			attribute.String("client.address", "[REDACTED]"),
		)),
	)
}

// previewHTTP 在请求 span 结束前记录实际 HTTP 正文，复用观测组件的脱敏与预览适配。
func previewHTTP(observer httpObserver) gin.HandlerFunc {
	// 一个 Unicode 字符最多占四字节；有界读写副本不额外消耗请求正文。
	limit := min(observer.ContentLimit(), math.MaxInt/utf8.UTFMax) * utf8.UTFMax
	capture := httpx.CaptureBodies(limit, func(c *gin.Context, request, response []byte) {
		span := trace.SpanFromContext(c.Request.Context())
		observer.Payload(span, "app.request", request)
		observer.Payload(span, "app.response", response)
	})
	return func(c *gin.Context) {
		if !trace.SpanFromContext(c.Request.Context()).IsRecording() {
			c.Next()
			return
		}
		capture(c)
	}
}

// writeHTTPRsp 统一包装业务结果与错误，业务失败时保留已有的部分结果。
func writeHTTPRsp(c *gin.Context, status int, data any, failure *httpx.Error) {
	span := trace.SpanFromContext(c.Request.Context())
	var traceID string
	if span.IsRecording() {
		traceID = span.SpanContext().TraceID().String()
	}
	rsp := entity.Rsp[any]{
		Code:    entity.CodeOK,
		Data:    data,
		TraceID: traceID,
		Msg:     "成功",
	}
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
			Code:    entity.CodeInternalError,
			Data:    nil,
			TraceID: traceID,
			Msg:     "响应编码失败",
		})
	}
}

// chatRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func chatRspJSON(rsp *proto.ChatRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson:           rsp.GetLesson(),
		Completed:        rsp.GetCompleted(),
		Answer:           rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(),
		ElapsedMS:        rsp.GetElapsedMs(),
		ModelCalls:       rsp.GetModelCalls(),
		ToolCalls:        rsp.GetToolCalls(),
		KnownTokens:      rsp.GetKnownTokens(),
		UsageComplete:    rsp.GetUsageComplete(),
		Steps:            agentStepsJSON(rsp.GetSteps()),
	}
}

// messagesRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func messagesRspJSON(rsp *proto.MessagesRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson:           rsp.GetLesson(),
		Completed:        rsp.GetCompleted(),
		Answer:           rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(),
		ElapsedMS:        rsp.GetElapsedMs(),
		ModelCalls:       rsp.GetModelCalls(),
		ToolCalls:        rsp.GetToolCalls(),
		KnownTokens:      rsp.GetKnownTokens(),
		UsageComplete:    rsp.GetUsageComplete(),
		Steps:            agentStepsJSON(rsp.GetSteps()),
	}
}

// runRspJSON 保留本接口的 HTTP JSON 字段、数字与零值表现。
func runRspJSON(rsp *proto.RunRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.AgentExecutionResult{
		Lesson:           rsp.GetLesson(),
		Completed:        rsp.GetCompleted(),
		Answer:           rsp.GetAnswer(),
		TelemetryEnabled: rsp.GetTelemetryEnabled(),
		ElapsedMS:        rsp.GetElapsedMs(),
		ModelCalls:       rsp.GetModelCalls(),
		ToolCalls:        rsp.GetToolCalls(),
		KnownTokens:      rsp.GetKnownTokens(),
		UsageComplete:    rsp.GetUsageComplete(),
		Steps:            agentStepsJSON(rsp.GetSteps()),
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
				PromptTokens:     step.Usage.PromptTokens,
				CompletionTokens: step.Usage.CompletionTokens,
				TotalTokens:      step.Usage.TotalTokens,
			}
		}
		data = append(data, entity.AgentExecutionStep{
			Kind:         step.GetKind(),
			Name:         step.GetName(),
			Request:      step.GetRequest(),
			Response:     step.GetResponse(),
			ElapsedMS:    step.GetElapsedMs(),
			FinishReason: step.GetFinishReason(),
			Usage:        usage,
			Error:        step.GetError(),
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
		return http.StatusBadRequest, &httpx.Error{
			Code:    entity.CodeInvalidInput,
			Message: fmt.Sprintf("聊天与工具调用需要一个问题；历史消息需要两个非空问题，每个问题最多 %d 字符", agent.MaxQuestionChars),
		}
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, &httpx.Error{
			Code:    entity.CodeTimeout,
			Message: "执行超时，已停止后续调用",
		}
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, &httpx.Error{
			Code:    entity.CodeCanceled,
			Message: "执行已取消",
		}
	case errors.Is(err, agent.ErrBudgetExceeded):
		return http.StatusConflict, &httpx.Error{
			Code:    entity.CodeBudgetExceeded,
			Message: "达到运行预算，任务未完成",
		}
	case errors.Is(err, agent.ErrInvalidResponse):
		return http.StatusBadGateway, &httpx.Error{
			Code:    entity.CodeInvalidModelResponse,
			Message: "模型未返回完整回答或工具协议无效",
		}
	default:
		return http.StatusBadGateway, &httpx.Error{
			Code:    entity.CodeModelRequestFailed,
			Message: "模型服务调用失败，请通过执行记录和 trace ID 排查",
		}
	}
}
