package openobserve

import (
	"context"
	"encoding/json"
	"time"

	"saklay/ai/domain/agent"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// runSpanName 是业务运行 span 的稳定名称。
const runSpanName = "agent.run"

// ModelInfo 描述模型客户端的公开配置，不包含鉴权信息。
type ModelInfo struct {
	Provider     string        // 模型服务提供方。
	Name         string        // 请求使用的模型名称。
	Endpoint     string        // 不包含凭据的模型请求地址。
	Timeout      time.Duration // 单次模型调用的超时时长。
	MaxTokens    int           // 单次模型请求的最大输出 token 数。
	ThinkingType string        // 模型思考模式。
	Stream       bool          // 是否使用流式响应。
}

// Run 持有单次业务运行的观测资源，由应用层负责结束。
type Run struct {
	observeCli *Client    // 负责记录正文与脱敏的观测组件。
	span       trace.Span // 本次调用的链路 span。
	started    time.Time  // 本次调用的开始时间，用于统计耗时。
	operation  string     // 本次业务操作标识。
	model      *ModelInfo // 运行开始时的模型公开配置快照；nil 表示未提供。
	deadline   string     // 本次运行的 UTC 截止时间，无截止时间时为空。
}

// StartRun 建立业务运行 span，应用层在执行结束后提交实际输入和结果。
func (c *Client) StartRun(ctx context.Context, operation string, model *ModelInfo) (context.Context, *Run) {
	ctx, span := c.Tracer.Start(ctx, runSpanName, trace.WithAttributes(attribute.String("app.lesson", operation)))
	run := &Run{
		observeCli: c,
		span:       span,
		started:    time.Now(),
		operation:  operation,
	}
	if !c.captureContent || !span.IsRecording() {
		return ctx, run
	}
	if model != nil {
		snapshot := *model
		run.model = &snapshot
	}
	if deadline, ok := ctx.Deadline(); ok {
		run.deadline = deadline.UTC().Format(time.RFC3339Nano)
	}
	return ctx, run
}

// End 记录实际输入、完整或部分结果和最终错误，并结束本次运行。
// result 为 nil 时只记录结束状态，不构造虚假的业务结果。
func (r *Run) End(result *agent.ChatRsp, err error) {
	defer End(r.span, r.started, err)
	if result == nil || !r.span.IsRecording() {
		return
	}
	if r.observeCli.captureContent {
		// 白名单字段仅含 JSON 标量、容器及已校验的 RawMessage，编码不会失败。
		request, _ := json.Marshal(r.inputPreview(result.Input))
		response, _ := json.Marshal(outputPreview(result))
		r.observeCli.Payload(r.span, requestPayloadKey, request)
		r.observeCli.Payload(r.span, responsePayloadKey, response)
	}
	r.span.SetAttributes(
		attribute.Int("app.llm_calls", result.ModelCalls),
		attribute.Int("app.tool_calls", result.ToolCalls),
		attribute.Int("app.total_tokens", result.KnownTokens),
		attribute.Bool("app.usage.complete", result.UsageComplete),
	)
}
