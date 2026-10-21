package agent

import (
	"context"
	"errors"
	"testing"
)

func TestDomainValidationReturnsNoResult(t *testing.T) {
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (*Response, error) {
		t.Fatal("无效输入不应调用模型")
		return nil, nil
	}), DefaultLimits())
	chat, err := agentCli.Chat(t.Context(), ChatReq{})
	if chat != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际聊天结果 %+v、错误 %v，预期 nil 和 ErrInput", chat, err)
	}
	messages, err := agentCli.Messages(t.Context(), MessagesInput{
		Questions: []string{},
	})
	if messages != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际历史结果 %+v、错误 %v，预期 nil 和 ErrInput", messages, err)
	}
	structured, err := agentCli.DemoStructured(t.Context(), " ")
	if structured != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际结构化结果 %+v、错误 %v，预期 nil 和 ErrInput", structured, err)
	}
	retrieval, err := agentCli.DemoRetrieval(t.Context(), " ")
	if retrieval != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际检索结果 %+v、错误 %v，预期 nil 和 ErrInput", retrieval, err)
	}
	workflow, err := agentCli.DemoWorkflow(t.Context(), " ")
	if workflow != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际工作流结果 %+v、错误 %v，预期 nil 和 ErrInput", workflow, err)
	}
	stream, err := agentCli.DemoStream(t.Context(), DemoStreamInput{})
	if stream != nil || !errors.Is(err, ErrInput) {
		t.Fatalf("实际流式结果 %+v、错误 %v，预期 nil 和 ErrInput", stream, err)
	}
}

func TestNilModelResponseIsProtocolError(t *testing.T) {
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (*Response, error) {
		return nil, nil
	}), DefaultLimits())
	result, err := agentCli.Chat(t.Context(), ChatReq{
		Question: "问题",
	})
	if result == nil || !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("实际执行结果 %+v、错误 %v，预期保留失败尝试并返回协议错误", result, err)
	}
	if result.Completed || result.ModelCalls != 1 || len(result.Calls) != 1 || result.Calls[0].Error == "" {
		t.Fatalf("空模型响应丢失了调用记录：%+v", result)
	}
}

// nilResultTools 验证工具返回 nil 且不报错时的协议防护。
type nilResultTools struct{}

// Definitions 返回正常的乘法声明，使执行进入工具调用。
func (nilResultTools) Definitions() []ToolDefinition {
	return MultiplyTools{}.Definitions()
}

// Dispatch 模拟错误实现遗漏工具结果和错误。
func (nilResultTools) Dispatch(context.Context, ToolCallReq) (*ToolCallRsp, error) {
	return nil, nil
}

func TestNilToolResultIsProtocolError(t *testing.T) {
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (*Response, error) {
		return &Response{
			Message: Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCallReq{
					{
						ID:   "one",
						Type: ToolTypeFunction,
						Function: ToolFunctionCall{
							Name:      ToolNameMultiply,
							Arguments: `{"a":2,"b":3}`,
						},
					},
				},
			},
			FinishReason: FinishReasonToolCalls,
		}, nil
	}), DefaultLimits())
	result, err := agentCli.Chat(t.Context(), ChatReq{
		Question: "问题",
		Tools:    nilResultTools{},
	})
	if result == nil || !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("实际执行结果 %+v、错误 %v，预期空工具结果转换为协议错误", result, err)
	}
	if result.Completed || result.ToolCalls != 1 || len(result.Calls) != 2 || result.Calls[1].Error == "" {
		t.Fatalf("空工具结果丢失了执行记录：%+v", result)
	}
}

// unencodableResultTools 模拟工具已经执行但返回无法编码的值。
type unencodableResultTools struct{}

// Definitions 返回正常的工具声明。
func (unencodableResultTools) Definitions() []ToolDefinition {
	return MultiplyTools{}.Definitions()
}

// Dispatch 返回不可编码的结果，用于检查调用记录是否保留。
func (unencodableResultTools) Dispatch(context.Context, ToolCallReq) (*ToolCallRsp, error) {
	return &ToolCallRsp{
		OK:     true,
		Result: make(chan int),
	}, nil
}

func TestToolEncodingFailureKeepsCallRecord(t *testing.T) {
	result := NewChatRsp(OperationRun)
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (*Response, error) {
		t.Fatal("直接测试工具执行时不应调用模型")
		return nil, nil
	}), DefaultLimits())
	run, err := agentCli.prepareRun(result, RunInput{
		Questions: []string{"问题"},
	})
	if err != nil {
		t.Fatal(err)
	}
	message, err := run.callTool(t.Context(), unencodableResultTools{}, ToolCallReq{
		ID:   "one",
		Type: ToolTypeFunction,
		Function: ToolFunctionCall{
			Name:      ToolNameMultiply,
			Arguments: `{"a":2,"b":3}`,
		},
	})
	if message != nil || err == nil || result.ToolCalls != 1 || len(result.Calls) != 1 {
		t.Fatalf("实际消息 %+v、错误 %v、结果 %+v，预期没有回填消息但保留一次失败记录", message, err, result)
	}
	if result.Calls[0].Kind != CallKindTool || result.Calls[0].Request == "" || result.Calls[0].Error == "" {
		t.Fatalf("工具编码失败丢失了请求或错误说明：%+v", result.Calls[0])
	}
}
