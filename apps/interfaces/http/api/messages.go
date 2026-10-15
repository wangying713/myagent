package api

import (
	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/interfaces/http/api/agent/v1"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

// Messages 处理 POST /api/agent/messages，保留固定两轮的现有业务行为。
func (a *Agent) Messages(c *gin.Context) {
	var request agentv1.MessagesRequest
	present, err := httpx.DecodeProtoJSON(c, &request)
	if err != nil {
		writeRequestError(c, err)
		return
	}
	questions := request.GetQuestions()
	// repeated 字段没有存在性，不能将显式空数组当成省略后使用默认问题。
	if present["questions"] && len(questions) == 0 {
		questions = []string{}
	}
	ctx, cancel := a.context(c)
	defer cancel()
	result, err := a.service.Messages(ctx, agent.MessagesInput{Questions: questions})
	writeResult(c, result, err)
}
