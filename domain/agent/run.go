package agent

import "context"

const (
	AgentQuestion = "每盒有 17 支笔，买 23 盒，一共多少支？" // 工具调用示例的默认乘法问题。
	// AgentPrompt 约束模型通过工具计算乘法，并在工具失败时如实回答。
	AgentPrompt = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
)

// RunInput 保存工具循环的问题；Question 为空字符串时使用 AgentQuestion。
type RunInput struct {
	Question string // 工具循环问题，空字符串使用默认乘法示例。
}

// Run 通过受预算约束的循环调用模型和已注册的乘法工具。
func (c *Client) Run(ctx context.Context, req RunInput) (result ExecutionResult, err error) {
	result = c.newResult(OperationRun)
	if req.Question == "" {
		req.Question = AgentQuestion
	}
	if err := validateQuestion(req.Question); err != nil {
		return result, err
	}
	baseTools := MultiplyTools{}
	ctx, run := c.startRun(ctx, &result, RunDetails{
		Questions:  []string{req.Question},
		Prompt:     AgentPrompt,
		Tools:      baseTools.Definitions(),
		ToolChoice: ToolChoiceAuto,
	})
	defer func() {
		run.end(err)
	}()
	tools := &recordingTools{
		tools:    baseTools,
		observer: c.observer,
		result:   &result,
	}
	result.Answer, err = Run(ctx, LoopInput{
		ModelCli: run.modelCli,
		Tools:    tools,
		Question: req.Question,
		Prompt:   AgentPrompt,
		Limits:   c.limits,
	})
	return result, err
}
