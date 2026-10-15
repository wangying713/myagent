package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

// ExecutionResult 是 Agent API 的响应数据；外层公共响应结构由 httpx 提供。
type ExecutionResult struct {
	Lesson           string          `json:"lesson"`
	Completed        bool            `json:"completed"`
	Answer           string          `json:"answer"`
	TraceID          string          `json:"trace_id"`
	TelemetryEnabled bool            `json:"telemetry_enabled"`
	ElapsedMS        int64           `json:"elapsed_ms"`
	ModelCalls       int             `json:"model_calls"`
	ToolCalls        int             `json:"tool_calls"`
	KnownTokens      int             `json:"known_tokens"`
	UsageComplete    bool            `json:"usage_complete"`
	Steps            []ExecutionStep `json:"steps"`
}

type Usage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

type ExecutionStep struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Request      string `json:"request"`
	Response     string `json:"response"`
	ElapsedMS    int64  `json:"elapsed_ms"`
	FinishReason string `json:"finish_reason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
	Error        string `json:"error,omitempty"`
}

func writeResult(c *gin.Context, result agent.ExecutionResult, err error) {
	status, failure := classify(err)
	data := ExecutionResult{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TraceID: result.TraceID, TelemetryEnabled: result.TelemetryEnabled, ElapsedMS: result.ElapsedMS,
		ModelCalls: result.ModelCalls, ToolCalls: result.ToolCalls, KnownTokens: result.KnownTokens,
		UsageComplete: result.UsageComplete,
	}
	if result.Steps != nil {
		data.Steps = make([]ExecutionStep, 0, len(result.Steps))
	}
	for _, step := range result.Steps {
		var usage *Usage
		if step.Usage != nil {
			usage = &Usage{PromptTokens: step.Usage.PromptTokens, CompletionTokens: step.Usage.CompletionTokens, TotalTokens: step.Usage.TotalTokens}
		}
		data.Steps = append(data.Steps, ExecutionStep{Kind: step.Kind, Name: step.Name, Request: step.Request, Response: step.Response, ElapsedMS: step.ElapsedMS, FinishReason: step.FinishReason, Usage: usage, Error: step.Error})
	}
	httpx.WriteResult(c, status, data, failure)
}
