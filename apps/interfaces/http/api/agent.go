// Package api 定义 HTTP 请求、响应及具名处理方法，不编排 Agent 业务流程。
package api

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"sakelei/ai/domain/agent"
)

type Agent struct {
	service *agent.Service
	timeout time.Duration
}

func NewAgent(service *agent.Service, timeout time.Duration) (*Agent, error) {
	if service == nil || timeout <= 0 {
		return nil, errors.New("HTTP 依赖与请求超时无效")
	}
	return &Agent{service: service, timeout: timeout}, nil
}

func (a *Agent) context(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), a.timeout)
}
