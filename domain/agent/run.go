package agent

import "context"

const (
	AgentQuestion = "每盒有 17 支笔，买 23 盒，一共多少支？" // 工具调用示例的默认乘法问题。
	// AgentPrompt 约束模型通过工具计算乘法，并在工具失败时如实回答。
	AgentPrompt = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
)

// Run 通过受预算约束的循环调用模型和已注册的乘法工具。
// question 为空字符串时使用 AgentQuestion。
func (c *Client) Run(ctx context.Context, question string) (Result, error) {
	if question == "" {
		question = AgentQuestion
	}
	return c.chatWithTools(ctx, chatWithToolsReq{
		Operation: OperationRun,
		Tools:     MultiplyTools{},
		Question:  question,
		Prompt:    AgentPrompt,
	})
}
