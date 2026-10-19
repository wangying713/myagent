package app

import (
	"context"
	"errors"
	"iter"
	"strings"

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"
)

// ImplAgentDemos 为学习页面的每个演示准备输入并组装独立响应，业务仍由领域层执行。
type ImplAgentDemos struct {
	client *agent.Client // 与旧 API 复用的 Agent 客户端，不复制模型客户端或配置。
}

func newImplAgentDemos(client *agent.Client) *ImplAgentDemos {
	return &ImplAgentDemos{client: client}
}

// Overview 演示Agent 到底是什么。 @alias=/api/agent-demos/overview
func (a *ImplAgentDemos) Overview(ctx context.Context, req *proto.DemosOverviewReq) (*proto.DemosOverviewRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoOverview(ctx, agent.ChatInput{Question: question})
	return &proto.DemosOverviewRsp{
		Direct:    demoExecution(result.Direct),
		WithTools: demoExecution(result.WithTools),
	}, err
}

// Chat 演示普通聊天。 @alias=/api/agent-demos/chat
func (a *ImplAgentDemos) Chat(ctx context.Context, req *proto.DemosChatReq) (*proto.DemosChatRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.Chat(ctx, agent.ChatInput{Question: question})
	return &proto.DemosChatRsp{Execution: demoExecution(result)}, err
}

// Messages 演示消息历史。 @alias=/api/agent-demos/messages
func (a *ImplAgentDemos) Messages(ctx context.Context, req *proto.DemosMessagesReq) (*proto.DemosMessagesRsp, error) {
	questions := req.GetQuestions()
	result, err := a.client.Messages(ctx, agent.MessagesInput{Questions: questions})
	return &proto.DemosMessagesRsp{Execution: demoExecution(result)}, err
}

// Tools 演示工具调用与循环。 @alias=/api/agent-demos/tools
func (a *ImplAgentDemos) Tools(ctx context.Context, req *proto.DemosToolsReq) (*proto.DemosToolsRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.Run(ctx, agent.RunInput{Question: question})
	return &proto.DemosToolsRsp{Execution: demoExecution(result)}, err
}

// Structured 演示结构化输出。 @alias=/api/agent-demos/structured
func (a *ImplAgentDemos) Structured(ctx context.Context, req *proto.DemosStructuredReq) (*proto.DemosStructuredRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoStructured(ctx, agent.ChatInput{Question: question})
	return &proto.DemosStructuredRsp{
		Execution: demoExecution(result.Execution),
		Product:   result.Order.Product,
		Quantity:  int64(result.Order.Quantity),
	}, err
}

// MultipleTools 演示多个工具协作。 @alias=/api/agent-demos/multiple-tools
func (a *ImplAgentDemos) MultipleTools(ctx context.Context, req *proto.DemosMultipleToolsReq) (*proto.DemosMultipleToolsRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoMultipleTools(ctx, agent.ChatInput{Question: question})
	return &proto.DemosMultipleToolsRsp{Execution: demoExecution(result)}, err
}

// Reliability 演示失败与执行边界。 @alias=/api/agent-demos/reliability
func (a *ImplAgentDemos) Reliability(ctx context.Context, req *proto.DemosReliabilityReq) (*proto.DemosReliabilityRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	calls := int64(1)
	if req.ModelCalls != nil {
		calls = req.GetModelCalls()
	}
	if calls < 1 || calls > int64(agent.DefaultLimits().ModelCalls) {
		return nil, agent.ErrInput
	}
	result, err := a.client.DemoReliability(ctx, agent.DemoReliabilityInput{
		Question:   question,
		ModelCalls: int(calls),
	})
	reason := "已完成，未触发次数限制"
	if errors.Is(err, agent.ErrBudgetExceeded) {
		reason = "已达到本次模型调用次数上限；前面的执行记录仍然保留"
	} else if err != nil {
		reason = "执行未完成，请查看错误与步骤"
	}
	return &proto.DemosReliabilityRsp{
		Execution:  demoExecution(result),
		StopReason: reason,
	}, err
}

