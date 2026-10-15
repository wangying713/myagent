package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

type RunRequest struct {
	Question *string `json:"question,omitempty"`
}

// Run 处理 POST /api/agent/run。
func (a *Agent) Run(c *gin.Context) {
	var request RunRequest
	if err := httpx.DecodeJSON(c, &request, "question"); err != nil {
		httpx.WriteRequestError(c, err, ExecutionResult{})
		return
	}
	var input agent.RunInput
	if request.Question != nil {
		if *request.Question == "" {
			writeResult(c, agent.ExecutionResult{}, agent.ErrInput)
			return
		}
		input.Question = *request.Question
	}
	ctx, cancel := a.context(c)
	defer cancel()
	result, err := a.service.Run(ctx, input)
	writeResult(c, result, err)
}
