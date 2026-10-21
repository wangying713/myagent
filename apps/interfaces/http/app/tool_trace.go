package app

import (
	"context"
	"encoding/json"
	"time"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracedTools 在实际工具执行期间建立子 span，工具定义直接复用原集合。
type tracedTools struct {
	agent.Tools                     // 实际执行的工具集合，沿用其定义和分发规则。
	observeCli  *openobserve.Client // 写入脱敏预览的观测客户端。
}

// toolTraceInput 平铺工具标识并展开实际参数，保留旧版输入预览的字段格式。
type toolTraceInput struct {
	ID        string `json:"id"`        // 用于关联模型请求和工具结果的调用 ID。
	Type      string `json:"type"`      // 工具协议类型，当前为 function。
	Name      string `json:"name"`      // 待执行的工具名称。
	Arguments any    `json:"arguments"` // 有效 JSON 展开为对象或值，无效片段保留为文本。
}

// Dispatch 保留工具返回值和错误，单独记录参数、结果、耗时及失败状态。
func (t *tracedTools) Dispatch(ctx context.Context, req agent.ToolCallReq) (result *agent.ToolCallRsp, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	toolName := t.observeCli.Clean(req.Function.Name)
	spanName := toolName
	if spanName == "" {
		spanName = "tool.execute"
	}
	ctx, span := t.observeCli.Tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("app.tool.name", toolName),
			attribute.String("app.tool.call.id", t.observeCli.Clean(req.ID)),
		),
	)
	started := time.Now()
	defer func() {
		openobserve.End(span, started, err)
	}()
	if t.observeCli.ContentLimit > 0 && span.IsRecording() {
		var arguments any = req.Function.Arguments
		if json.Valid([]byte(req.Function.Arguments)) {
			arguments = json.RawMessage(req.Function.Arguments)
		}
		// 标识均为字符串，参数仅使用文本或已校验的 JSON，编码不会失败。
		request, _ := json.Marshal(toolTraceInput{
			ID:        req.ID,
			Type:      req.Type,
			Name:      req.Function.Name,
			Arguments: arguments,
		})
		t.observeCli.Payload(span, "app.request", request)
	}
	result, err = t.Tools.Dispatch(ctx, req)
	if result != nil {
		span.SetAttributes(attribute.Bool("app.tool.ok", result.OK))
		if !result.OK {
			span.SetStatus(codes.Error, "工具拒绝执行")
			span.SetAttributes(attribute.String("app.tool.error", t.observeCli.Clean(result.Error)))
		}
	} else if err == nil {
		span.SetStatus(codes.Error, "工具未返回执行结果")
	}
	if result != nil && t.observeCli.ContentLimit > 0 && span.IsRecording() {
		response, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			span.SetStatus(codes.Error, "工具结果无法编码")
		} else {
			t.observeCli.Payload(span, "app.response", response)
		}
	}
	return result, err
}
