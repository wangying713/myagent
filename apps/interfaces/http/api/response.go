package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/interfaces/http/api/agent/v1"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

func writeResult(c *gin.Context, result agent.ExecutionResult, err error) {
	status, failure := classify(err)
	httpx.WriteResult(c, status, resultJSON(resultMessage(result)), failure)
}

func writeRequestError(c *gin.Context, err error) {
	httpx.WriteRequestError(c, err, resultJSON(&agentv1.ExecutionResult{}))
}

// resultMessage 将领域结果转换为接口消息，领域层不依赖生成代码。
func resultMessage(result agent.ExecutionResult) *agentv1.ExecutionResult {
	data := &agentv1.ExecutionResult{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TraceId: result.TraceID, TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete,
	}
	if result.Steps != nil {
		data.Steps = make([]*agentv1.ExecutionStep, 0, len(result.Steps))
	}
	for _, step := range result.Steps {
		var usage *agentv1.Usage
		if step.Usage != nil {
			usage = &agentv1.Usage{
				PromptTokens:     tokenValue(step.Usage.PromptTokens),
				CompletionTokens: tokenValue(step.Usage.CompletionTokens),
				TotalTokens:      tokenValue(step.Usage.TotalTokens),
			}
		}
		data.Steps = append(data.Steps, &agentv1.ExecutionStep{
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
