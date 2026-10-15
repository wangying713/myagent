package agent

import "context"

const (
	AgentQuestion = "每盒有 17 支笔，买 23 盒，一共多少支？"
	AgentPrompt   = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
)

type RunInput struct {
	Question string
}

// Run 通过受预算约束的循环调用模型和已注册的乘法工具。
func (s *Service) Run(ctx context.Context, input RunInput) (result ExecutionResult, err error) {
	result = s.newResult("agent")
	if input.Question == "" {
		input.Question = AgentQuestion
	}
	if err := validateQuestion(input.Question); err != nil {
		return result, err
	}
	ctx, execution := s.begin(ctx, &result)
	defer func() { execution.end(err) }()
	tools := &recordingTools{inner: MultiplyTools{}, observer: s.observer, result: &result}
	result.Answer, err = Run(ctx, execution.model, tools, input.Question, AgentPrompt, s.limits)
	return result, err
}
