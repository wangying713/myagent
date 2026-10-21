package app

import (
	"context"
	"errors"
	"testing"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"
)

func TestAgentServiceValidationReturnsNoResult(t *testing.T) {
	service := &agentService{
		limits: agent.DefaultLimits(),
	}
	for _, req := range []agent.ChatReq{
		{
			Question: " ",
		},
		{
			Question:   "有效问题",
			ToolChoice: "unknown",
		},
	} {
		result, err := service.Chat(t.Context(), req)
		if result != nil || !errors.Is(err, agent.ErrInput) {
			t.Fatalf("实际结果 %+v、错误 %v，预期输入校验失败返回 nil 和 ErrInput", result, err)
		}
	}
	for _, questions := range [][]string{
		{},
		{"只有一轮"},
		{
			"第一轮有效",
			" ",
		},
	} {
		result, err := service.Messages(t.Context(), agent.MessagesInput{
			Questions: questions,
		})
		if result != nil || !errors.Is(err, agent.ErrInput) {
			t.Fatalf("实际消息结果 %+v、错误 %v，预期 nil 和 ErrInput", result, err)
		}
	}
	structured, err := service.DemoStructured(t.Context(), " ")
	if structured != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际结构化结果 %+v、错误 %v，预期 nil 和 ErrInput", structured, err)
	}
	retrieval, err := service.DemoRetrieval(t.Context(), " ")
	if retrieval != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际检索结果 %+v、错误 %v，预期 nil 和 ErrInput", retrieval, err)
	}
	workflow, err := service.DemoWorkflow(t.Context(), " ")
	if workflow != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际工作流结果 %+v、错误 %v，预期 nil 和 ErrInput", workflow, err)
	}
	stream, err := service.DemoStream(t.Context(), agent.DemoStreamInput{})
	if stream != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际流式结果 %+v、错误 %v，预期 nil 和 ErrInput", stream, err)
	}
	overview, err := service.DemoOverview(t.Context(), " ")
	if overview != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际对比结果 %+v、错误 %v，预期 nil 和 ErrInput", overview, err)
	}
	reliability, err := service.DemoReliability(t.Context(), demoReliabilityInput{})
	if reliability != nil || !errors.Is(err, agent.ErrInput) {
		t.Fatalf("实际预算案例结果 %+v、错误 %v，预期 nil 和 ErrInput", reliability, err)
	}
	if service.agentResult(nil) != nil {
		t.Fatal("nil 执行结果不应组装成 HTTP 数据")
	}
}

// resultModelFunc 提供返回完整或部分结果的离线模型替身。
type resultModelFunc func(context.Context, agent.Request) (*agent.Response, error)

// Chat 按用例指定的顺序返回模型响应或执行错误。
func (f resultModelFunc) Chat(ctx context.Context, req agent.Request) (*agent.Response, error) {
	return f(ctx, req)
}

// newResultTestService 组装不连接外部服务的领域客户端和观测客户端。
func newResultTestService(t *testing.T, chatCli agent.ChatClient) *agentService {
	t.Helper()
	observeCli, err := openobserve.New(t.Context(), openobserve.Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := observeCli.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	limits := agent.DefaultLimits()
	agentCli, err := agent.NewClient(chatCli, limits)
	if err != nil {
		t.Fatal(err)
	}
	service, err := newAgentService(agentCli, observeCli, openobserve.ModelInfo{}, limits)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestAgentServiceKeepsPartialResultWithError(t *testing.T) {
	calls := 0
	service := newResultTestService(t, resultModelFunc(func(context.Context, agent.Request) (*agent.Response, error) {
		calls++
		if calls == 2 {
			return nil, context.DeadlineExceeded
		}
		return &agent.Response{
			Message: agent.Message{
				Role:    agent.RoleAssistant,
				Content: "第一轮已完成",
			},
			FinishReason: agent.FinishReasonStop,
		}, nil
	}))
	result, err := service.Messages(t.Context(), agent.MessagesInput{})
	if result == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("实际结果 %+v、错误 %v，预期保留结果并返回超时", result, err)
	}
	if result.Completed || result.Answer != "第一轮已完成" || result.ModelCalls != 2 || len(result.Calls) != 2 {
		t.Fatalf("实际部分结果 %+v，预期第一轮回答和两次调用记录", result)
	}
	if service.agentResult(result) == nil {
		t.Fatal("执行错误不应丢弃已有 HTTP 数据")
	}
}

func TestCompositeDemoKeepsStartedExecutionOnError(t *testing.T) {
	service := newResultTestService(t, resultModelFunc(func(context.Context, agent.Request) (*agent.Response, error) {
		return nil, context.DeadlineExceeded
	}))
	result, err := service.DemoOverview(t.Context(), "有效问题")
	if result == nil || result.Direct == nil || result.WithTools != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("实际对比结果 %+v、错误 %v，预期保留已开始的执行且未开始的执行为 nil", result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	evaluation, err := service.DemoEvaluation(ctx)
	if evaluation != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("实际评估结果 %+v、错误 %v，预期未执行时返回 nil 和取消错误", evaluation, err)
	}
}
