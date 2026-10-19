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

// ChatQuestion 是单轮聊天在未指定问题时使用的学习示例。
const ChatQuestion = "用一句话解释什么是递归。"

// ChatPrompt 是单轮聊天实际使用的系统提示词。
const ChatPrompt = "你是一个简洁的中文助手。"

// ChatInput 保存单轮问题；Question 为空字符串时使用 ChatQuestion。
type ChatInput struct {
	Question string // 单轮聊天问题，空字符串使用默认示例。
}

// Chat 发送一次模型请求；未提供问题时使用默认问题。
func (c *Client) Chat(ctx context.Context, req ChatInput) (result Result, err error) {
	result = NewResult(OperationChat, c.observer.Enabled())
	if req.Question == "" {
		req.Question = ChatQuestion
	}
	ctx, run, err := c.prepareRun(ctx, &result, RunDetails{
		Questions: []string{req.Question},
		Prompt:    ChatPrompt,
	})
	if err != nil {
		return result, err
	}
	defer func() {
		run.end(err)
	}()
	response, err := run.chatCli.Chat(ctx, Request{
		Messages: []Message{
			{
				Role:    RoleSystem,
				Content: ChatPrompt,
			},
			{
				Role:    RoleUser,
				Content: req.Question,
			},
		},
	})
	if err != nil {
		return result, err
	}
	if response.FinishReason != FinishReasonStop {
		return result, ErrInvalidResponse
	}
	result.Answer = response.Message.Content
	return result, nil
}

// chatWithToolsReq 保存本次工具对话的业务输入，模型客户端由 Client 提供。
type chatWithToolsReq struct {
	Operation string // 本次业务标识，用于执行结果和观测记录。
	Tools     Tools  // 模型可以调用的工具，不能为 nil。
	Question  string // 已由业务入口补齐默认值的问题。
	Prompt    string // 告诉模型该怎么回答的系统提示词。
	Limits    Limits // 全零时使用客户端默认预算；单次覆盖时三个值都必须大于 0。
}

// chatWithTools 统一准备记录、执行模型与工具对话，并在结束时汇总结果。
// 工具拒绝时将原因交给模型；取消或预算用尽时保留已有步骤和用量。
func (c *Client) chatWithTools(ctx context.Context, req chatWithToolsReq) (result Result, err error) {
	result = NewResult(req.Operation, c.observer.Enabled())
	if req.Tools == nil {
		return result, ErrInput
	}
	limits := c.limits
	if req.Limits != (Limits{}) {
		limits = req.Limits
	}
	if limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return result, fmt.Errorf("运行预算必须为正数")
	}
	// 只给当前调用设置预算，客户端本身保持不变，可以继续供其他请求使用。
	local := *c
	local.limits = limits
	ctx, run, err := local.prepareRun(ctx, &result, RunDetails{
		Questions:  []string{req.Question},
		Prompt:     req.Prompt,
		Tools:      req.Tools.Definitions(),
		ToolChoice: ToolChoiceAuto,
	})
	if err != nil {
		return result, err
	}
	defer func() {
		run.end(err)
	}()
	tools := &recordingTools{
		tools:    req.Tools,
		observer: c.observer,
		result:   &result,
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
	toolCallCount := 0
	remainingTokens := limits.Tokens
	seenToolCallIDs := make(map[string]bool)
	for round := 0; round < limits.ModelCalls; round++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if remainingTokens == 0 {
			return result, ErrBudgetExceeded
		}
		rsp, err := run.chatCli.Chat(ctx, Request{
			Messages:   messages,
			Tools:      tools.Definitions(),
			ToolChoice: ToolChoiceAuto,
		})
		if err != nil {
			return result, fmt.Errorf("第 %d 轮模型调用: %w", round+1, err)
		}
		if err := rsp.Validate(); err != nil {
			return result, err
		}
		if used, known := rsp.Usage.Total(); known {
			// 这次可以用超，但剩余额度归零后就不能再调用模型。
			remainingTokens -= min(used, remainingTokens)
		}
		messages = append(messages, rsp.Message)
		if rsp.FinishReason == FinishReasonStop {
			result.Answer = rsp.Message.Content
			return result, nil
		}
		toolCalls := rsp.Message.ToolCalls
		if len(toolCalls) > limits.ToolCalls-toolCallCount {
			return result, ErrBudgetExceeded
		}
		if err := validateToolCalls(toolCalls, seenToolCallIDs); err != nil {
			return result, err
		}
		for _, toolCall := range toolCalls {
			message, err := callTool(ctx, tools, toolCall)
			if err != nil {
				return result, err
			}
			toolCallCount++
			messages = append(messages, message)
		}
	}
	return result, ErrBudgetExceeded
}

// validateToolCalls 先检查这一轮所有工具的 ID，再开始执行，避免执行到一半才发现 ID 有问题。
func validateToolCalls(toolCalls []ToolCall, seenToolCallIDs map[string]bool) error {
	for _, toolCall := range toolCalls {
		if toolCall.ID == "" || seenToolCallIDs[toolCall.ID] {
			return ErrInvalidResponse
		}
		seenToolCallIDs[toolCall.ID] = true
	}
	return nil
}

// callTool 执行一个工具，把结果整理成模型能继续读取的消息。
func callTool(ctx context.Context, tools Tools, req ToolCall) (Message, error) {
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
