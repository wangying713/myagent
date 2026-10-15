package agentapi

import (
	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

func questionInput(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	// 省略使用业务默认问题，显式空字符串仍应拒绝。
	if *value == "" {
		return "", agent.ErrInput
	}
	return *value, nil
}

// resultMessage 将领域结果转换为接口消息，领域层不依赖生成代码。
func resultMessage(result agent.ExecutionResult) *entity.ExecutionResult {
	data := &entity.ExecutionResult{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TraceId: result.TraceID, TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete,
	}
	if result.Steps != nil {
		data.Steps = make([]*entity.ExecutionStep, 0, len(result.Steps))
	}
	for _, step := range result.Steps {
		var usage *entity.Usage
		if step.Usage != nil {
			usage = &entity.Usage{
				PromptTokens:     tokenValue(step.Usage.PromptTokens),
				CompletionTokens: tokenValue(step.Usage.CompletionTokens),
				TotalTokens:      tokenValue(step.Usage.TotalTokens),
			}
		}
		data.Steps = append(data.Steps, &entity.ExecutionStep{
			Kind: step.Kind, Name: step.Name, Request: step.Request, Response: step.Response,
			ElapsedMs: step.ElapsedMS, FinishReason: step.FinishReason, Usage: usage, Error: step.Error,
		})
	}
	return data
}

func tokenValue(value *int) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}

// resultJSON 将接口消息转换为共用的 HTTP JSON 数据，保留数字、零值、null 和空数组语义。
func resultJSON(message *entity.ExecutionResult) entity.AgentExecutionResult {
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
