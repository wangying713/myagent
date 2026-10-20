package agent

import "context"

const (
	FirstQuestion  = "我给一只虚构小猫取名叫豆包，请记住。" // 历史消息示例中要求模型记住名字的第一轮问题。
	SecondQuestion = "我给小猫取了什么名字？"        // 历史消息示例中检查上下文是否保留的第二轮问题。
	MessagesPrompt = "用简短中文回答。"           // 两轮历史消息实际使用的系统提示词。
)

// MessagesInput 保存固定两轮的问题；Questions 为 nil 时使用默认示例，空切片无效。
type MessagesInput struct {
	// Questions 是按顺序执行的两轮问题；nil 使用默认示例，空切片无效。
	Questions []string
}

// Messages 顺序执行两轮，第二轮携带第一轮实际返回的 assistant 消息。
// 第二轮失败时仍返回第一轮已完成的回答、步骤和已知用量。
func (c *Client) Messages(ctx context.Context, req MessagesInput) (result ChatRsp, err error) {
	result = NewChatRsp(OperationMessages)
	if req.Questions == nil {
		req.Questions = []string{
			FirstQuestion,
			SecondQuestion,
		}
	}
	if len(req.Questions) != 2 {
		return result, ErrInput
	}
	run, err := c.prepareRun(&result, RunInput{
		Questions: req.Questions,
		Prompt:    MessagesPrompt,
	})
	if err != nil {
		return result, err
	}
	defer func() {
		run.end(err)
	}()
	messages := []Message{
		{
			Role:    RoleSystem,
			Content: MessagesPrompt,
		},
	}
	for _, question := range req.Questions {
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: question,
		})
		response, callErr := run.callModel(ctx, Request{
			Messages: messages,
		})
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
