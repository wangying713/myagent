package app

import (
	"context"
	"errors"
	"iter"
	"strings"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// ImplAgentDemos 为学习页面的每个演示准备输入并组装独立响应，业务仍由领域层执行。
type ImplAgentDemos struct {
	agentCli *agent.Client // 与旧 API 复用的 Agent 客户端，不复制模型客户端或配置。
}

func newImplAgentDemos(agentCli *agent.Client) *ImplAgentDemos {
	return &ImplAgentDemos{
		agentCli: agentCli,
	}
}

// Overview 演示Agent 到底是什么。 @alias=/api/agent-demos/overview
func (a *ImplAgentDemos) Overview(ctx context.Context, req *entity.DemosOverviewReq) (*entity.DemosOverviewRsp, error) {
	result, err := a.agentCli.DemoOverview(ctx, req.Question)
	return &entity.DemosOverviewRsp{
		Direct:    agentResult(result.Direct),
		WithTools: agentResult(result.WithTools),
	}, err
}

// Chat 演示普通聊天。 @alias=/api/agent-demos/chat
func (a *ImplAgentDemos) Chat(ctx context.Context, req *entity.DemosChatReq) (*entity.DemosChatRsp, error) {
	question := req.Question
	if question == "" {
		question = agent.DemoChatQuestion
	}
	result, err := a.agentCli.Chat(ctx, agent.ChatReq{
		Question:   question,
		Prompt:     agent.DemoChatPrompt,
		ToolChoice: agent.ToolChoiceNone,
	})
	return &entity.DemosChatRsp{
		Execution: agentResult(result),
	}, err
}

// Tools 演示工具调用与循环。 @alias=/api/agent-demos/tools
func (a *ImplAgentDemos) Tools(ctx context.Context, req *entity.DemosToolsReq) (*entity.DemosToolsRsp, error) {
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
	return &entity.DemosToolsRsp{
		Execution: agentResult(result),
	}, err
}

// Messages 演示消息历史。 @alias=/api/agent-demos/messages
func (a *ImplAgentDemos) Messages(ctx context.Context, req *entity.DemosMessagesReq) (*entity.DemosMessagesRsp, error) {
	questions := req.Questions
	result, err := a.agentCli.Messages(ctx, agent.MessagesInput{
		Questions: questions,
	})
	return &entity.DemosMessagesRsp{
		Execution: agentResult(result),
	}, err
}

// Structured 演示结构化输出。 @alias=/api/agent-demos/structured
func (a *ImplAgentDemos) Structured(ctx context.Context, req *entity.DemosStructuredReq) (*entity.DemosStructuredRsp, error) {
	result, err := a.agentCli.DemoStructured(ctx, req.Question)
	return &entity.DemosStructuredRsp{
		Execution: agentResult(result.Execution),
		Product:   result.Order.Product,
		Quantity:  result.Order.Quantity,
	}, err
}

// MultipleTools 演示多个工具协作。 @alias=/api/agent-demos/multiple-tools
func (a *ImplAgentDemos) MultipleTools(ctx context.Context, req *entity.DemosMultipleToolsReq) (*entity.DemosMultipleToolsRsp, error) {
	result, err := a.agentCli.DemoMultipleTools(ctx, req.Question)
	return &entity.DemosMultipleToolsRsp{
		Execution: agentResult(result),
	}, err
}

// Reliability 演示失败与执行边界。 @alias=/api/agent-demos/reliability
func (a *ImplAgentDemos) Reliability(ctx context.Context, req *entity.DemosReliabilityReq) (*entity.DemosReliabilityRsp, error) {
	calls := 1
	if req.ModelCalls != nil {
		calls = *req.ModelCalls
	}
	if calls < 1 || calls > agent.DefaultLimits().ModelCalls {
		return nil, agent.ErrInput
	}
	result, err := a.agentCli.DemoReliability(ctx, agent.DemoReliabilityInput{
		Question:   req.Question,
		ModelCalls: calls,
	})
	reason := "已完成，未触发次数限制"
	if errors.Is(err, agent.ErrBudgetExceeded) {
		reason = "已达到本次模型调用次数上限；前面的执行记录仍然保留"
	} else if err != nil {
		reason = "执行未完成，请查看错误与步骤"
	}
	return &entity.DemosReliabilityRsp{
		Execution:  agentResult(result),
		StopReason: reason,
	}, err
}

// Retrieval 演示检索与记忆。 @alias=/api/agent-demos/retrieval
func (a *ImplAgentDemos) Retrieval(ctx context.Context, req *entity.DemosRetrievalReq) (*entity.DemosRetrievalRsp, error) {
	result, err := a.agentCli.DemoRetrieval(ctx, req.Question)
	sources := make([]entity.DemoSource, 0, len(result.Sources))
	for _, source := range result.Sources {
		sources = append(sources, entity.DemoSource{
			ID:    source.ID,
			Title: source.Title,
			Text:  source.Text,
		})
	}
	return &entity.DemosRetrievalRsp{
		Execution: agentResult(result.Execution),
		Sources:   sources,
	}, err
}

// Workflow 演示工作流。 @alias=/api/agent-demos/workflow
func (a *ImplAgentDemos) Workflow(ctx context.Context, req *entity.DemosWorkflowReq) (*entity.DemosWorkflowRsp, error) {
	result, err := a.agentCli.DemoWorkflow(ctx, req.Question)
	stages := make([]entity.DemoStage, 0, len(result.Stages))
	for _, stage := range result.Stages {
		stages = append(stages, entity.DemoStage{
			Name:    stage.Name,
			Content: stage.Content,
		})
	}
	return &entity.DemosWorkflowRsp{
		Execution: agentResult(result.Execution),
		Stages:    stages,
	}, err
}

// Observation 演示日志与观测。 @alias=/api/agent-demos/observation
func (a *ImplAgentDemos) Observation(ctx context.Context, req *entity.DemosObservationReq) (*entity.DemosObservationRsp, error) {
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
	return &entity.DemosObservationRsp{
		Execution: agentResult(result),
	}, err
}

// Evaluation 演示评估与回归。 @alias=/api/agent-demos/evaluation
func (a *ImplAgentDemos) Evaluation(ctx context.Context, req *entity.DemosEvaluationReq) (*entity.DemosEvaluationRsp, error) {
	result, err := a.agentCli.DemoEvaluation(ctx)
	cases := make([]entity.DemoCase, 0, len(result.Cases))
	for _, item := range result.Cases {
		cases = append(cases, entity.DemoCase{
			Question:  item.Question,
			Expected:  item.Expected,
			Passed:    item.Passed,
			Execution: agentResult(item.Execution),
			Error:     item.Error,
		})
	}
	return &entity.DemosEvaluationRsp{
		Cases:  cases,
		Passed: result.Passed,
		Total:  result.Total,
	}, err
}

// Delivery 演示综合练习。 @alias=/api/agent-demos/delivery
func (a *ImplAgentDemos) Delivery(ctx context.Context, req *entity.DemosDeliveryReq) (*entity.DemosDeliveryRsp, error) {
	result, err := a.agentCli.DemoDelivery(ctx, req.Question)
	return &entity.DemosDeliveryRsp{
		Execution: agentResult(result),
	}, err
}

// Stream 返回同步拉取的响应序列；传输层负责 SSE，领域层只接收普通回调。
// 消费者停止读取时回调立即返回取消，不启动后台生产协程。
func (a *ImplAgentDemos) Stream(ctx context.Context, req *entity.DemosStreamReq) (iter.Seq2[*entity.DemosStreamRsp, error], error) {
	question := req.Question
	if question != "" && (len([]rune(question)) > agent.MaxQuestionChars || strings.TrimSpace(question) == "") {
		return nil, agent.ErrInput
	}
	return func(yield func(*entity.DemosStreamRsp, error) bool) {
		stopped := false
		result, err := a.agentCli.DemoStream(ctx, agent.DemoStreamInput{
			Question: question,
			Emit: func(text string) error {
				if stopped || !yield(&entity.DemosStreamRsp{
					Delta: text,
				}, nil) {
					stopped = true
					return context.Canceled
				}
				return nil
			},
		})
		if !stopped {
			yield(&entity.DemosStreamRsp{
				Execution: agentResult(result),
				Done:      true,
			}, err)
		}
	}, nil
}
