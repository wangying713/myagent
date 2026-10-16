package agent

import "context"

const (
	// FirstQuestion 是历史消息示例中要求模型记住名字的第一轮问题。
	FirstQuestion = "我给一只虚构小猫取名叫豆包，请记住。"
	// SecondQuestion 是历史消息示例中检查上下文是否保留的第二轮问题。
	SecondQuestion = "我给小猫取了什么名字？"
)

// MessagesInput 保存固定两轮的问题；Questions 为 nil 时使用默认示例，空切片无效。
type MessagesInput struct {
	Questions []string
}

// Messages 顺序执行两轮，第二轮携带第一轮实际返回的 assistant 消息。
// 第二轮失败时仍返回第一轮已完成的回答、步骤和已知用量。
func (s *Service) Messages(ctx context.Context, input MessagesInput) (result ExecutionResult, err error) {
	result = s.newResult(OperationMessages)
	if input.Questions == nil {
		input.Questions = []string{
			FirstQuestion,
			SecondQuestion,
		}
	}
	if len(input.Questions) != 2 {
		return result, ErrInput
	}
	for _, question := range input.Questions {
		if err := validateQuestion(question); err != nil {
			return result, err
		}
	}
	ctx, execution := s.begin(ctx, &result, input.Questions)
	defer func() {
		execution.end(err)
	}()
	messages := []Message{
		{
			Role:    RoleSystem,
			Content: "用简短中文回答。",
		},
	}
	for _, question := range input.Questions {
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: question,
		})
		response, callErr := execution.model.Chat(ctx, Request{Messages: messages})
		if callErr != nil {
			return result, callErr
		}
		if response.FinishReason != FinishReasonStop {
			return result, ErrInvalidResponse
		}
		messages = append(messages, response.Message)
		result.Answer = response.Message.Content
	}
	return result, nil
}
