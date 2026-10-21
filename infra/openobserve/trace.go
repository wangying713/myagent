package openobserve

import (
	"context"
	"encoding/json"
	"time"

	"saklay/ai/domain/agent"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Trace 持有单次业务运行的观测资源，由应用层负责结束。
type Trace struct {
	observeCli *Client    // 负责记录正文与脱敏的观测组件。
	span       trace.Span // 本次调用的链路 span。
	started    time.Time  // 本次调用的开始时间，用于统计耗时。
	operation  string     // 本次业务操作标识。
	deadline   string     // 本次运行的 UTC 截止时间，无截止时间时为空。
}

// traceInput 保存业务输入的观测预览，不承担领域或 HTTP 序列化契约。
type traceInput struct {
	Question   string      `json:"question,omitempty"`    // 聊天或工具循环的单个输入问题。
	Questions  []string    `json:"questions,omitempty"`   // 消息历史业务的两轮输入问题。
	Prompt     string      `json:"prompt"`                // 本次运行的系统提示词。
	Tools      []traceTool `json:"tools"`                 // 可供模型选择的工具定义预览。
	ToolChoice string      `json:"tool_choice,omitempty"` // 模型选择工具的策略。
	Limits     traceLimits `json:"limits"`                // 本次运行的预算预览。
	// Deadline 是业务上下文的 UTC 截止时间，格式为 RFC3339Nano。
	Deadline string `json:"deadline,omitempty"`
}

// traceLimits 保存单次业务运行的预算预览。
type traceLimits struct {
	ModelCalls int `json:"model_calls"` // 单次运行允许的最大模型调用次数。
	ToolCalls  int `json:"tool_calls"`  // 单次运行允许的最大工具调用次数。
	Tokens     int `json:"tokens"`      // 单次运行的累计已知 token 软预算。
}

// traceTool 保存提供给模型的工具协议预览。
type traceTool struct {
	Type     string        `json:"type"`     // 工具协议类型。
	Function traceFunction `json:"function"` // 供模型选择的函数定义预览。
}

// traceFunction 保存工具函数说明和参数约束的观测预览。
type traceFunction struct {
	Name        string `json:"name"`        // 已注册的工具函数名称。
	Description string `json:"description"` // 供模型选择工具时阅读的能力说明。
	// Parameters 展开参数 JSON Schema，无效 JSON 保留为文本。
	Parameters any `json:"parameters"`
}

// traceOutput 保存业务执行结果的观测预览。
type traceOutput struct {
	Operation     string            `json:"operation"`      // 本次业务操作标识。
	Completed     bool              `json:"completed"`      // 本次业务是否成功完成。
	Answer        string            `json:"answer"`         // 脱敏后的回答，失败时保留已有内容。
	ElapsedMS     int64             `json:"elapsed_ms"`     // 本次运行耗时，单位为毫秒。
	ModelCalls    int               `json:"model_calls"`    // 已发起的模型调用次数。
	ToolCalls     int               `json:"tool_calls"`     // 已发起的工具调用次数。
	KnownTokens   int               `json:"known_tokens"`   // 可确认的累计 token 用量。
	UsageComplete bool              `json:"usage_complete"` // 所有模型调用的用量是否均已确认并累计。
	Calls         []traceCallRecord `json:"steps"`          // 按执行顺序保存的模型与工具调用记录。
}

// traceCallRecord 保存单次模型或工具调用的观测预览。
type traceCallRecord struct {
	Kind         string       `json:"kind"`                    // 调用类型，区分模型调用与工具调用。
	Name         string       `json:"name"`                    // 模型调用说明或工具名称。
	Request      any          `json:"request"`                 // 展开的请求 JSON，无效或已截断的正文保留为文本。
	Response     any          `json:"response"`                // 展开的响应 JSON，无效或已截断的正文保留为文本。
	ElapsedMS    int64        `json:"elapsed_ms"`              // 本次调用耗时，单位为毫秒。
	FinishReason string       `json:"finish_reason,omitempty"` // 模型结束原因，工具调用时为空。
	Usage        *agent.Usage `json:"usage,omitempty"`         // 模型返回的 token 用量，工具调用时为 nil。
	Error        string       `json:"error,omitempty"`         // 执行失败或工具拒绝的公开说明。
}

// StartTrace 使用调用方指定的 span 名称建立运行观测，应用层在执行结束后提交实际输入和结果。
func (c *Client) StartTrace(ctx context.Context, spanName string, operation string) (context.Context, *Trace) {
	ctx, span := c.Tracer.Start(ctx, spanName, trace.WithAttributes(attribute.String("app.lesson", operation)))
	runTrace := &Trace{
		observeCli: c,
		span:       span,
		started:    time.Now(),
		operation:  operation,
	}
	if c.ContentLimit == 0 || !span.IsRecording() {
		return ctx, runTrace
	}
	if deadline, ok := ctx.Deadline(); ok {
		runTrace.deadline = deadline.UTC().Format(time.RFC3339Nano)
	}
	return ctx, runTrace
}

// End 记录实际输入、完整或部分结果和最终错误，并结束本次运行观测。
// result 为 nil 时只记录结束状态，不构造虚假的业务结果。
func (r *Trace) End(result *agent.ChatRsp, err error) {
	defer End(r.span, r.started, err)
	if result == nil || !r.span.IsRecording() {
		return
	}
	if r.observeCli.ContentLimit > 0 {
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

// previewJSON 保留有效 JSON 的层级与数值；截断、空值和无效 JSON 保留原文。
func previewJSON(text string) any {
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	return text
}

// inputPreview 转换实际执行输入，不在观测层重新推断默认值。
func (r *Trace) inputPreview(req agent.RunInput) *traceInput {
	input := traceInput{
		Prompt:     req.Prompt,
		Tools:      make([]traceTool, 0, len(req.Tools)),
		ToolChoice: req.ToolChoice,
		Limits:     traceLimits(req.Limits),
		Deadline:   r.deadline,
	}
	if r.operation == agent.OperationMessages {
		input.Questions = req.Questions
	} else if len(req.Questions) > 0 {
		input.Question = req.Questions[0]
	}
	for _, tool := range req.Tools {
		input.Tools = append(input.Tools, traceTool{
			Type: tool.Type,
			Function: traceFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  previewJSON(string(tool.Function.Parameters)),
			},
		})
	}
	return &input
}

// outputPreview 转换完整或部分结果，调用正文保留 JSON 数值及无效片段。
func outputPreview(result *agent.ChatRsp) *traceOutput {
	output := traceOutput{
		Operation:     result.Operation,
		Completed:     result.Completed,
		Answer:        result.Answer,
		ElapsedMS:     result.ElapsedMS,
		ModelCalls:    result.ModelCalls,
		ToolCalls:     result.ToolCalls,
		KnownTokens:   result.KnownTokens,
		UsageComplete: result.UsageComplete,
		Calls:         make([]traceCallRecord, 0, len(result.Calls)),
	}
	for _, record := range result.Calls {
		output.Calls = append(output.Calls, traceCallRecord{
			Kind:         record.Kind,
			Name:         record.Name,
			Request:      previewJSON(record.Request),
			Response:     previewJSON(record.Response),
			ElapsedMS:    record.ElapsedMS,
			FinishReason: record.FinishReason,
			Usage:        record.Usage,
			Error:        record.Error,
		})
	}
	return &output
}
