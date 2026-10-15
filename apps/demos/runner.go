// Package demos 实现指定的三课流程，向入口返回结构化结果，不负责终端或 HTTP。
package demos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/openobserve"
)

const (
	ChatQuestion   = "用一句话解释什么是递归。"
	FirstQuestion  = "我给一只虚构小猫取名叫豆包，请记住。"
	SecondQuestion = "我给小猫取了什么名字？"
	AgentQuestion  = "每盒有 17 支笔，买 23 盒，一共多少支？"
	AgentPrompt    = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
)

var ErrInput = errors.New("演示输入无效")

type Input struct {
	Question  string   `json:"question,omitempty"`
	Questions []string `json:"questions,omitempty"`
}

type Step struct {
	Kind         string       `json:"kind"`
	Name         string       `json:"name"`
	Request      string       `json:"request"`
	Response     string       `json:"response"`
	ElapsedMS    int64        `json:"elapsed_ms"`
	FinishReason string       `json:"finish_reason,omitempty"`
	Usage        *agent.Usage `json:"usage,omitempty"`
	Error        string       `json:"error,omitempty"`
}

type Result struct {
	Lesson           string `json:"lesson"`
	Completed        bool   `json:"completed"`
	Answer           string `json:"answer"`
	TraceID          string `json:"trace_id"`
	TelemetryEnabled bool   `json:"telemetry_enabled"`
	ElapsedMS        int64  `json:"elapsed_ms"`
	ModelCalls       int    `json:"model_calls"`
	ToolCalls        int    `json:"tool_calls"`
	KnownTokens      int    `json:"known_tokens"`
	UsageComplete    bool   `json:"usage_complete"`
	Steps            []Step `json:"steps"`
}

type Runner struct {
	model     agent.ChatClient
	telemetry *openobserve.Telemetry
	limits    agent.Limits
}

func New(model agent.ChatClient, telemetry *openobserve.Telemetry, limits agent.Limits) (*Runner, error) {
	if model == nil || telemetry == nil || limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return nil, errors.New("演示依赖或运行预算无效")
	}
	return &Runner{model: model, telemetry: telemetry, limits: limits}, nil
}

func normalize(lesson string, input Input) (Input, error) {
	switch lesson {
	case "chat", "agent":
		if input.Questions != nil {
			return input, ErrInput
		}
		if input.Question == "" {
			if lesson == "chat" {
				input.Question = ChatQuestion
			} else {
				input.Question = AgentQuestion
			}
		}
		if strings.TrimSpace(input.Question) == "" || utf8.RuneCountInString(input.Question) > 4096 {
			return input, ErrInput
		}
	case "messages":
		if input.Question != "" {
			return input, ErrInput
		}
		if input.Questions == nil {
			input.Questions = []string{FirstQuestion, SecondQuestion}
		}
		if len(input.Questions) != 2 {
			return input, ErrInput
		}
		for _, q := range input.Questions {
			if strings.TrimSpace(q) == "" || utf8.RuneCountInString(q) > 4096 {
				return input, ErrInput
			}
		}
	default:
		return input, ErrInput
	}
	return input, nil
}

