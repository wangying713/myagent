package agent

import "context"

const (
	AgentQuestion = "每盒有 17 支笔，买 23 盒，一共多少支？"               // 工具调用示例的默认乘法问题。
	AgentPrompt   = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。" // 约束模型通过工具计算乘法，并在工具失败时如实回答。
)

// RunInput 保存工具循环的问题；Question 为空字符串时使用 AgentQuestion。
type RunInput struct {
	Question string
}

// Run 通过受预算约束的循环调用模型和已注册的乘法工具。
func (s *Service) Run(ctx context.Context, input RunInput) (result ExecutionResult, err error) {
	result = s.newResult(OperationRun)
	if input.Question == "" {
		input.Question = AgentQuestion
	}
	if err := validateQuestion(input.Question); err != nil {
		return result, err
	}
	baseTools := MultiplyTools{}
	ctx, execution := s.begin(ctx, &result, RunDetails{
		Questions:  []string{input.Question},
		Prompt:     AgentPrompt,
		Tools:      baseTools.Definitions(),
		ToolChoice: ToolChoiceAuto,
	})
	defer func() {
		execution.end(err)
	}()
	tools := &recordingTools{
		inner:    baseTools,
		observer: s.observer,
		result:   &result,
	}
	result.Answer, err = Run(ctx, execution.model, tools, input.Question, AgentPrompt, s.limits)
	return result, err
}