// Retrieval 演示检索与记忆。 @alias=/api/agent-demos/retrieval
func (a *ImplAgentDemos) Retrieval(ctx context.Context, req *proto.DemosRetrievalReq) (*proto.DemosRetrievalRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoRetrieval(ctx, agent.ChatInput{Question: question})
	sources := make([]*proto.DemosSource, 0, len(result.Sources))
	for _, source := range result.Sources {
		sources = append(sources, &proto.DemosSource{
			Id:    source.ID,
			Title: source.Title,
			Text:  source.Text,
		})
	}
	return &proto.DemosRetrievalRsp{
		Execution: demoExecution(result.Execution),
		Sources:   sources,
	}, err
}

// Workflow 演示工作流。 @alias=/api/agent-demos/workflow
func (a *ImplAgentDemos) Workflow(ctx context.Context, req *proto.DemosWorkflowReq) (*proto.DemosWorkflowRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoWorkflow(ctx, agent.ChatInput{Question: question})
	stages := make([]*proto.DemosStage, 0, len(result.Stages))
	for _, stage := range result.Stages {
		stages = append(stages, &proto.DemosStage{
			Name:    stage.Name,
			Content: stage.Content,
		})
	}
	return &proto.DemosWorkflowRsp{
		Execution: demoExecution(result.Execution),
		Stages:    stages,
	}, err
}

// Observation 演示日志与观测。 @alias=/api/agent-demos/observation
func (a *ImplAgentDemos) Observation(ctx context.Context, req *proto.DemosObservationReq) (*proto.DemosObservationRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoObservation(ctx, agent.RunInput{Question: question})
	return &proto.DemosObservationRsp{Execution: demoExecution(result)}, err
}

// Evaluation 演示评估与回归。 @alias=/api/agent-demos/evaluation
func (a *ImplAgentDemos) Evaluation(ctx context.Context, req *proto.DemosEvaluationReq) (*proto.DemosEvaluationRsp, error) {
	result, err := a.client.DemoEvaluation(ctx)
	cases := make([]*proto.DemosCase, 0, len(result.Cases))
	for _, item := range result.Cases {
		cases = append(cases, &proto.DemosCase{
			Question:  item.Question,
			Expected:  item.Expected,
			Passed:    item.Passed,
			Execution: demoExecution(item.Execution),
			Error:     item.Error,
		})
	}
	return &proto.DemosEvaluationRsp{
		Cases:  cases,
		Passed: int64(result.Passed),
		Total:  int64(result.Total),
	}, err
}

// Delivery 演示综合练习。 @alias=/api/agent-demos/delivery
func (a *ImplAgentDemos) Delivery(ctx context.Context, req *proto.DemosDeliveryReq) (*proto.DemosDeliveryRsp, error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	result, err := a.client.DemoDelivery(ctx, agent.ChatInput{Question: question})
	return &proto.DemosDeliveryRsp{Execution: demoExecution(result)}, err
}

// Stream 返回同步拉取的响应序列；传输层负责 SSE，领域层只接收普通回调。
// 消费者停止读取时回调立即返回取消，不启动后台生产协程。
func (a *ImplAgentDemos) Stream(ctx context.Context, req *proto.DemosStreamReq) (iter.Seq2[*proto.DemosStreamRsp, error], error) {
	question, err := agentQuestionInput(req.Question)
	if err != nil {
		return nil, err
	}
	if question != "" && (len([]rune(question)) > agent.MaxQuestionChars || strings.TrimSpace(question) == "") {
		return nil, agent.ErrInput
	}
	return func(yield func(*proto.DemosStreamRsp, error) bool) {
		stopped := false
		result, err := a.client.DemoStream(ctx, agent.DemoStreamInput{
			Question: question,
			Emit: func(text string) error {
				if stopped || !yield(&proto.DemosStreamRsp{Delta: text}, nil) {
					stopped = true
					return context.Canceled
				}
				return nil
			},
		})
		if !stopped {
			yield(&proto.DemosStreamRsp{
				Execution: demoExecution(result),
				Done:      true,
			}, err)
		}
	}, nil
}

// demoExecution 只转换共用的执行记录，各业务响应仍由对应方法分别组装。
func demoExecution(result agent.ExecutionResult) *proto.DemosExecution {
	return &proto.DemosExecution{
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
	}
}
