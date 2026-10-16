package agent

import "context"

// ChatQuestion 是单轮聊天在未指定问题时使用的学习示例。
const ChatQuestion = "用一句话解释什么是递归。"

// ChatInput 保存单轮问题；Question 为空字符串时使用 ChatQuestion。
type ChatInput struct {
	Question string
}

// Chat 发送一次模型请求；未提供问题时使用默认问题。
func (s *Service) Chat(ctx context.Context, input ChatInput) (result ExecutionResult, err error) {
	result = s.newResult(OperationChat)
	if input.Question == "" {
		input.Question = ChatQuestion
	}
	if err := validateQuestion(input.Question); err != nil {
		return result, err
	}
	ctx, execution := s.begin(ctx, &result)
	defer func() { execution.end(err) }()
	response, err := execution.model.Chat(ctx, Request{
		Messages: []Message{
			{
				Role:    RoleSystem,
				Content: "你是一个简洁的中文助手。",
			},
			{
				Role:    RoleUser,
				Content: input.Question,
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
