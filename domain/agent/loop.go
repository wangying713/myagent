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
	Chat(ctx context.Context, req Request) (Response, error)
}

// Tools 提供已注册的工具定义和分发能力，协议拒绝通过 ToolResult 返回。
type Tools interface {
	Definitions() []ToolDefinition
	// Dispatch 返回工具结果；上下文取消等执行错误通过 error 返回。
	Dispatch(ctx context.Context, req ToolCall) (ToolResult, error)
}

// LoopInput 放这次工具循环要用的模型、工具、问题和调用限制。
type LoopInput struct {
	Model    ChatClient // 这次使用的模型，不能为 nil。
	Tools    Tools      // 模型可以调用的工具，不能为 nil。
	Question string     // 用户要问的问题。
	Prompt   string     // 告诉模型该怎么回答的系统提示词。
	Limits   Limits     // 最多调用几次模型、工具，以及能用多少 token；都必须大于 0。
}

// Run 先问模型，需要工具时就执行工具，再把工具结果交给模型，直到得到回答或用完额度。
// 每次运行单独保存消息和用量。工具拒绝执行时，也会把拒绝原因告诉模型。
// 模型和工具收到 ctx 的取消通知后，要能停止执行。
func Run(ctx context.Context, req LoopInput) (string, error) {
	limits := req.Limits
	if limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return "", fmt.Errorf("运行预算必须为正数")
	}
	messages := []Message{
		{
			Role:    RoleSystem,
			Content: req.Prompt,
		},
		{
			Role:    RoleUser,
			Content: req.Question,
		},
	}
	toolCalls := 0
	remainingTokens := limits.Tokens
	seen := make(map[string]bool)
	for round := 0; round < limits.ModelCalls; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if remainingTokens == 0 {
			return "", ErrBudgetExceeded
		}
		response, err := req.Model.Chat(ctx, Request{
			Messages:   messages,
			Tools:      req.Tools.Definitions(),
			ToolChoice: ToolChoiceAuto,
		})
		if err != nil {
			return "", fmt.Errorf("第 %d 轮模型调用: %w", round+1, err)
		}
		if err := response.Validate(); err != nil {
			return "", err
		}
		if used, known := response.Usage.Total(); known {
			// 这次可以用超，但剩余额度归零后就不能再调用模型。
			remainingTokens -= min(used, remainingTokens)
		}
		messages = append(messages, response.Message)
		if response.FinishReason == FinishReasonStop {
			return response.Message.Content, nil
		}
		calls := response.Message.ToolCalls
		if len(calls) > limits.ToolCalls-toolCalls {
			return "", ErrBudgetExceeded
		}
		if err := validateToolCalls(calls, seen); err != nil {
			return "", err
		}
		for _, call := range calls {
			message, err := runTool(ctx, req.Tools, call)
			if err != nil {
				return "", err
			}
			toolCalls++
			messages = append(messages, message)
		}
	}
	return "", ErrBudgetExceeded
}

// validateToolCalls 先检查这一轮所有工具的 ID，再开始执行，避免执行到一半才发现 ID 有问题。
func validateToolCalls(calls []ToolCall, seen map[string]bool) error {
	for _, call := range calls {
		if call.ID == "" || seen[call.ID] {
			return ErrInvalidResponse
		}
		seen[call.ID] = true
	}
	return nil
}

// runTool 执行一个工具，把结果整理成模型能继续读取的消息。
func runTool(ctx context.Context, tools Tools, req ToolCall) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	output, err := tools.Dispatch(ctx, req)
	if err != nil {
		return Message{}, err
	}
	body, err := json.Marshal(output)
	if err != nil {
		return Message{}, fmt.Errorf("编码工具结果: %w", err)
	}
	return Message{
		Role:       RoleTool,
		ToolCallID: req.ID,
		Content:    string(body),
	}, nil
}
