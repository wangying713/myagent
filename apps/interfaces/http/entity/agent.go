// Package entity 定义 HTTP 接口共用的数据结构，不承担业务处理或传输行为。
package entity

// AgentExecutionResult 是三个 Agent 接口共用的 HTTP JSON 响应数据。
type AgentExecutionResult struct {
	Lesson           string               `json:"lesson"`
	Completed        bool                 `json:"completed"`
	Answer           string               `json:"answer"`
	TraceID          string               `json:"trace_id"`
	TelemetryEnabled bool                 `json:"telemetry_enabled"`
	ElapsedMS        int64                `json:"elapsed_ms"`
	ModelCalls       int64                `json:"model_calls"`
	ToolCalls        int64                `json:"tool_calls"`
	KnownTokens      int64                `json:"known_tokens"`
	UsageComplete    bool                 `json:"usage_complete"`
	Steps            []AgentExecutionStep `json:"steps"`
}

// AgentUsage 用 nil 表示未知用量，保留已知为零的数值。
type AgentUsage struct {
	PromptTokens     *int64 `json:"prompt_tokens"`
	CompletionTokens *int64 `json:"completion_tokens"`
	TotalTokens      *int64 `json:"total_tokens"`
}

// AgentExecutionStep 是一次模型或工具执行的 HTTP JSON 记录。
type AgentExecutionStep struct {
	Kind         string      `json:"kind"`
	Name         string      `json:"name"`
	Request      string      `json:"request"`
	Response     string      `json:"response"`
	ElapsedMS    int64       `json:"elapsed_ms"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Usage        *AgentUsage `json:"usage,omitempty"`
	Error        string      `json:"error,omitempty"`
}
