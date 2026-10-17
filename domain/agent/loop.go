package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// ChatClient 由模型的使用方定义，只要求当前业务需要的非流式能力。
type ChatClient interface {
	// Chat 执行一次模型请求，须响应 ctx 取消。
	// 失败时可同时返回用于记录的响应数据。
	Chat(context.Context, Request) (Response, error)
}

// Tools 提供已注册的工具定义和分发能力，协议拒绝通过 ToolResult 返回。
type Tools interface {
	Definitions() []ToolDefinition
	// Dispatch 返回工具结果；上下文取消等执行错误通过 error 返回。
	Dispatch(context.Context, ToolCall) (ToolResult, error)
}

// Run 顺序执行模型 → 工具 → 回填 → 模型，直到完成或达到预算。
// 消息、用量和调用 ID 都属于本次运行，不保存在共享对象上。
// model 和 tools 须非 nil 且响应上下文取消；工具拒绝的结果仍会回填给模型。
func Run(ctx context.Context, model ChatClient, tools Tools, question, prompt string, limits Limits) (string, error) {
	if limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return "", fmt.Errorf("运行预算必须为正数")
	}
	messages := []Message{
		{
			Role:    RoleSystem,
			Content: prompt,
		},
		{
			Role:    RoleUser,
			Content: question,
		},
	}
	toolCalls, tokens := 0, 0
	seen := make(map[string]bool)
	for round := 0; round < limits.ModelCalls; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if tokens >= limits.Tokens {
			return "", ErrBudgetExceeded
		}
		response, err := model.Chat(ctx, Request{
			Messages:   messages,
			Tools:      tools.Definitions(),
			ToolChoice: ToolChoiceAuto,
		})
		if err != nil {
			return "", fmt.Errorf("第 %d 轮模型调用: %w", round+1, err)
		}
		if err := response.Validate(); err != nil {
			return "", err
		}
		if used, known := response.Usage.Total(); known {
			// 单次调用允许越过软预算，但下一次调用必须停止。
			if used >= limits.Tokens-tokens {
				tokens = limits.Tokens
			} else {
				tokens += used
			}
		}
		messages = append(messages, response.Message)
		if response.FinishReason == FinishReasonStop {
			return response.Message.Content, nil
		}
		calls := response.Message.ToolCalls
		if len(calls) > limits.ToolCalls-toolCalls {
			return "", ErrBudgetExceeded
		}
		// 整轮先检查 ID，避免部分工具已经执行才发现同轮协议错误。
		for _, call := range calls {
			if call.ID == "" || seen[call.ID] {
				return "", ErrInvalidResponse
			}
			seen[call.ID] = true
		}
		for _, call := range calls {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			output, err := tools.Dispatch(ctx, call)
			if err != nil {
				return "", err
			}
			toolCalls++
			body, err := json.Marshal(output)
			if err != nil {
				return "", fmt.Errorf("编码工具结果: %w", err)
			}
			messages = append(messages, Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Content:    string(body),
			})
		}
	}
	return "", ErrBudgetExceeded
}