// Run 的统计和记录都在栈上的本次执行中；Runner 可用于并发请求。
func (r *Runner) Run(ctx context.Context, lesson string, input Input) (result Result, err error) {
	result = Result{Lesson: lesson, UsageComplete: true, Steps: []Step{}, TelemetryEnabled: r.telemetry.Enabled()}
	input, err = normalize(lesson, input)
	if err != nil {
		return result, err
	}
	ctx, root := r.telemetry.Start(ctx, "agent.run", attribute.String("app.lesson", lesson))
	if root.SpanContext().IsValid() {
		result.TraceID = root.SpanContext().TraceID().String()
	}
	started := time.Now()
	defer func() {
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Completed = err == nil
		root.SetAttributes(attribute.Int("app.llm_calls", result.ModelCalls), attribute.Int("app.tool_calls", result.ToolCalls), attribute.Int("app.total_tokens", result.KnownTokens), attribute.Bool("app.usage.complete", result.UsageComplete))
		openobserve.End(root, started, err)
	}()
	model := &recordingModel{inner: r.model, telemetry: r.telemetry, result: &result, limits: r.limits}
	if lesson == "agent" {
		tools := &recordingTools{inner: agent.MultiplyTools{}, telemetry: r.telemetry, result: &result}
		result.Answer, err = agent.Run(ctx, model, tools, input.Question, AgentPrompt, r.limits)
		return result, err
	}
	prompt := "你是一个简洁的中文助手。"
	questions := []string{input.Question}
	if lesson == "messages" {
		prompt = "用简短中文回答。"
		questions = input.Questions
	}
	messages := []agent.Message{{Role: "system", Content: prompt}}
	for _, question := range questions {
		messages = append(messages, agent.Message{Role: "user", Content: question})
		response, callErr := model.Chat(ctx, agent.Request{Messages: messages})
		if callErr != nil {
			return result, callErr
		}
		if response.FinishReason != "stop" {
			return result, agent.ErrInvalidResponse
		}
		// 必须保存实际 assistant 消息，不能用第二个问题替代真实历史。
		messages = append(messages, response.Message)
		result.Answer = response.Message.Content
	}
	return result, nil
}

type recordingModel struct {
	inner     agent.ChatClient
	telemetry *openobserve.Telemetry
	result    *Result
	limits    agent.Limits
}

func (m *recordingModel) Chat(ctx context.Context, input agent.Request) (agent.Response, error) {
	if err := ctx.Err(); err != nil {
		return agent.Response{}, err
	}
	if m.result.ModelCalls >= m.limits.ModelCalls || m.result.KnownTokens >= m.limits.Tokens {
		return agent.Response{}, agent.ErrBudgetExceeded
	}
	started := time.Now()
	m.result.ModelCalls++
	response, err := m.inner.Chat(ctx, input)
	if err == nil {
		err = response.Validate()
	}
	if total, known := response.Usage.Total(); known && total <= math.MaxInt-m.result.KnownTokens {
		m.result.KnownTokens += total
	} else {
		m.result.UsageComplete = false
	}
	request := response.RequestJSON
	if len(request) == 0 {
		request, _ = json.Marshal(input)
	}
	step := Step{Kind: "model", Name: fmt.Sprintf("第 %d 次模型请求", m.result.ModelCalls), Request: m.telemetry.Safe(request), Response: m.telemetry.Safe(response.Raw), ElapsedMS: time.Since(started).Milliseconds(), FinishReason: response.FinishReason, Usage: &response.Usage}
	if err != nil {
		step.Error = "模型调用未完成"
	}
	m.result.Steps = append(m.result.Steps, step)
	return response, err
}

type recordingTools struct {
	inner     agent.Tools
	telemetry *openobserve.Telemetry
	result    *Result
}

func (t *recordingTools) Definitions() []agent.ToolDefinition { return t.inner.Definitions() }
func (t *recordingTools) Dispatch(ctx context.Context, call agent.ToolCall) (output agent.ToolResult, err error) {
	if err := ctx.Err(); err != nil {
		return output, err
	}
	ctx, span := t.telemetry.Start(ctx, "Tool call", attribute.String("gen_ai.operation.name", "execute_tool"), attribute.String("app.tool.name", t.telemetry.Clean(call.Function.Name)), attribute.String("app.tool.call.id", t.telemetry.Clean(call.ID)))
	started := time.Now()
	defer func() { openobserve.End(span, started, err) }()
	t.result.ToolCalls++
	request, _ := json.Marshal(call)
	t.telemetry.Payload(span, "app.request", request)
	output, err = t.inner.Dispatch(ctx, call)
	response, _ := json.Marshal(output)
	t.telemetry.Payload(span, "app.response", response)
	step := Step{Kind: "tool", Name: call.Function.Name, Request: t.telemetry.Safe(request), Response: t.telemetry.Safe(response), ElapsedMS: time.Since(started).Milliseconds()}
	if !output.OK {
		step.Error = output.Error
		span.SetStatus(codes.Error, "工具调用被拒绝")
		span.SetAttributes(attribute.String("app.tool.error", output.Error))
	}
	if err != nil {
		step.Error = "工具执行未完成"
	}
	t.result.Steps = append(t.result.Steps, step)
	return output, err
}
