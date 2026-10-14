package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"sakelei/ai/domain/agent/entity"
)

// ChatClient 是 Agent 所需的模型能力，由使用方定义，便于注入离线模型。
type ChatClient interface {
	Chat(context.Context, entity.ChatRequest) (entity.ChatResponse, error)
}

// ToolDispatcher 是 Agent 所需的工具描述和受控分发能力。
type ToolDispatcher interface {
	Definitions() []entity.ToolDefinition
	Dispatch(context.Context, entity.ToolCall) (entity.ToolResult, error)
}

// ErrBudgetExceeded 表示本次运行没有在预算内获得完整答案。
var ErrBudgetExceeded = errors.New("Agent 达到运行预算，任务未完成")

// Limits 限制模型请求和工具执行。TokenBudget 是调用间检查的软预算。
type Limits struct {
	ModelCalls  int
	ToolCalls   int
	TokenBudget int
}

// DefaultLimits 返回第一阶段的默认预算。
func DefaultLimits() Limits {
	return Limits{ModelCalls: 5, ToolCalls: 8, TokenBudget: 8000}
}

// Agent 是同步的有限循环；不同运行的消息和统计互不共享。
type Agent struct {
	model ChatClient
	tools ToolDispatcher
}

// New 创建依赖明确的 Agent。
func New(model ChatClient, tools ToolDispatcher) (*Agent, error) {
	if model == nil || tools == nil {
		return nil, errors.New("模型和工具分发器不能为空")
	}
	return &Agent{model: model, tools: tools}, nil
}

// Run 将工具结果逐个回填。返回错误时仍保留已经执行的统计和证据。
func (a *Agent) Run(ctx context.Context, question, prompt string, limits Limits) (entity.RunResult, error) {
	result := entity.RunResult{UsageComplete: true}
	if limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.TokenBudget <= 0 {
		return result, errors.New("运行预算必须为正数")
	}
	messages := []entity.Message{{Role: "system", Content: prompt}, {Role: "user", Content: question}}
	seenIDs := make(map[string]bool)
	for result.ModelCalls < limits.ModelCalls {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if result.KnownTokens >= limits.TokenBudget {
			return result, fmt.Errorf("累计已知 token 超限: %w", ErrBudgetExceeded)
		}
		result.ModelCalls++
		response, err := a.model.Chat(ctx, entity.ChatRequest{Messages: messages, Tools: a.tools.Definitions(), ToolChoice: "auto"})
		result.AddUsage(response.Usage)
		if err != nil {
			return result, fmt.Errorf("第 %d 次模型调用: %w", result.ModelCalls, err)
		}
		if err := response.Validate(); err != nil {
			return result, err
		}
		messages = append(messages, response.Message)
		if response.FinishReason == "stop" {
			result.Answer = response.Message.Content
			return result, nil
		}
		calls := response.Message.ToolCalls
		if len(calls) > limits.ToolCalls-result.ToolCalls {
			return result, fmt.Errorf("工具调用次数超限: %w", ErrBudgetExceeded)
		}
		// 整轮先验证，避免先执行一个工具后才发现另一个调用的 ID 无效。
		for _, call := range calls {
			if call.ID == "" || seenIDs[call.ID] {
				return result, errors.New("工具调用 ID 缺失或重复")
			}
			seenIDs[call.ID] = true
		}
		for _, call := range calls {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			result.ToolCalls++
			output, err := a.tools.Dispatch(ctx, call)
			if err != nil {
				return result, err
			}
			result.ToolExecutions = append(result.ToolExecutions, entity.ToolExecution{Call: call, Result: output})
			body, err := json.Marshal(output)
			if err != nil {
				return result, fmt.Errorf("编码工具结果: %w", err)
			}
			messages = append(messages, entity.Message{Role: "tool", ToolCallID: call.ID, Content: string(body)})
		}
	}
	return result, fmt.Errorf("模型请求次数超限: %w", ErrBudgetExceeded)
}
