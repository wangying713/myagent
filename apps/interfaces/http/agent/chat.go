package agentapi

import (
	"context"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// Chat 处理 POST /api/agent/chat。
func (a *Agent) Chat(ctx context.Context, req *entity.ChatRequest) (*entity.ExecutionResult, error) {
	question, err := questionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.service.Chat(ctx, agent.ChatInput{Question: question})
	return resultMessage(result), err
}
