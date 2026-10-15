package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/interfaces/http/api/agent/v1"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

// Run 处理 POST /api/agent/run。
func (a *Agent) Run(c *gin.Context) {
	var request agentv1.RunRequest
	if _, err := httpx.DecodeProtoJSON(c, &request); err != nil {
		writeRequestError(c, err)
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
