package agent

const (
	StepKindModel = "model" // 执行记录中的模型调用步骤。
	StepKindTool  = "tool"  // 执行记录中的工具调用步骤。
)

// ExecutionResult 保存本次业务执行的结果，不承担 HTTP 序列化契约。
type ExecutionResult struct {
	// 执行结果。
	Operation string // 本次业务操作标识。
	Completed bool   // 本次业务是否成功完成。
	Answer    string // 脱敏后的回答，失败时保留已有内容。

	// 运行统计。
	ElapsedMS     int64 // 本次运行耗时，单位为毫秒。
	ModelCalls    int   // 已发起的模型调用次数。
	ToolCalls     int   // 已发起的工具调用次数。
	KnownTokens   int   // 可确认的累计 token 用量。
	UsageComplete bool  // 所有模型调用的用量是否均已确认并累计。

	// 执行过程。
	Steps []ExecutionStep // 按执行顺序保存的模型与工具调用记录。

	// 链路信息。
	TraceID          string // 本次运行的链路标识，关闭观测时为空。
	TelemetryEnabled bool   // 是否启用链路观测。
}

// NewExecutionResult 创建本次运行的初始结果，操作标识和是否开启观测由调用方传入。
func NewExecutionResult(operation string, telemetryEnabled bool) ExecutionResult {
	return ExecutionResult{
		Operation: operation,
		// 尚未调用模型时没有缺失用量；后续无法确认或累计用量时改为 false。
		UsageComplete: true,
		// 使用非 nil 空切片，使尚无步骤的结果在 HTTP JSON 中表现为 []。
		Steps:            []ExecutionStep{},
		TelemetryEnabled: telemetryEnabled,
	}
}

// ExecutionStep 保存一次模型或工具调用的耗时、脱敏正文和可用的用量。
type ExecutionStep struct {
	Kind         string // 步骤类型，区分模型调用与工具调用。
	Name         string // 模型调用说明或工具名称。
	Request      string // 脱敏并限长后的请求正文。
	Response     string // 脱敏并限长后的响应正文。
	ElapsedMS    int64  // 本步骤耗时，单位为毫秒。
	FinishReason string // 模型结束原因，工具步骤为空。
	Usage        *Usage // 模型返回的 token 用量，工具步骤为 nil。
	Error        string // 执行失败或工具拒绝的公开说明。
}
