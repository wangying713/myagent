package app

import (
	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/apps/interfaces/http/proto"
)

// demosExecutionJSON 保留执行记录的零值、null 与用量是否已知。
func demosExecutionJSON(rsp *proto.DemosExecution) *entity.AgentExecutionResult {
	if rsp == nil {
		return nil
	}
	return &entity.AgentExecutionResult{
		Lesson:           rsp.Lesson,
		Completed:        rsp.Completed,
		Answer:           rsp.Answer,
		TelemetryEnabled: rsp.TelemetryEnabled,
		ElapsedMS:        rsp.ElapsedMs,
		ModelCalls:       rsp.ModelCalls,
		ToolCalls:        rsp.ToolCalls,
		KnownTokens:      rsp.KnownTokens,
		UsageComplete:    rsp.UsageComplete,
		Steps:            agentStepsJSON(rsp.Steps),
	}
}

// demosOverviewRspJSON 保留Agent 到底是什么的字段与零值。
func demosOverviewRspJSON(rsp *proto.DemosOverviewRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosOverviewRsp{
		Direct:    demosExecutionJSON(rsp.Direct),
		WithTools: demosExecutionJSON(rsp.WithTools),
	}
}

// demosChatRspJSON 保留普通聊天的字段与零值。
func demosChatRspJSON(rsp *proto.DemosChatRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosChatRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}

// demosMessagesRspJSON 保留消息历史的字段与零值。
func demosMessagesRspJSON(rsp *proto.DemosMessagesRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosMessagesRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}

// demosToolsRspJSON 保留工具调用与循环的字段与零值。
func demosToolsRspJSON(rsp *proto.DemosToolsRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosToolsRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}

// demosStructuredRspJSON 保留结构化输出的字段与零值。
func demosStructuredRspJSON(rsp *proto.DemosStructuredRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosStructuredRsp{
		Execution: demosExecutionJSON(rsp.Execution),
		Product:   rsp.Product,
		Quantity:  rsp.Quantity,
	}
}

// demosMultipleToolsRspJSON 保留多个工具协作的字段与零值。
func demosMultipleToolsRspJSON(rsp *proto.DemosMultipleToolsRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosMultipleToolsRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}

// demosStreamRspJSON 保留流式回答的字段与零值。
func demosStreamRspJSON(rsp *proto.DemosStreamRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosStreamRsp{
		Execution: demosExecutionJSON(rsp.Execution),
		Delta:     rsp.Delta,
		Done:      rsp.Done,
	}
}

// demosReliabilityRspJSON 保留失败与执行边界的字段与零值。
func demosReliabilityRspJSON(rsp *proto.DemosReliabilityRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosReliabilityRsp{
		Execution:  demosExecutionJSON(rsp.Execution),
		StopReason: rsp.StopReason,
	}
}

// demosRetrievalRspJSON 保留检索与记忆的字段与零值。
func demosRetrievalRspJSON(rsp *proto.DemosRetrievalRsp) any {
	if rsp == nil {
		return nil
	}
	sources := make([]entity.DemoSource, 0, len(rsp.Sources))
	for _, item := range rsp.Sources {
		sources = append(sources, entity.DemoSource{
			ID:    item.GetId(),
			Title: item.GetTitle(),
			Text:  item.GetText(),
		})
	}
	return entity.DemosRetrievalRsp{
		Execution: demosExecutionJSON(rsp.Execution),
		Sources:   sources,
	}
}

// demosWorkflowRspJSON 保留工作流的字段与零值。
func demosWorkflowRspJSON(rsp *proto.DemosWorkflowRsp) any {
	if rsp == nil {
		return nil
	}
	stages := make([]entity.DemoStage, 0, len(rsp.Stages))
	for _, item := range rsp.Stages {
		stages = append(stages, entity.DemoStage{
			Name:    item.GetName(),
			Content: item.GetContent(),
		})
	}
	return entity.DemosWorkflowRsp{
		Execution: demosExecutionJSON(rsp.Execution),
		Stages:    stages,
	}
}

// demosObservationRspJSON 保留日志与观测的字段与零值。
func demosObservationRspJSON(rsp *proto.DemosObservationRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosObservationRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}

// demosEvaluationRspJSON 保留评估与回归的字段与零值。
func demosEvaluationRspJSON(rsp *proto.DemosEvaluationRsp) any {
	if rsp == nil {
		return nil
	}
	cases := make([]entity.DemoCase, 0, len(rsp.Cases))
	for _, item := range rsp.Cases {
		cases = append(cases, entity.DemoCase{
			Question:  item.GetQuestion(),
			Expected:  item.GetExpected(),
			Passed:    item.GetPassed(),
			Execution: demosExecutionJSON(item.GetExecution()),
			Error:     item.GetError(),
		})
	}
	return entity.DemosEvaluationRsp{
		Cases:  cases,
		Passed: rsp.Passed,
		Total:  rsp.Total,
	}
}

// demosDeliveryRspJSON 保留综合练习的字段与零值。
func demosDeliveryRspJSON(rsp *proto.DemosDeliveryRsp) any {
	if rsp == nil {
		return nil
	}
	return entity.DemosDeliveryRsp{
		Execution: demosExecutionJSON(rsp.Execution),
	}
}
