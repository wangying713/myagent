package app

import (
	"context"
	"errors"

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
)

// agentAPI 为各接口准备领域输入，并显式组装各自的 Proto 响应。
type agentAPI struct {
	service *agent.Service
}

func newAgentAPI(service *agent.Service) (*agentAPI, error) {
	if service == nil {
		return nil, errors.New("Agent API 业务依赖不能为空")
	}
	return &agentAPI{service: service}, nil
}

// Chat 处理 POST /api/agent/chat。
func (a *agentAPI) Chat(ctx context.Context, req *proto.ChatReq) (*proto.ChatRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.service.Chat(ctx, agent.ChatInput{Question: question})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.ChatRsp{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete, Steps: agentSteps(result.Steps),
	}, err
}

// Messages 处理 POST /api/agent/messages，保留固定两轮的现有业务行为。
func (a *agentAPI) Messages(ctx context.Context, req *proto.MessagesReq) (*proto.MessagesRsp, error) {
	result, err := a.service.Messages(ctx, agent.MessagesInput{Questions: req.GetQuestions()})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.MessagesRsp{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete, Steps: agentSteps(result.Steps),
	}, err
}

// Run 处理 POST /api/agent/run。
func (a *agentAPI) Run(ctx context.Context, req *proto.RunReq) (*proto.RunRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.service.Run(ctx, agent.RunInput{Question: question})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.RunRsp{
		Lesson: result.Operation, Completed: result.Completed, Answer: result.Answer,
		TelemetryEnabled: result.TelemetryEnabled, ElapsedMs: result.ElapsedMS,
		ModelCalls: int64(result.ModelCalls), ToolCalls: int64(result.ToolCalls), KnownTokens: int64(result.KnownTokens),
		UsageComplete: result.UsageComplete, Steps: agentSteps(result.Steps),
	}, err
}

// prepareAgentMessages 在 HTTP 解码后保留省略与显式空数组的区别。
// repeated 字段本身没有存在性，不能把显式空数组变为默认问题。
func prepareAgentMessages(req *proto.MessagesReq, present map[string]bool) {
	if present["questions"] && len(req.Questions) == 0 {
		req.Questions = []string{}
	}
}

func agentQuestionInput(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	// 省略使用业务默认问题，显式空字符串仍应拒绝。
	if *value == "" {
		return "", agent.ErrInput
	}
	return *value, nil
}

// agentSteps 转换接口复用的步骤与用量数据，保留 nil 与空切片的区别。
func agentSteps(steps []agent.ExecutionStep) []*proto.ExecutionStep {
	var data []*proto.ExecutionStep
	if steps != nil {
		data = make([]*proto.ExecutionStep, 0, len(steps))
	}
	for _, step := range steps {
		var usage *proto.Usage
		if step.Usage != nil {
			usage = &proto.Usage{
				PromptTokens:     agentTokenValue(step.Usage.PromptTokens),
				CompletionTokens: agentTokenValue(step.Usage.CompletionTokens),
				TotalTokens:      agentTokenValue(step.Usage.TotalTokens),
			}
		}
		data = append(data, &proto.ExecutionStep{
			Kind: step.Kind, Name: step.Name, Request: step.Request, Response: step.Response,
			ElapsedMs: step.ElapsedMS, FinishReason: step.FinishReason, Usage: usage, Error: step.Error,
		})
	}
	return data
}

func agentTokenValue(value *int) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}
