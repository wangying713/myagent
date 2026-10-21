package app

import (
	"context"
	"errors"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// ImplAgent 为各接口准备领域输入，并显式组装各自的 HTTP 响应。
type ImplAgent struct {
	agentCli *agentService // 由入口注入的 Agent 应用服务。
}

func newImplAgent(agentCli *agentService) (*ImplAgent, error) {
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
	if result == nil {
		return nil, err
	}
	return &entity.ChatRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: a.agentCli.observeCli.Provider != nil,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       result.ModelCalls,
		ToolCalls:        result.ToolCalls,
		KnownTokens:      result.KnownTokens,
		UsageComplete:    result.UsageComplete,
		Calls:            agentCalls(result.Calls),
	}, err
}

// Messages 执行两轮对话，第二轮携带第一轮实际返回的消息。 @alias=/api/agent/messages
func (a *ImplAgent) Messages(ctx context.Context, req *entity.MessagesReq) (*entity.MessagesRsp, error) {
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{
		Questions: req.Questions,
	})
	if result == nil {
		return nil, err
	}
	return &entity.MessagesRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: a.agentCli.observeCli.Provider != nil,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       result.ModelCalls,
		ToolCalls:        result.ToolCalls,
		KnownTokens:      result.KnownTokens,
		UsageComplete:    result.UsageComplete,
		Calls:            agentCalls(result.Calls),
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
	if result == nil {
		return nil, err
	}
	return &entity.RunRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: a.agentCli.observeCli.Provider != nil,
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       result.ModelCalls,
		ToolCalls:        result.ToolCalls,
		KnownTokens:      result.KnownTokens,
		UsageComplete:    result.UsageComplete,
		Calls:            agentCalls(result.Calls),
	}, err
}

// agentResult 转换共用的 Agent 执行结果，各接口响应仍由对应方法组装。
func (a *agentService) agentResult(result *agent.ChatRsp) *entity.AgentResult {
	if result == nil {
		return nil
	}
	return &entity.AgentResult{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: a.observeCli.Provider != nil && result.Operation != "",
		ElapsedMS:        result.ElapsedMS,
		ModelCalls:       result.ModelCalls,
		ToolCalls:        result.ToolCalls,
		KnownTokens:      result.KnownTokens,
		UsageComplete:    result.UsageComplete,
		Calls:            agentCalls(result.Calls),
	}
}

// agentCalls 转换接口复用的调用记录与用量数据，保留 nil 与空切片的区别。
func agentCalls(calls []agent.CallRecord) []entity.AgentCallRecord {
	var data []entity.AgentCallRecord
	if calls != nil {
		data = make([]entity.AgentCallRecord, 0, len(calls))
	}
	for _, record := range calls {
		var usage *entity.AgentUsage
		if record.Usage != nil {
			usage = &entity.AgentUsage{
				PromptTokens:     record.Usage.PromptTokens,
				CompletionTokens: record.Usage.CompletionTokens,
				TotalTokens:      record.Usage.TotalTokens,
			}
		}
		data = append(data, entity.AgentCallRecord{
			Kind:         record.Kind,
			Name:         record.Name,
			Request:      record.Request,
			Response:     record.Response,
			ElapsedMS:    record.ElapsedMS,
			FinishReason: record.FinishReason,
			Usage:        usage,
			Error:        record.Error,
		})
	}
	return data
}
