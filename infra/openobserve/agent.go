package openobserve

import (
	"context"
	"encoding/json"
	"time"

	"saklay/ai/domain/agent"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ agent.Observer = (*Telemetry)(nil)

// StartRun 将领域执行事件适配为 span，沿用入口传入的父链路，领域层无需依赖 OpenTelemetry。
func (t *Telemetry) StartRun(ctx context.Context, operation string) (context.Context, agent.RunObservation) {
	ctx, span := t.Start(ctx, "agent.run", attribute.String("app.lesson", operation))
	return ctx, &runObservation{span: span, started: time.Now()}
}

type runObservation struct {
	span    trace.Span
	started time.Time
}

func (r *runObservation) TraceID() string {
	if !r.span.SpanContext().IsValid() {
		return ""
	}
	return r.span.SpanContext().TraceID().String()
}

func (r *runObservation) End(result agent.ExecutionResult, err error) {
	r.span.SetAttributes(attribute.Int("app.llm_calls", result.ModelCalls), attribute.Int("app.tool_calls", result.ToolCalls), attribute.Int("app.total_tokens", result.KnownTokens), attribute.Bool("app.usage.complete", result.UsageComplete))
	End(r.span, r.started, err)
}

// StartTool 沿用父链路记录工具请求，调用方须在执行结束后调用返回句柄的 End。
func (t *Telemetry) StartTool(ctx context.Context, call agent.ToolCall) (context.Context, agent.ToolObservation) {
	ctx, span := t.Start(ctx, "Tool call", attribute.String("gen_ai.operation.name", "execute_tool"), attribute.String("app.tool.name", t.Clean(call.Function.Name)), attribute.String("app.tool.call.id", t.Clean(call.ID)))
	// ToolCall 仅含字符串字段，不存在 JSON 编码失败的值。
	request, _ := json.Marshal(call)
	t.Payload(span, "app.request", request)
	return ctx, &toolObservation{telemetry: t, span: span, started: time.Now()}
}

type toolObservation struct {
	telemetry *Telemetry
	span      trace.Span
	started   time.Time
}

func (t *toolObservation) End(output agent.ToolResult, err error) {
	if response, encodeErr := json.Marshal(output); encodeErr == nil {
		t.telemetry.Payload(t.span, "app.response", response)
	}
	if !output.OK {
		t.span.SetStatus(codes.Error, "工具调用被拒绝")
		t.span.SetAttributes(attribute.String("app.tool.error", t.telemetry.Clean(output.Error)))
	}
	End(t.span, t.started, err)
}
