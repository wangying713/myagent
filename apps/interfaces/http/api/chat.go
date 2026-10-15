package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

type ChatRequest struct {
	Question *string `json:"question,omitempty"`
}

// Chat 处理 POST /api/agent/chat。
func (a *Agent) Chat(c *gin.Context) {
	var request ChatRequest
	if err := httpx.DecodeJSON(c, &request, "question"); err != nil {
		httpx.WriteRequestError(c, err, ExecutionResult{})
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
