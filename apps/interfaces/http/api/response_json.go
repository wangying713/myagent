package api

import "sakelei/ai/apps/interfaces/http/api/agent/v1"

// 以下私有类型只适配现有 HTTP JSON 格式，不作为另一套业务消息使用。
// 不直接编码生成结构体，也不直接套用 ProtoJSON：管理台需要数字整数、
// 必定出现的零值字段、未知用量的 null，以及 steps 的 null/[] 区别。
type executionResultJSON struct {
	Lesson           string              `json:"lesson"`
	Completed        bool                `json:"completed"`
	Answer           string              `json:"answer"`
	TraceID          string              `json:"trace_id"`
	TelemetryEnabled bool                `json:"telemetry_enabled"`
	ElapsedMS        int64               `json:"elapsed_ms"`
	ModelCalls       int64               `json:"model_calls"`
	ToolCalls        int64               `json:"tool_calls"`
	KnownTokens      int64               `json:"known_tokens"`
	UsageComplete    bool                `json:"usage_complete"`
	Steps            []executionStepJSON `json:"steps"`
}

type usageJSON struct {
	PromptTokens     *int64 `json:"prompt_tokens"`
	CompletionTokens *int64 `json:"completion_tokens"`
	TotalTokens      *int64 `json:"total_tokens"`
}

type executionStepJSON struct {
	Kind         string     `json:"kind"`
	Name         string     `json:"name"`
	Request      string     `json:"request"`
	Response     string     `json:"response"`
	ElapsedMS    int64      `json:"elapsed_ms"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Usage        *usageJSON `json:"usage,omitempty"`
	Error        string     `json:"error,omitempty"`
}

func resultJSON(message *agentv1.ExecutionResult) executionResultJSON {
	data := executionResultJSON{
		Lesson: message.GetLesson(), Completed: message.GetCompleted(), Answer: message.GetAnswer(),
		TraceID: message.GetTraceId(), TelemetryEnabled: message.GetTelemetryEnabled(), ElapsedMS: message.GetElapsedMs(),
		ModelCalls: message.GetModelCalls(), ToolCalls: message.GetToolCalls(), KnownTokens: message.GetKnownTokens(),
		UsageComplete: message.GetUsageComplete(),
	}
	if message.GetSteps() != nil {
		data.Steps = make([]executionStepJSON, 0, len(message.GetSteps()))
	}
	for _, step := range message.GetSteps() {
		var usage *usageJSON
		if step.GetUsage() != nil {
			usage = &usageJSON{
				PromptTokens:     step.Usage.PromptTokens,
				CompletionTokens: step.Usage.CompletionTokens,
				TotalTokens:      step.Usage.TotalTokens,
			}
		}
		data.Steps = append(data.Steps, executionStepJSON{
			Kind: step.GetKind(), Name: step.GetName(), Request: step.GetRequest(), Response: step.GetResponse(),
			ElapsedMS: step.GetElapsedMs(), FinishReason: step.GetFinishReason(), Usage: usage, Error: step.GetError(),
		})
	}
	return data
}
