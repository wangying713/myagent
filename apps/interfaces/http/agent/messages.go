package agentapi

import (
	"context"

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
)

// Messages 处理 POST /api/agent/messages，保留固定两轮的现有业务行为。
func (a *Agent) Messages(ctx context.Context, req *httpproto.MessagesRequest) (*httpproto.ExecutionResult, error) {
	result, err := a.service.Messages(ctx, agent.MessagesInput{Questions: req.GetQuestions()})
	return resultMessage(result), err
}
