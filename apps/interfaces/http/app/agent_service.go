package app

import (
	"context"
	"errors"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"
)

// runSpanName 是应用层业务运行 span 的稳定名称。
const runSpanName = "agent.run"

// agentService 在应用边界管理运行观测和展示副本，领域客户端只负责执行。
type agentService struct {
	agentCli   *agent.Client       // 执行聊天及各业务案例的领域客户端。
	observeCli *openobserve.Client // 具体观测实现，由应用入口管理生命周期。
	limits     agent.Limits        // 启动时校验的默认执行预算。
}

// newAgentService 组装执行与观测依赖，不创建额外的观测接口。
func newAgentService(agentCli *agent.Client, observeCli *openobserve.Client, limits agent.Limits) (*agentService, error) {
	if agentCli == nil || observeCli == nil {
		return nil, errors.New("Agent 应用依赖不能为空")
	}
	return &agentService{
		agentCli:   agentCli,
		observeCli: observeCli,
		limits:     limits,
	}, nil
}

// Chat 建立运行观测，直接使用调用方传入的工具集合。
// 输入校验失败返回 nil；开始执行后保留完整或部分结果。
func (a *agentService) Chat(ctx context.Context, req agent.ChatReq) (result *agent.ChatRsp, err error) {
	if req.Operation == "" {
		req.Operation = agent.OperationChat
	}
	if err := a.validCahtReq(req); err != nil {
		return nil, err
	}

	ctx, runTrace := a.observeCli.StartTrace(ctx, runSpanName, req.Operation)
	defer func() {
		runTrace.End(result, err)
	}()
	result, err = a.agentCli.Chat(ctx, req)
	return result, err
}

// validCahtReq 在建立运行观测前校验聊天问题和工具调用策略。
func (a *agentService) validCahtReq(req agent.ChatReq) error {
	if err := agent.ValidateQuestion(req.Question); err != nil {
		return err
	}
	if req.ToolChoice != "" && req.ToolChoice != agent.ToolChoiceAuto && req.ToolChoice != agent.ToolChoiceNone {
		return agent.ErrInput
	}
	return nil
}

// Messages 为整段对话建立一个运行 span，实际历史始终由领域层维护。
// 输入校验失败返回 nil；执行失败时保留已完成的轮次。
func (a *agentService) Messages(ctx context.Context, req agent.MessagesInput) (result *agent.ChatRsp, err error) {
	if req.Questions != nil {
		if len(req.Questions) != 2 {
			return nil, agent.ErrInput
		}
		for _, question := range req.Questions {
			if err := agent.ValidateQuestion(question); err != nil {
				return nil, err
			}
		}
	}
	ctx, runTrace := a.observeCli.StartTrace(ctx, runSpanName, agent.OperationMessages)
	defer func() {
		runTrace.End(result, err)
	}()
	result, err = a.agentCli.Messages(ctx, req)
	return result, err
}

// validateDemoQuestion 允许案例使用自己的默认问题，显式问题必须符合领域约束。
func validateDemoQuestion(question string) error {
	if question == "" {
		return nil
	}
	return agent.ValidateQuestion(question)
}
