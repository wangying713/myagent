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
	agentCli   *agent.Client         // 执行聊天及各业务案例的领域客户端。
	observeCli *openobserve.Client   // 具体观测实现，由应用入口管理生命周期。
	model      openobserve.ModelInfo // 模型客户端的公开配置，不包含凭据。
	limits     agent.Limits          // 启动时校验的默认执行预算。
}

// newAgentService 组装执行与观测依赖，不创建额外的观测接口。
func newAgentService(agentCli *agent.Client, observeCli *openobserve.Client, model openobserve.ModelInfo, limits agent.Limits) (*agentService, error) {
	if agentCli == nil || observeCli == nil {
		return nil, errors.New("Agent 应用依赖不能为空")
	}
	return &agentService{
		agentCli:   agentCli,
		observeCli: observeCli,
		model:      model,
		limits:     limits,
	}, nil
}

// Chat 在业务调用边界建立运行观测，并装饰本次请求使用的工具集合。
// 输入校验失败返回 nil；开始执行后保留完整或部分结果。
func (a *agentService) Chat(ctx context.Context, req agent.ChatReq) (rsp *agent.ChatRsp, err error) {
	if req.Operation == "" {
		req.Operation = agent.OperationChat
	}
	if err := agent.ValidateQuestion(req.Question); err != nil {
		return nil, err
	}
	if req.ToolChoice != "" && req.ToolChoice != agent.ToolChoiceAuto && req.ToolChoice != agent.ToolChoiceNone {
		return nil, agent.ErrInput
	}

	if req.Tools != nil {
		req.Tools = openobserve.Tools{
			Tools:      req.Tools,
			ObserveCli: a.observeCli,
		}
	}
	var result agent.ChatRsp
	ctx, run := a.observeCli.StartRun(ctx, req.Operation, &a.model)
	defer func() {
		a.finishRun(run, &result, err)
	}()
	result, err = a.agentCli.Chat(ctx, req)
	return &result, err
}

// Messages 为整段对话建立一个运行 span，实际历史始终由领域层维护。
// 输入校验失败返回 nil；执行失败时保留已完成的轮次。
func (a *agentService) Messages(ctx context.Context, req agent.MessagesInput) (rsp *agent.ChatRsp, err error) {
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
	var result agent.ChatRsp
	ctx, run := a.observeCli.StartRun(ctx, agent.OperationMessages, &a.model)
	defer func() {
		a.finishRun(run, &result, err)
	}()
	result, err = a.agentCli.Messages(ctx, req)
	return &result, err
}

// finishRun 只处理对外展示的结果副本，保留原始执行输入供观测记录。
func (a *agentService) finishRun(run *openobserve.Run, result *agent.ChatRsp, err error) {
	result.Answer = a.observeCli.Clean(result.Answer)
	result.Calls = slices.Clone(result.Calls)
	for i := range result.Calls {
		record := &result.Calls[i]
		record.Name = a.observeCli.Clean(record.Name)
		record.Request = a.observeCli.Safe([]byte(record.Request))
		record.Response = a.observeCli.Safe([]byte(record.Response))
		record.Error = a.observeCli.Clean(record.Error)
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
