package app

import (
	"context"
	"errors"
	"slices"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"
)

// agentService 在应用边界管理运行观测和展示副本，领域客户端只负责执行。
type agentService struct {
	agentCli  *agent.Client          // 执行聊天及各业务案例的领域客户端。
	telemetry *openobserve.Telemetry // 具体观测实现，由应用入口管理生命周期。
	model     openobserve.ModelInfo  // 模型客户端的公开配置，不包含凭据。
	limits    agent.Limits           // 启动时校验的默认执行预算。
}

// newAgentService 组装执行与观测依赖，不创建额外的观测接口。
func newAgentService(agentCli *agent.Client, telemetry *openobserve.Telemetry, model openobserve.ModelInfo, limits agent.Limits) (*agentService, error) {
	if agentCli == nil || telemetry == nil {
		return nil, errors.New("Agent 应用依赖不能为空")
	}
	return &agentService{
		agentCli:  agentCli,
		telemetry: telemetry,
		model:     model,
		limits:    limits,
	}, nil
}

// Chat 在业务调用边界建立运行观测，并装饰本次请求使用的工具集合。
func (a *agentService) Chat(ctx context.Context, req agent.ChatReq) (result agent.ChatRsp, err error) {
	if req.Operation == "" {
		req.Operation = agent.OperationChat
	}
	result = agent.NewChatRsp(req.Operation)
	if err := agent.ValidateQuestion(req.Question); err != nil {
		return result, err
	}
	if req.ToolChoice != "" && req.ToolChoice != agent.ToolChoiceAuto && req.ToolChoice != agent.ToolChoiceNone {
		return result, agent.ErrInput
	}

	if req.Tools != nil {
		req.Tools = openobserve.Tools{
			Tools:     req.Tools,
			Telemetry: a.telemetry,
		}
	}
	ctx, run := a.telemetry.StartRun(ctx, req.Operation, &a.model)
	defer func() {
		a.finishRun(run, &result, err)
	}()
	return a.agentCli.Chat(ctx, req)
}

// Messages 为整段对话建立一个运行 span，实际历史始终由领域层维护。
func (a *agentService) Messages(ctx context.Context, req agent.MessagesInput) (result agent.ChatRsp, err error) {
	result = agent.NewChatRsp(agent.OperationMessages)
	if req.Questions != nil {
		if len(req.Questions) != 2 {
			return result, agent.ErrInput
		}
		for _, question := range req.Questions {
			if err := agent.ValidateQuestion(question); err != nil {
				return result, err
			}
		}
	}
	ctx, run := a.telemetry.StartRun(ctx, agent.OperationMessages, &a.model)
	defer func() {
		a.finishRun(run, &result, err)
	}()
	return a.agentCli.Messages(ctx, req)
}

// finishRun 只处理对外展示的结果副本，保留原始执行输入供观测记录。
func (a *agentService) finishRun(run *openobserve.Run, result *agent.ChatRsp, err error) {
	result.Answer = a.telemetry.Clean(result.Answer)
	result.Calls = slices.Clone(result.Calls)
	for i := range result.Calls {
		record := &result.Calls[i]
		record.Name = a.telemetry.Clean(record.Name)
		record.Request = a.telemetry.Safe([]byte(record.Request))
		record.Response = a.telemetry.Safe([]byte(record.Response))
		record.Error = a.telemetry.Clean(record.Error)
	}
	run.End(*result, err)
}

// validateDemoQuestion 允许案例使用自己的默认问题，显式问题必须符合领域约束。
func validateDemoQuestion(question string) error {
	if question == "" {
		return nil
	}
	return agent.ValidateQuestion(question)
}
