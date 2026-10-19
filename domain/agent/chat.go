package agent

import "context"

// ChatQuestion 是单轮聊天在未指定问题时使用的学习示例。
const ChatQuestion = "用一句话解释什么是递归。"

// ChatPrompt 是单轮聊天实际使用的系统提示词。
const ChatPrompt = "你是一个简洁的中文助手。"

// ChatInput 保存单轮问题；Question 为空字符串时使用 ChatQuestion。
type ChatInput struct {
	Question string // 单轮聊天问题，空字符串使用默认示例。
}

// Chat 发送一次模型请求；未提供问题时使用默认问题。
func (c *Client) Chat(ctx context.Context, req ChatInput) (result ExecutionResult, err error) {
	result = NewExecutionResult(OperationChat, c.observer.Enabled())
	if req.Question == "" {
		req.Question = ChatQuestion
	}
	if err := validateQuestion(req.Question); err != nil {
		return result, err
	}
	ctx, run := c.startRun(ctx, &result, RunDetails{
		Questions: []string{req.Question},
		Prompt:    ChatPrompt,
	})
	defer func() {
		run.end(err)
	}()
	response, err := run.modelCli.Chat(ctx, Request{
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
