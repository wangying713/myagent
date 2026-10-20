// Package entity 定义手写的 HTTP 请求、响应及共用数据，不承担业务处理或传输行为。
package entity

// ChatReq 是普通聊天请求，省略或空问题使用默认学习示例。
type ChatReq struct {
	Question string `json:"question"` // 用户问题，空字符串使用接口默认问题。
}

// ChatRsp 是普通聊天接口的业务响应，公共外层由 HTTP handler 包装。
type ChatRsp struct {
	Lesson           string               `json:"lesson"`            // 业务操作标识，对应 chat、messages 或 agent。
	Completed        bool                 `json:"completed"`         // 本次业务是否成功完成。
	Answer           string               `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool                 `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64                `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int                  `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int                  `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int                  `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool                 `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	Steps            []AgentExecutionStep `json:"steps"`             // 按执行顺序保存的模型与工具调用记录。
}

// MessagesReq 是两轮消息请求，省略问题列表使用默认示例，显式空数组无效。
type MessagesReq struct {
	Questions []string `json:"questions"` // 两轮问题；nil 表示省略，非 nil 空切片表示显式空数组。
}

// MessagesRsp 是两轮消息接口的业务响应，公共外层由 HTTP handler 包装。
type MessagesRsp struct {
	Lesson           string               `json:"lesson"`            // 业务操作标识，对应 chat、messages 或 agent。
	Completed        bool                 `json:"completed"`         // 本次业务是否成功完成。
	Answer           string               `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool                 `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64                `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int                  `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int                  `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int                  `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool                 `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	Steps            []AgentExecutionStep `json:"steps"`             // 按执行顺序保存的模型与工具调用记录。
}

// RunReq 是受控工具调用请求，省略或空问题使用默认学习示例。
type RunReq struct {
	Question string `json:"question"` // 用户问题，空字符串使用接口默认问题。
}

// RunRsp 是受控工具调用接口的业务响应，公共外层由 HTTP handler 包装。
type RunRsp struct {
	Lesson           string               `json:"lesson"`            // 业务操作标识，对应 chat、messages 或 agent。
	Completed        bool                 `json:"completed"`         // 本次业务是否成功完成。
	Answer           string               `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool                 `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64                `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int                  `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int                  `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int                  `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool                 `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	Steps            []AgentExecutionStep `json:"steps"`             // 按执行顺序保存的模型与工具调用记录。
}

// AgentResult 是 Agent 执行结果。
type AgentResult struct {
	Lesson           string `json:"lesson"`            // 业务操作标识。
	Completed        bool   `json:"completed"`         // 本次业务是否成功完成。
	Answer           string `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool   `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64  `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int    `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int    `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int    `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool   `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	// 按执行顺序保存模型与工具调用记录；nil 输出 null，非 nil 空切片输出空数组。
	Steps []AgentExecutionStep `json:"steps"`
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

// AgentUsage 用 nil 表示未知用量，保留已知为零的数值。
type AgentUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`     // 输入 token 数，nil 表示未提供。
	CompletionTokens *int `json:"completion_tokens"` // 输出 token 数，nil 表示未提供。
	TotalTokens      *int `json:"total_tokens"`      // 总 token 数，nil 表示未提供。
}
