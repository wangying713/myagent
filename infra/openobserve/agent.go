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

// StartRun 记录实际运行输入，沿用父链路；领域层无需依赖 OpenTelemetry。
func (t *Telemetry) StartRun(ctx context.Context, operation string, details agent.RunDetails) (context.Context, agent.RunObservation) {
	ctx, span := t.Start(ctx, "agent.run", attribute.String("app.lesson", operation))
	input := runInput{
		Prompt:     details.Prompt,
		Tools:      make([]runTool, 0, len(details.Tools)),
		ToolChoice: details.ToolChoice,
		Limits: runLimits{
			ModelCalls: details.Limits.ModelCalls,
			ToolCalls:  details.Limits.ToolCalls,
			Tokens:     details.Limits.Tokens,
		},
	}
	if operation == agent.OperationMessages {
		input.Questions = details.Questions
	} else if len(details.Questions) > 0 {
		input.Question = details.Questions[0]
	}
	if deadline, ok := ctx.Deadline(); ok {
		input.Deadline = deadline.UTC().Format(time.RFC3339Nano)
	}
	if info := details.Model; info != nil {
		input.Model = &runModel{
			Provider:     info.Provider,
			Name:         info.Name,
			Endpoint:     info.Endpoint,
			TimeoutMS:    info.Timeout.Milliseconds(),
			MaxTokens:    info.MaxTokens,
			ThinkingType: info.ThinkingType,
			Stream:       info.Stream,
		}
	}
	for _, tool := range details.Tools {
		input.Tools = append(input.Tools, runTool{
			Type: tool.Type,
			Function: runFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  previewJSON(string(tool.Function.Parameters)),
			},
		})
	}
	// 预览只包含白名单标量及已校验的 JSON；无效协议片段按文本记录，不影响业务。
	request, _ := json.Marshal(input)
	t.Payload(span, "app.request", request)
	return ctx, &runObservation{
		telemetry: t,
		span:      span,
		started:   time.Now(),
	}
}

// 以下类型只定义观测展示，不承担领域或 HTTP 序列化契约。
type runInput struct {
	Question   string    `json:"question,omitempty"`
	Questions  []string  `json:"questions,omitempty"`
	Prompt     string    `json:"prompt"`
	Model      *runModel `json:"model"`
	Tools      []runTool `json:"tools"`
	ToolChoice string    `json:"tool_choice,omitempty"`
	Limits     runLimits `json:"limits"`
	Deadline   string    `json:"deadline,omitempty"`
}

type runModel struct {
	Provider     string `json:"provider"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	TimeoutMS    int64  `json:"timeout_ms"`
	MaxTokens    int    `json:"max_tokens"`
	ThinkingType string `json:"thinking_type"`
	Stream       bool   `json:"stream"`
}

type runLimits struct {
	ModelCalls int `json:"model_calls"`
	ToolCalls  int `json:"tool_calls"`
	Tokens     int `json:"tokens"`
}

type runTool struct {
	Type     string      `json:"type"`
	Function runFunction `json:"function"`
}

type runFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type runOutput struct {
	Operation     string    `json:"operation"`
	Completed     bool      `json:"completed"`
	Answer        string    `json:"answer"`
	ElapsedMS     int64     `json:"elapsed_ms"`
	ModelCalls    int       `json:"model_calls"`
	ToolCalls     int       `json:"tool_calls"`
	KnownTokens   int       `json:"known_tokens"`
	UsageComplete bool      `json:"usage_complete"`
	Steps         []runStep `json:"steps"`
}

type runStep struct {
	Kind         string       `json:"kind"`
	Name         string       `json:"name"`
	Request      any          `json:"request"`
	Response     any          `json:"response"`
	ElapsedMS    int64        `json:"elapsed_ms"`
	FinishReason string       `json:"finish_reason,omitempty"`
	Usage        *agent.Usage `json:"usage,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// previewJSON 保留有效 JSON 的层级与数值；截断、空值和无效 JSON 保留原文。
func previewJSON(text string) any {
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	return text
}

type runObservation struct {
	telemetry *Telemetry
	span      trace.Span
	started   time.Time
}

func (r *runObservation) TraceID() string {
	if !r.span.SpanContext().IsValid() {
		return ""
	}
	return r.span.SpanContext().TraceID().String()
}

func (r *runObservation) End(result agent.ExecutionResult, err error) {
	output := runOutput{
		Operation:     result.Operation,
		Completed:     result.Completed,
		Answer:        result.Answer,
		ElapsedMS:     result.ElapsedMS,
		ModelCalls:    result.ModelCalls,
		ToolCalls:     result.ToolCalls,
		KnownTokens:   result.KnownTokens,
		UsageComplete: result.UsageComplete,
		Steps:         make([]runStep, 0, len(result.Steps)),
	}
	for _, step := range result.Steps {
		output.Steps = append(output.Steps, runStep{
			Kind:         step.Kind,
			Name:         step.Name,
			Request:      previewJSON(step.Request),
			Response:     previewJSON(step.Response),
			ElapsedMS:    step.ElapsedMS,
			FinishReason: step.FinishReason,
			Usage:        step.Usage,
			Error:        step.Error,
		})
	}
	response, _ := json.Marshal(output)
	r.telemetry.Payload(r.span, "app.response", response)
	r.span.SetAttributes(attribute.Int("app.llm_calls", result.ModelCalls), attribute.Int("app.tool_calls", result.ToolCalls), attribute.Int("app.total_tokens", result.KnownTokens), attribute.Bool("app.usage.complete", result.UsageComplete))
	End(r.span, r.started, err)
}

// StartTool 沿用父链路记录工具请求，调用方须在执行结束后调用返回句柄的 End。
func (t *Telemetry) StartTool(ctx context.Context, call agent.ToolCall) (context.Context, agent.ToolObservation) {
	ctx, span := t.Start(ctx, "Tool call", attribute.String("gen_ai.operation.name", "execute_tool"), attribute.String("app.tool.name", t.Clean(call.Function.Name)), attribute.String("app.tool.call.id", t.Clean(call.ID)))
	// ToolCall 仅含字符串字段，不存在 JSON 编码失败的值。
	request, _ := json.Marshal(call)
	t.Payload(span, "app.request", request)
	return ctx, &toolObservation{
		telemetry: t,
		span:      span,
		started:   time.Now(),
	}
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
