package agentapi

import (
	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/apps/interfaces/http/proto"
)

// resultJSON 将接口消息转换为共用的 HTTP JSON 数据，保留数字、零值、null 和空数组语义。
func resultJSON(message *httpproto.ExecutionResult) entity.AgentExecutionResult {
	data := entity.AgentExecutionResult{
		Lesson: message.GetLesson(), Completed: message.GetCompleted(), Answer: message.GetAnswer(),
		TraceID: message.GetTraceId(), TelemetryEnabled: message.GetTelemetryEnabled(), ElapsedMS: message.GetElapsedMs(),
		ModelCalls: message.GetModelCalls(), ToolCalls: message.GetToolCalls(), KnownTokens: message.GetKnownTokens(),
		UsageComplete: message.GetUsageComplete(),
	}
	if message.GetSteps() != nil {
		data.Steps = make([]entity.AgentExecutionStep, 0, len(message.GetSteps()))
	}
	for _, step := range message.GetSteps() {
		var usage *entity.AgentUsage
		if step.GetUsage() != nil {
			usage = &entity.AgentUsage{
				PromptTokens:     step.Usage.PromptTokens,
				CompletionTokens: step.Usage.CompletionTokens,
				TotalTokens:      step.Usage.TotalTokens,
			}
		}
		data.Steps = append(data.Steps, entity.AgentExecutionStep{
			Kind: step.GetKind(), Name: step.GetName(), Request: step.GetRequest(), Response: step.GetResponse(),
			ElapsedMS: step.GetElapsedMs(), FinishReason: step.GetFinishReason(), Usage: usage, Error: step.GetError(),
		})
	}
	return data
}
