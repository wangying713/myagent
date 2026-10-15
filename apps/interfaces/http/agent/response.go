package agentapi

import (
	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
)

// resultMessage 将领域结果转换为接口消息，领域层不依赖生成代码。
func resultMessage(result agent.ExecutionResult) *httpproto.ExecutionResult {
	data := &httpproto.ExecutionResult{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TraceId: result.TraceID, TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete,
	}
	if result.Steps != nil {
		data.Steps = make([]*httpproto.ExecutionStep, 0, len(result.Steps))
	}
	for _, step := range result.Steps {
		var usage *httpproto.Usage
		if step.Usage != nil {
			usage = &httpproto.Usage{
				PromptTokens:     tokenValue(step.Usage.PromptTokens),
				CompletionTokens: tokenValue(step.Usage.CompletionTokens),
				TotalTokens:      tokenValue(step.Usage.TotalTokens),
			}
		}
		data.Steps = append(data.Steps, &httpproto.ExecutionStep{
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
