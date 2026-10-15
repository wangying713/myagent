package agent

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInput = errors.New("Agent 输入无效")

// Observer 由业务使用方定义，具体观测实现由入口注入。
// Safe 和 Clean 只处理展示副本，不能修改实际模型消息或工具结果。
type Observer interface {
	Enabled() bool
	Safe([]byte) string
	Clean(string) string
	StartRun(context.Context, string) (context.Context, RunObservation)
	StartTool(context.Context, ToolCall) (context.Context, ToolObservation)
}

type RunObservation interface {
	TraceID() string
	End(ExecutionResult, error)
}

type ToolObservation interface {
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

func NewService(model ChatClient, observer Observer, limits Limits) (*Service, error) {
	if model == nil || observer == nil || limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return nil, errors.New("Agent 依赖或运行预算无效")
	}
	return &Service{model: model, observer: observer, limits: limits}, nil
}

func validateQuestion(question string) error {
	if strings.TrimSpace(question) == "" || utf8.RuneCountInString(question) > 4096 {
		return ErrInput
	}
	return nil
}

func (s *Service) newResult(operation string) ExecutionResult {
	return ExecutionResult{Operation: operation, UsageComplete: true, Steps: []ExecutionStep{}, TelemetryEnabled: s.observer.Enabled()}
}

type execution struct {
	service *Service
	result  *ExecutionResult
	model   *recordingModel
	run     RunObservation
	started time.Time
}

func (s *Service) begin(ctx context.Context, result *ExecutionResult) (context.Context, *execution) {
	ctx, run := s.observer.StartRun(ctx, result.Operation)
	result.TraceID = run.TraceID()
	e := &execution{service: s, result: result, run: run, started: time.Now()}
	e.model = &recordingModel{inner: s.model, observer: s.observer, result: result, limits: s.limits}
	return ctx, e
}

func (e *execution) end(err error) {
	e.result.ElapsedMS = time.Since(e.started).Milliseconds()
	e.result.Completed = err == nil
	e.result.Answer = e.service.observer.Clean(e.result.Answer)
	e.run.End(*e.result, err)
}
