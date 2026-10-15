package agentapi

import (
	"context"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// Run 处理 POST /api/agent/run。
func (a *Agent) Run(ctx context.Context, req *entity.RunRequest) (*entity.ExecutionResult, error) {
	question, err := questionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.service.Run(ctx, agent.RunInput{Question: question})
	return resultMessage(result), err
}
