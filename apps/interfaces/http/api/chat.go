package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/interfaces/http/api/agent/v1"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

// Chat 处理 POST /api/agent/chat。
func (a *Agent) Chat(c *gin.Context) {
	var request agentv1.ChatRequest
	if _, err := httpx.DecodeProtoJSON(c, &request); err != nil {
		writeRequestError(c, err)
		return
	}
	var input agent.ChatInput
	if request.Question != nil {
		// 省略字段使用默认值；显式传空字符串仍属于无效输入。
		if *request.Question == "" {
			writeResult(c, agent.ExecutionResult{}, agent.ErrInput)
			return
		}
		input.Question = *request.Question
	}
	ctx, cancel := a.context(c)
	defer cancel()
	result, err := a.service.Chat(ctx, input)
	writeResult(c, result, err)
}
