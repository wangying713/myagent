package agent

import "context"

const ChatQuestion = "用一句话解释什么是递归。"

type ChatInput struct {
	Question string
}

// Chat 发送一次模型请求；未提供问题时使用默认问题。
func (s *Service) Chat(ctx context.Context, input ChatInput) (result ExecutionResult, err error) {
	result = s.newResult("chat")
	if input.Question == "" {
		input.Question = ChatQuestion
	}
	if err := validateQuestion(input.Question); err != nil {
		return result, err
	}
	ctx, execution := s.begin(ctx, &result)
	defer func() { execution.end(err) }()
	response, err := execution.model.Chat(ctx, Request{Messages: []Message{
		{Role: "system", Content: "你是一个简洁的中文助手。"},
		{Role: "user", Content: input.Question},
	}})
	if err != nil {
		return result, err
	}
	if response.FinishReason != "stop" {
		return result, ErrInvalidResponse
	}
	result.Answer = response.Message.Content
	return result, nil
}
