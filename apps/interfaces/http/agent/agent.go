// Package agentapi 将 HTTP 接口消息转换为领域输入，通过具名方法调用 Agent 业务并返回接口响应。
package agentapi

import (
	"errors"

	"saklay/ai/domain/agent"
)

type Agent struct {
	service *agent.Service
}

func NewAgent(service *agent.Service) (*Agent, error) {
	if service == nil {
		return nil, errors.New("Agent API 业务依赖不能为空")
	}
	return &Agent{service: service}, nil
}
