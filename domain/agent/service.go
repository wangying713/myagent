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
	// OperationChat 标识单轮聊天，沿用接口与观测中的既有取值。
	OperationChat = "chat"
	// OperationMessages 标识固定两轮的消息历史调用。
	OperationMessages = "messages"
	// OperationRun 标识工具循环，保留既有协议值 agent。
	OperationRun = "agent"
)

const (
	// StepKindModel 标识执行记录中的模型调用步骤。
	StepKindModel = "model"
	// StepKindTool 标识执行记录中的工具调用步骤。
	StepKindTool = "tool"
)

// MaxQuestionChars 是单个问题允许的最大 Unicode 字符数。
const MaxQuestionChars = 4096

// Observer 由业务使用方定义，具体观测实现由入口注入。
// Safe 和 Clean 只处理展示副本，不能修改实际模型消息或工具结果。
// 同一实例可被多个 Service 调用并发使用，实现须保证并发安全。
type Observer interface {
	Enabled() bool
	Safe([]byte) string
	Clean(string) string
	// StartRun 接收实际运行输入的只读快照，须保留父上下文的取消和链路。
	StartRun(context.Context, string, RunDetails) (context.Context, RunObservation)
	// StartTool 返回工具观测及其上下文，须保留父上下文的取消和链路。
	StartTool(context.Context, ToolCall) (context.Context, ToolObservation)
}

// RunDetails 保存可观测的运行输入，不保存客户端、上下文对象或凭据。
type RunDetails struct {
	Questions  []string
	Prompt     string
	Tools      []ToolDefinition
	ToolChoice string
	Limits     Limits
	Model      *ModelInfo
}

// ModelInfo 只描述客户端的公开配置，禁止放入 API Key 或请求头。
type ModelInfo struct {
	Provider     string
	Name         string
	Endpoint     string
	Timeout      time.Duration
	MaxTokens    int
	ThinkingType string
	Stream       bool
}

// ModelInfoProvider 是运行观测可选使用的模型描述能力，不改变聊天接口。
type ModelInfoProvider interface {
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
	// End 记录工具结果和错误并结束观测，每次 StartTool 对应一次调用。
	End(ToolResult, error)
}

// ExecutionResult 保存本次业务执行的结果，不承担 HTTP 序列化契约。
type ExecutionResult struct {
	Operation        string
	Completed        bool
	Answer           string
	TraceID          string
	TelemetryEnabled bool
	ElapsedMS        int64
	ModelCalls       int
	ToolCalls        int
	KnownTokens      int
	UsageComplete    bool
	Steps            []ExecutionStep
}

// ExecutionStep 保存一次模型或工具调用的耗时、脱敏正文和可用的用量。
type ExecutionStep struct {
	Kind         string
	Name         string
	Request      string
	Response     string
	ElapsedMS    int64
	FinishReason string
	Usage        *Usage
	Error        string
}

// Service 的依赖在构造后保持不变，每次调用单独保存历史和执行记录，可并发使用。
type Service struct {
	model    ChatClient
	observer Observer
	limits   Limits
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

func validateQuestion(question string) error {
	if strings.TrimSpace(question) == "" || utf8.RuneCountInString(question) > MaxQuestionChars {
		return ErrInput
	}
	return nil
}

func (s *Service) newResult(operation string) ExecutionResult {
	return ExecutionResult{
		Operation:        operation,
		UsageComplete:    true,
		Steps:            []ExecutionStep{},
		TelemetryEnabled: s.observer.Enabled(),
	}
}

// execution 持有单次运行的可变结果与观测句柄，供该次业务调用独占使用。
type execution struct {
	service *Service
	result  *ExecutionResult
	model   *recordingModel
	run     RunObservation
	started time.Time
}

func (s *Service) begin(ctx context.Context, result *ExecutionResult, input RunDetails) (context.Context, *execution) {
	input.Limits = s.limits
	if model, ok := s.model.(ModelInfoProvider); ok {
		info := model.ModelInfo()
		input.Model = &info
	}
	ctx, run := s.observer.StartRun(ctx, result.Operation, input)
	result.TraceID = run.TraceID()
	e := &execution{
		service: s,
		result:  result,
		run:     run,
		started: time.Now(),
	}
	e.model = &recordingModel{
		inner:    s.model,
		observer: s.observer,
		result:   result,
		limits:   s.limits,
	}
	return ctx, e
}

func (e *execution) end(err error) {
	e.result.ElapsedMS = time.Since(e.started).Milliseconds()
	e.result.Completed = err == nil
	e.result.Answer = e.service.observer.Clean(e.result.Answer)
	e.run.End(*e.result, err)
}
