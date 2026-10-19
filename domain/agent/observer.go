package agent

import (
	"context"
	"time"
)

// Observer 由业务使用方定义，具体观测实现由入口注入。
// Safe 和 Clean 只处理展示副本，不能修改实际模型消息或工具结果。
// 同一实例可被多个 Client 调用并发使用，实现须保证并发安全。
type Observer interface {
	// Enabled 报告是否启用链路观测，用于填写执行结果中的观测状态。
	Enabled() bool
	// Safe 返回脱敏、限长后的正文副本，用于保存模型和工具的执行记录。
	Safe([]byte) string
	// Clean 返回脱敏后的文本副本，用于处理回答、名称和错误说明。
	Clean(string) string
	// StartRun 接收实际运行输入的只读快照，须保留父上下文的取消和链路。
	StartRun(context.Context, string, RunDetails) (context.Context, RunObservation)
	// StartTool 返回工具观测及其上下文，须保留父上下文的取消和链路。
	StartTool(ctx context.Context, req ToolCall) (context.Context, ToolObservation)
}

// RunDetails 保存可观测的运行输入，不保存客户端、上下文对象或凭据。
type RunDetails struct {
	Questions  []string         // 已补齐默认值的问题列表，prepareRun 校验通过后交给观测。
	Prompt     string           // 本次业务使用的系统提示词。
	Tools      []ToolDefinition // 本次运行可用的工具定义。
	ToolChoice string           // 模型选择工具的策略。
	Limits     Limits           // 本次运行的模型、工具与用量预算。
	Model      *ModelInfo       // 可选的模型公开配置，未提供时为 nil。
}

// ModelInfo 只描述客户端的公开配置，禁止放入 API Key 或请求头。
type ModelInfo struct {
	Provider     string        // 模型服务提供方标识。
	Name         string        // 实际请求的模型名称。
	Endpoint     string        // 模型请求地址，不包含凭据。
	Timeout      time.Duration // 单次模型调用的超时时长。
	MaxTokens    int           // 单次模型请求的最大输出 token 数。
	ThinkingType string        // 模型思考模式。
	Stream       bool          // 是否使用流式响应。
}

// ModelInfoProvider 是运行观测可选使用的模型描述能力，不改变聊天接口。
type ModelInfoProvider interface {
	// ModelInfo 返回用于观测展示的模型公开配置，不包含鉴权信息。
	ModelInfo() ModelInfo
}

// RunObservation 保存一次业务执行的观测句柄，由该次执行负责结束。
type RunObservation interface {
	// TraceID 返回本次执行的链路标识；没有链路时返回空字符串。
	TraceID() string
	// End 记录结果和错误并结束观测，每次 StartRun 对应一次调用。
	End(Result, error)
}

// ToolObservation 保存一次工具调用的观测句柄，由该次调用负责结束。
type ToolObservation interface {
	// End 记录结果和错误并结束观测，每次 StartTool 对应一次调用。
	End(ToolResult, error)
}
