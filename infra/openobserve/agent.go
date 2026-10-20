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

// StartRun 建立业务运行 span，应用层在执行结束后提交实际输入和结果。
func (t *Telemetry) StartRun(ctx context.Context, operation string, model *ModelInfo) (context.Context, *Run) {
	ctx, span := t.Start(ctx, "agent.run", attribute.String("app.lesson", operation))
	run := &Run{
		telemetry: t,
		span:      span,
		started:   time.Now(),
		operation: operation,
		model:     model,
	}
	if deadline, ok := ctx.Deadline(); ok {
		run.deadline = deadline.UTC().Format(time.RFC3339Nano)
	}
	return ctx, run
}

// recordInput 记录业务实际使用的输入，不在观测层重新推断默认值。
func (r *Run) recordInput(req agent.RunInput) {
	input := runInput{
		Prompt:     req.Prompt,
		Tools:      make([]runTool, 0, len(req.Tools)),
		ToolChoice: req.ToolChoice,
		Limits: runLimits{
			ModelCalls: req.Limits.ModelCalls,
			ToolCalls:  req.Limits.ToolCalls,
			Tokens:     req.Limits.Tokens,
		},
	}
	if r.operation == agent.OperationMessages {
		input.Questions = req.Questions
	} else if len(req.Questions) > 0 {
		input.Question = req.Questions[0]
	}
	input.Deadline = r.deadline
	if info := r.model; info != nil {
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
	for _, tool := range req.Tools {
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
	r.telemetry.Payload(r.span, "app.request", request)
}

// runInput 保存业务输入的观测预览，不承担领域或 HTTP 序列化契约。
type runInput struct {
	Question   string    `json:"question,omitempty"`    // 聊天或工具循环的单个输入问题。
	Questions  []string  `json:"questions,omitempty"`   // 消息历史业务的两轮输入问题。
	Prompt     string    `json:"prompt"`                // 本次运行的系统提示词。
	Model      *runModel `json:"model"`                 // 可选的模型公开配置预览。
	Tools      []runTool `json:"tools"`                 // 可供模型选择的工具定义预览。
	ToolChoice string    `json:"tool_choice,omitempty"` // 模型选择工具的策略。
	Limits     runLimits `json:"limits"`                // 本次运行的预算预览。
	// Deadline 是业务上下文的 UTC 截止时间，格式为 RFC3339Nano。
	Deadline string `json:"deadline,omitempty"`
}

// runModel 保存模型公开配置的观测预览。
type runModel struct {
	Provider     string `json:"provider"`      // 模型服务提供方标识。
	Name         string `json:"name"`          // 实际请求的模型名称。
	Endpoint     string `json:"endpoint"`      // 模型请求地址，不包含凭据。
	TimeoutMS    int64  `json:"timeout_ms"`    // 单次模型调用超时，单位为毫秒。
	MaxTokens    int    `json:"max_tokens"`    // 单次模型请求的最大输出 token 数。
	ThinkingType string `json:"thinking_type"` // 模型思考模式。
	Stream       bool   `json:"stream"`        // 是否使用流式响应。
}

// runLimits 保存单次业务运行的预算预览。
type runLimits struct {
	ModelCalls int `json:"model_calls"` // 单次运行允许的最大模型调用次数。
	ToolCalls  int `json:"tool_calls"`  // 单次运行允许的最大工具调用次数。
	Tokens     int `json:"tokens"`      // 单次运行的累计已知 token 软预算。
}

// runTool 保存提供给模型的工具协议预览。
type runTool struct {
	Type     string      `json:"type"`     // 工具协议类型。
	Function runFunction `json:"function"` // 供模型选择的函数定义预览。
}

// runFunction 保存工具函数说明和参数约束的观测预览。
type runFunction struct {
	Name        string `json:"name"`        // 已注册的工具函数名称。
	Description string `json:"description"` // 供模型选择工具时阅读的能力说明。
	// Parameters 展开参数 JSON Schema，无效 JSON 保留为文本。
	Parameters any `json:"parameters"`
}

// runOutput 保存业务执行结果的观测预览。
type runOutput struct {
	Operation     string          `json:"operation"`      // 本次业务操作标识。
	Completed     bool            `json:"completed"`      // 本次业务是否成功完成。
	Answer        string          `json:"answer"`         // 脱敏后的回答，失败时保留已有内容。
	ElapsedMS     int64           `json:"elapsed_ms"`     // 本次运行耗时，单位为毫秒。
	ModelCalls    int             `json:"model_calls"`    // 已发起的模型调用次数。
	ToolCalls     int             `json:"tool_calls"`     // 已发起的工具调用次数。
	KnownTokens   int             `json:"known_tokens"`   // 可确认的累计 token 用量。
	UsageComplete bool            `json:"usage_complete"` // 所有模型调用的用量是否均已确认并累计。
	Calls         []runCallRecord `json:"steps"`          // 按执行顺序保存的模型与工具调用记录。
}

// runCallRecord 保存单次模型或工具调用的观测预览。
type runCallRecord struct {
	Kind         string       `json:"kind"`                    // 调用类型，区分模型调用与工具调用。
	Name         string       `json:"name"`                    // 模型调用说明或工具名称。
	Request      any          `json:"request"`                 // 展开的请求 JSON，无效或已截断的正文保留为文本。
	Response     any          `json:"response"`                // 展开的响应 JSON，无效或已截断的正文保留为文本。
	ElapsedMS    int64        `json:"elapsed_ms"`              // 本次调用耗时，单位为毫秒。
	FinishReason string       `json:"finish_reason,omitempty"` // 模型结束原因，工具调用时为空。
	Usage        *agent.Usage `json:"usage,omitempty"`         // 模型返回的 token 用量，工具调用时为 nil。
	Error        string       `json:"error,omitempty"`         // 执行失败或工具拒绝的公开说明。
}

// toolInput 是工具调用的观测展示形态，平铺名称和调用 ID 并展开参数，不承担领域协议契约。
type toolInput struct {
	ID        string `json:"id"`        // 用于关联工具结果的调用标识。
	Type      string `json:"type"`      // 工具调用的协议类型。
	Name      string `json:"name"`      // 待执行的工具函数名称。
	Arguments any    `json:"arguments"` // 展开的 JSON 参数，无效 JSON 保留为文本。
}

// previewJSON 保留有效 JSON 的层级与数值；截断、空值和无效 JSON 保留原文。
func previewJSON(text string) any {
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	return text
}

// Run 持有单次业务运行的观测资源，由应用层负责结束。
type Run struct {
	telemetry *Telemetry // 负责记录正文与脱敏的观测组件。
	span      trace.Span // 本次调用的链路 span。
	started   time.Time  // 本次调用的开始时间，用于统计耗时。
	operation string     // 本次业务操作标识。
	model     *ModelInfo // 模型公开配置；nil 表示未提供。
	deadline  string     // 本次运行的 UTC 截止时间，无截止时间时为空。
}

// End 记录实际输入、完整或部分结果和最终错误，并结束本次运行。
func (r *Run) End(result agent.ChatRsp, err error) {
	r.recordInput(result.Input)
	output := runOutput{
		Operation:     result.Operation,
		Completed:     result.Completed,
		Answer:        result.Answer,
		ElapsedMS:     result.ElapsedMS,
		ModelCalls:    result.ModelCalls,
		ToolCalls:     result.ToolCalls,
		KnownTokens:   result.KnownTokens,
		UsageComplete: result.UsageComplete,
		Calls:         make([]runCallRecord, 0, len(result.Calls)),
	}
	for _, record := range result.Calls {
		output.Calls = append(output.Calls, runCallRecord{
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
	response, _ := json.Marshal(output)
	r.telemetry.Payload(r.span, "app.response", response)
	r.span.SetAttributes(attribute.Int("app.llm_calls", result.ModelCalls), attribute.Int("app.tool_calls", result.ToolCalls), attribute.Int("app.total_tokens", result.KnownTokens), attribute.Bool("app.usage.complete", result.UsageComplete))
	End(r.span, r.started, err)
}

// StartTool 沿用父链路记录工具请求，调用方须在执行结束后调用返回句柄的 End。
func (t *Telemetry) StartTool(ctx context.Context, req agent.ToolCall) (context.Context, *toolObservation) {
	ctx, span := t.Start(ctx, "Tool call", attribute.String("gen_ai.operation.name", "execute_tool"), attribute.String("app.tool.name", t.Clean(req.Function.Name)), attribute.String("app.tool.call.id", t.Clean(req.ID)))
	// 展示副本平铺工具调用并展开参数；previewJSON 只在参数为合法 JSON 时返回 RawMessage，因此不会编码失败。
	request, _ := json.Marshal(toolInput{
		ID:        req.ID,
		Type:      req.Type,
		Name:      req.Function.Name,
		Arguments: previewJSON(req.Function.Arguments),
	})
	t.Payload(span, "app.request", request)
	return ctx, &toolObservation{
		telemetry: t,
		span:      span,
		started:   time.Now(),
	}
}

// toolObservation 持有单次工具调用的观测资源。
type toolObservation struct {
	telemetry *Telemetry // 负责记录正文与脱敏的观测组件。
	span      trace.Span // 本次调用的链路 span。
	started   time.Time  // 本次调用的开始时间，用于统计耗时。
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
