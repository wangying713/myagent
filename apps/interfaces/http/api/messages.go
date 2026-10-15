package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

type MessagesRequest struct {
	Questions []string `json:"questions,omitempty"`
}

// Messages 处理 POST /api/agent/messages，保留固定两轮的现有业务行为。
func (a *Agent) Messages(c *gin.Context) {
	var request MessagesRequest
	if err := httpx.DecodeJSON(c, &request, "questions"); err != nil {
		httpx.WriteRequestError(c, err, ExecutionResult{})
		return
	}
	ctx, cancel := a.context(c)
	defer cancel()
	result, err := a.service.Messages(ctx, agent.MessagesInput{Questions: request.Questions})
	writeResult(c, result, err)
}
