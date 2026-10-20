package app

import (
	"context"
	"errors"

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
)

// ImplAgent 为各接口准备领域输入，并显式组装各自的 Proto 响应。
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
func (a *ImplAgent) Chat(ctx context.Context, req *proto.ChatReq) (*proto.ChatRsp, error) {
	question := req.GetQuestion()
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
	return &proto.ChatRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMs:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// Messages 执行两轮对话，第二轮携带第一轮实际返回的消息。 @alias=/api/agent/messages
func (a *ImplAgent) Messages(ctx context.Context, req *proto.MessagesReq) (*proto.MessagesRsp, error) {
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{
		Questions: req.GetQuestions(),
	})
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.MessagesRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMs:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// Run 执行受预算约束的工具调用循环。 @alias=/api/agent/run
func (a *ImplAgent) Run(ctx context.Context, req *proto.RunReq) (*proto.RunRsp, error) {
	result, err := a.agentCli.Run(ctx, req.GetQuestion())
	if errors.Is(err, agent.ErrInput) {
		return nil, err
	}
	return &proto.RunRsp{
		Lesson:           result.Operation,
		Completed:        result.Completed,
		Answer:           result.Answer,
		TelemetryEnabled: result.TelemetryEnabled,
		ElapsedMs:        result.ElapsedMS,
		ModelCalls:       int64(result.ModelCalls),
		ToolCalls:        int64(result.ToolCalls),
		KnownTokens:      int64(result.KnownTokens),
		UsageComplete:    result.UsageComplete,
		Steps:            agentSteps(result.Steps),
	}, err
}

// prepareAgentMessages 在 HTTP 解码后保留省略与显式空数组的区别。
// Protobuf repeated 字段不保留存在性，显式空数组必须继续交给业务校验。
func prepareAgentMessages(req *proto.MessagesReq, present map[string]bool) {
	if present["questions"] && len(req.Questions) == 0 {
		req.Questions = []string{}
	}
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
			Kind:         step.Kind,
			Name:         step.Name,
			Request:      step.Request,
			Response:     step.Response,
			ElapsedMs:    step.ElapsedMS,
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
