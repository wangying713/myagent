package agent

import "context"

const (
	FirstQuestion  = "我给一只虚构小猫取名叫豆包，请记住。"
	SecondQuestion = "我给小猫取了什么名字？"
)

type MessagesInput struct {
	Questions []string
}

// Messages 顺序执行两轮，第二轮携带第一轮实际返回的 assistant 消息。
func (s *Service) Messages(ctx context.Context, input MessagesInput) (result ExecutionResult, err error) {
	result = s.newResult("messages")
	if input.Questions == nil {
		input.Questions = []string{FirstQuestion, SecondQuestion}
	}
	if len(input.Questions) != 2 {
		return result, ErrInput
	}
	for _, question := range input.Questions {
		if err := validateQuestion(question); err != nil {
			return result, err
		}
	}
	ctx, execution := s.begin(ctx, &result)
	defer func() { execution.end(err) }()
	messages := []Message{{Role: "system", Content: "用简短中文回答。"}}
	for _, question := range input.Questions {
		messages = append(messages, Message{Role: "user", Content: question})
		response, callErr := execution.model.Chat(ctx, Request{Messages: messages})
		if callErr != nil {
			return result, callErr
		}
		if response.FinishReason != "stop" {
			return result, ErrInvalidResponse
		}
		messages = append(messages, response.Message)
		result.Answer = response.Message.Content
	}
	return result, nil
}
