package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, chatCli ChatClient, limits Limits) *Client {
	t.Helper()
	agentCli, err := NewClient(chatCli, limits)
	if err != nil {
		t.Fatal(err)
	}
	return agentCli
}

func TestClientChatRecordsOriginalResult(t *testing.T) {
	chatCli := modelFunc(func(ctx context.Context, req Request) (*Response, error) {
		if len(req.Messages) != 2 || req.Messages[0].Content != "按调用方的要求回答" || req.Messages[1].Content != "  调用方的问题  " {
			t.Fatalf("实际请求 %+v，预期保留调用方的问题和提示词", req.Messages)
		}
		return &Response{
			Message: Message{
				Role:    "assistant",
				Content: "回答包含本机密钥",
			},
			FinishReason: "stop",
			Raw:          []byte(`{"answer":"本机密钥"}`),
		}, nil
	})
	result, err := newTestClient(t, chatCli, DefaultLimits()).Chat(context.Background(), ChatReq{
		Question: "  调用方的问题  ",
		Prompt:   "按调用方的要求回答",
	})
	if err != nil || !result.Completed || result.Answer != "回答包含本机密钥" || result.ModelCalls != 1 || len(result.Calls) != 1 || result.UsageComplete {
		t.Fatalf("聊天结果不符合契约：%+v，错误 %v", result, err)
	}
	if result.Calls[0].Response != `{"answer":"本机密钥"}` {
		t.Fatalf("领域记录应保留原始响应，实际 %+v", result.Calls[0])
	}
}

func TestClientMessagesPreservesHistoryAndPartialAnswer(t *testing.T) {
	for _, limited := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "完整两轮",
			true:  "预算停止",
		}[limited], func(t *testing.T) {
			calls := 0
			chatCli := modelFunc(func(ctx context.Context, req Request) (*Response, error) {
				calls++
				answer := "记住了豆包。"
				if calls == 2 {
					if len(req.Messages) != 4 || req.Messages[2].Content != "记住了豆包。" || req.Messages[3].Content != SecondQuestion {
						t.Fatalf("第二轮丢失真实历史：%+v", req.Messages)
					}
					answer = "豆包"
				}
				return &Response{
					Message: Message{
						Role:    "assistant",
						Content: answer,
					},
					FinishReason: "stop",
				}, nil
			})
			limits := DefaultLimits()
			if limited {
				limits.ModelCalls = 1
			}
			result, err := newTestClient(t, chatCli, limits).Messages(context.Background(), MessagesInput{})
			if limited {
				if !errors.Is(err, ErrBudgetExceeded) || result.Completed || result.Answer != "记住了豆包。" || calls != 1 {
					t.Fatalf("预算停止丢失部分结果：%+v，错误 %v", result, err)
				}
			} else if err != nil || !result.Completed || result.Answer != "豆包" || calls != 2 {
				t.Fatalf("两轮执行错误：%+v，错误 %v", result, err)
			}
		})
	}
}

func TestClientChatRecordsTool(t *testing.T) {
	calls := 0
	chatCli := modelFunc(func(ctx context.Context, req Request) (*Response, error) {
		calls++
		if calls == 1 {
			return &Response{
				Message: Message{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{
							ID:   "one",
							Type: "function",
							Function: FunctionCall{
								Name:      "multiply",
								Arguments: `{"a":17,"b":23}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			}, nil
		}
		if len(req.Messages) != 4 || req.Messages[3].ToolCallID != "one" || !strings.Contains(req.Messages[3].Content, "391") {
			t.Fatalf("未回填工具结果：%+v", req.Messages)
		}
		return &Response{
			Message: Message{
				Role:    "assistant",
				Content: "391",
			},
			FinishReason: "stop",
		}, nil
	})
	result, err := newTestClient(t, chatCli, DefaultLimits()).Chat(t.Context(), ChatReq{
		Operation: OperationRun,
		Tools:     MultiplyTools{},
		Question:  AgentQuestion,
		Prompt:    AgentPrompt,
	})
	if err != nil || !result.Completed || result.ModelCalls != 2 || result.ToolCalls != 1 || len(result.Calls) != 3 || result.Calls[1].Kind != "tool" {
		t.Fatalf("工具执行记录错误：%+v，错误 %v", result, err)
	}
	if !strings.Contains(result.Calls[1].Response, `"result":391`) {
		t.Fatalf("实际工具调用记录 %+v，预期保留乘法结果", result.Calls[1])
	}
}

func TestClientInputAndCancellation(t *testing.T) {
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (*Response, error) {
		t.Fatal("无效输入或取消后不应调用模型")
		return nil, nil
	}), DefaultLimits())
	for _, question := range []string{
		"  ",
		strings.Repeat("字", 4097),
	} {
		if _, err := agentCli.Chat(context.Background(), ChatReq{
			Question: question,
		}); !errors.Is(err, ErrInput) {
			t.Errorf("聊天实际错误 %v，预期 ErrInput", err)
		}
		if _, err := agentCli.Chat(t.Context(), ChatReq{
			Operation: OperationRun,
			Tools:     MultiplyTools{},
			Question:  question,
			Prompt:    AgentPrompt,
		}); !errors.Is(err, ErrInput) {
			t.Errorf("工具运行实际错误 %v，预期 ErrInput", err)
		}
	}
	for _, questions := range [][]string{
		{},
		{"一轮"},
		{
			" ",
			"第二轮",
		},
		{
			"第一轮",
			"",
		},
	} {
		if _, err := agentCli.Messages(context.Background(), MessagesInput{
			Questions: questions,
		}); !errors.Is(err, ErrInput) {
			t.Errorf("历史消息实际错误 %v，预期 ErrInput", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := agentCli.Chat(ctx, ChatReq{
		Question: "这是一次已取消的问题",
	})
	if !errors.Is(err, context.Canceled) || result.ModelCalls != 0 || result.Completed {
		t.Fatalf("取消执行错误：%+v，错误 %v", result, err)
	}
}
