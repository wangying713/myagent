// Package entity 定义手写的 HTTP 共用数据与 JSON 表现类型，不承担业务处理或传输行为。
package entity

// AgentExecutionResult 是三个 Agent 接口共用的 HTTP JSON 响应数据。
// 整数输出为 JSON 数字；Steps 为 nil 和空切片时分别输出 null 和空数组。
type AgentExecutionResult struct {
	Lesson           string               `json:"lesson"`            // 业务操作标识，对应 chat、messages 或 agent。
	Completed        bool                 `json:"completed"`         // 本次业务是否成功完成。
	Answer           string               `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool                 `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64                `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int64                `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int64                `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int64                `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool                 `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	Steps            []AgentExecutionStep `json:"steps"`             // 按执行顺序保存的模型与工具调用记录。
}

// AgentUsage 用 nil 表示未知用量，保留已知为零的数值。
type AgentUsage struct {
	PromptTokens     *int64 `json:"prompt_tokens"`     // 输入 token 数，nil 表示未提供。
	CompletionTokens *int64 `json:"completion_tokens"` // 输出 token 数，nil 表示未提供。
	TotalTokens      *int64 `json:"total_tokens"`      // 总 token 数，nil 表示未提供。
}

// AgentExecutionStep 是一次模型或工具执行的 HTTP JSON 记录。
type AgentExecutionStep struct {
	Kind         string      `json:"kind"`                    // 步骤类型，区分模型调用与工具调用。
	Name         string      `json:"name"`                    // 模型调用说明或工具名称。
	Request      string      `json:"request"`                 // 脱敏并限长后的请求正文。
	Response     string      `json:"response"`                // 脱敏并限长后的响应正文。
	ElapsedMS    int64       `json:"elapsed_ms"`              // 本步骤耗时，单位为毫秒。
	FinishReason string      `json:"finish_reason,omitempty"` // 模型结束原因，工具步骤为空。
	Usage        *AgentUsage `json:"usage,omitempty"`         // 模型返回的 token 用量，工具步骤为 nil。
	Error        string      `json:"error,omitempty"`         // 执行失败或工具拒绝的公开说明。
}
