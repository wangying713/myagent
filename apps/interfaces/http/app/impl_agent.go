package app

import (
	"context"
	"errors"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// ImplAgent 为各接口准备领域输入，并显式组装各自的 HTTP 响应。
type ImplAgent struct {
	agentCli *agent.Client // 由入口注入的 Agent 客户端。
}

func newImplAgent(agentCli *agent.Client) (*ImplAgent, error) {
	if agentCli == nil {
		return nil, errors.New("Agent API 业务依赖不能为空")
	}
	return &ImplAgent{
		agentCli: agentCli,
	}, nil
}

// Chat 执行单轮聊天，省略问题时使用默认问题。 @alias=/api/agent/chat
func (a *ImplAgent) Chat(ctx context.Context, req *entity.ChatReq) (*entity.ChatRsp, error) {
	question := req.Question
	if question == "" {
		question = agent.DemoChatQuestion
	}
	result, err := a.agentCli.Chat(ctx, agent.ChatReq{
		Question:   question,
		Prompt:     agent.DemoChatPrompt,
		ToolChoice: agent.ToolChoiceNone,
	})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &entity.ChatRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// Messages 执行两轮对话，第二轮携带第一轮实际返回的消息。 @alias=/api/agent/messages
func (a *ImplAgent) Messages(ctx context.Context, req *entity.MessagesReq) (*entity.MessagesRsp, error) {
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{
		Questions: req.Questions,
	})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &entity.MessagesRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// Run 执行受预算约束的工具调用循环。 @alias=/api/agent/run
func (a *ImplAgent) Run(ctx context.Context, req *entity.RunReq) (*entity.RunRsp, error) {
	question := req.Question
	if question == "" {
		question = agent.AgentQuestion
	}
	result, err := a.agentCli.Chat(ctx, agent.ChatReq{
		Operation: agent.OperationRun,
		Tools:     agent.MultiplyTools{},
		Question:  question,
		Prompt:    agent.AgentPrompt,
	})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &entity.RunRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// agentSteps 转换接口复用的步骤与用量数据，保留 nil 与空切片的区别。
func agentSteps(steps []agent.ExecutionStep) []entity.AgentExecutionStep {
	var data []entity.AgentExecutionStep
	if steps != nil {
		data = make([]entity.AgentExecutionStep, 0, len(steps))
	}
	for _, step := range steps {
		var usage *entity.AgentUsage
		if step.Usage != nil {
			usage = &entity.AgentUsage{
				PromptTokens:     agentTokenValue(step.Usage.PromptTokens),
				CompletionTokens: agentTokenValue(step.Usage.CompletionTokens),
				TotalTokens:      agentTokenValue(step.Usage.TotalTokens),
			}
		}
		data = append(data, entity.AgentExecutionStep{
			Kind:         step.Kind,
			Name:         step.Name,
			Request:      step.Request,
			Response:     step.Response,
			ElapsedMS:    step.ElapsedMS,
			FinishReason: step.FinishReason,
			Usage:        usage,
			Error:        step.Error,
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
