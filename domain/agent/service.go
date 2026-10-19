package agent

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInput 表示问题内容或固定两轮消息的数量不符合业务要求。
var ErrInput = errors.New("Agent 输入无效")

const (
	OperationChat     = "chat"     // 单轮聊天，沿用接口与观测中的既有取值。
	OperationMessages = "messages" // 固定两轮的消息历史调用。
	OperationRun      = "agent"    // 工具循环，保留既有协议值 agent。
)

const (
	StepKindModel = "model" // 执行记录中的模型调用步骤。
	StepKindTool  = "tool"  // 执行记录中的工具调用步骤。
)

// MaxQuestionChars 是单个问题允许的最大 Unicode 字符数。
const MaxQuestionChars = 4096

// Observer 由业务使用方定义，具体观测实现由入口注入。
// Safe 和 Clean 只处理展示副本，不能修改实际模型消息或工具结果。
// 同一实例可被多个 Service 调用并发使用，实现须保证并发安全。
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
	Questions  []string         // 已补齐默认值并通过校验的问题列表。
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
	End(ExecutionResult, error)
}

// ToolObservation 保存一次工具调用的观测句柄，由该次调用负责结束。
type ToolObservation interface {
	// End 记录结果和错误并结束观测，每次 StartTool 对应一次调用。
	End(ToolResult, error)
}

// ExecutionResult 保存本次业务执行的结果，不承担 HTTP 序列化契约。
type ExecutionResult struct {
	Operation        string          // 本次业务操作标识。
	Completed        bool            // 本次业务是否成功完成。
	Answer           string          // 脱敏后的回答，失败时保留已有内容。
	TraceID          string          // 本次运行的链路标识，关闭观测时为空。
	TelemetryEnabled bool            // 是否启用链路观测。
	ElapsedMS        int64           // 本次运行耗时，单位为毫秒。
	ModelCalls       int             // 已发起的模型调用次数。
	ToolCalls        int             // 已发起的工具调用次数。
	KnownTokens      int             // 可确认的累计 token 用量。
	UsageComplete    bool            // 所有模型调用的用量是否均已确认并累计。
	Steps            []ExecutionStep // 按执行顺序保存的模型与工具调用记录。
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

// Service 的依赖在构造后保持不变，每次调用单独保存历史和执行记录，可并发使用。
type Service struct {
	model    ChatClient // 支持取消和并发调用的模型客户端。
	observer Observer   // 支持并发使用的观测与脱敏接口。
	limits   Limits     // 构造时校验并固定的单次运行预算。
}

// NewService 校验非 nil 依赖及正数预算，创建可并发调用的业务服务。
// model 和 observer 由调用方注入并管理生命周期，须支持并发调用。
func NewService(model ChatClient, observer Observer, limits Limits) (*Service, error) {
	if model == nil || observer == nil || limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return nil, errors.New("Agent 依赖或运行预算无效")
	}
	return &Service{
		model:    model,
		observer: observer,
		limits:   limits,
	}, nil
}

// validateQuestion 拒绝纯空白或超过 MaxQuestionChars 个 Unicode 码点的问题。
// 校验通过返回 nil，否则返回 ErrInput；原问题内容保持不变。
func validateQuestion(question string) error {
	// TrimSpace 只用于判断是否为空；按码点计数，使中文等多字节字符按一个字符计算。
	if strings.TrimSpace(question) == "" || utf8.RuneCountInString(question) > MaxQuestionChars {
		return ErrInput
	}
	return nil
}
