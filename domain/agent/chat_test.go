package agent

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

type modelFunc func(context.Context, Request) (*Response, error)

func (f modelFunc) Chat(ctx context.Context, req Request) (*Response, error) {
	return f(ctx, req)
}

func TestChatChoosesToolPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string // 是否提供工具以及是否明确指定策略。
		tools      Tools  // 本次对话提供的工具，nil 表示不提供。
		choice     string // 调用方指定的策略，空值使用默认行为。
		wantChoice string // 实际发送给模型并记录的工具策略。
	}{
		{
			name:       "不提供工具",
			wantChoice: ToolChoiceNone,
		},
		{
			name:       "没有工具时不能自动调用",
			choice:     ToolChoiceAuto,
			wantChoice: ToolChoiceNone,
		},
		{
			name:       "提供工具后默认允许选择",
			tools:      MultiplyTools{},
			wantChoice: ToolChoiceAuto,
		},
		{
			name:       "明确允许选择工具",
			tools:      MultiplyTools{},
			choice:     ToolChoiceAuto,
			wantChoice: ToolChoiceAuto,
		},
		{
			name:       "提供工具但明确禁用",
			tools:      MultiplyTools{},
			choice:     ToolChoiceNone,
			wantChoice: ToolChoiceNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			chatCli := modelFunc(func(_ context.Context, req Request) (*Response, error) {
				calls++
				if len(req.Messages) != 1 || req.Messages[0].Role != RoleUser || req.Messages[0].Content != "打个招呼" {
					t.Fatalf("未提供提示词时应只发送用户问题，实际 %+v", req.Messages)
				}
				if req.ToolChoice != tc.wantChoice || (len(req.Tools) > 0) != (tc.tools != nil) {
					t.Fatalf("实际策略 %q、工具 %d 个，预期策略 %q、提供工具 %v", req.ToolChoice, len(req.Tools), tc.wantChoice, tc.tools != nil)
				}
				return &Response{
					Message: Message{
						Role:    RoleAssistant,
						Content: "直接回答",
					},
					FinishReason: FinishReasonStop,
				}, nil
			})
			agentCli := newTestClient(t, chatCli, DefaultLimits())
			result, err := agentCli.Chat(t.Context(), ChatReq{
				Question:   "打个招呼",
				Tools:      tc.tools,
				ToolChoice: tc.choice,
			})
			if err != nil || !result.Completed || result.Answer != "直接回答" || calls != 1 || result.ModelCalls != 1 || result.ToolCalls != 0 {
				t.Fatalf("普通回答未正确结束：%+v，错误 %v，模型请求 %d 次", result, err, calls)
			}
			if result.Input.ToolChoice != tc.wantChoice {
				t.Fatal("运行记录没有保存实际工具策略或没有收尾")
			}
		})
	}
}

func TestChatRejectsToolCallsWhenDisabled(t *testing.T) {
	for _, tc := range []struct {
		name  string // 没有工具或明确禁止工具的场景。
		tools Tools  // 本次提供的工具，nil 表示没有工具。
	}{
		{
			name: "没有提供工具",
		},
		{
			name:  "提供工具但禁止调用",
			tools: MultiplyTools{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
				calls++
				return &Response{
					Message: Message{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{
								ID:   "unexpected-tool",
								Type: ToolTypeFunction,
								Function: FunctionCall{
									Name:      ToolNameMultiply,
									Arguments: `{"a":2,"b":3}`,
								},
							},
						},
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			})
			agentCli := newTestClient(t, chatCli, DefaultLimits())
			result, err := agentCli.Chat(t.Context(), ChatReq{
				Question:   "2 乘 3",
				Tools:      tc.tools,
				ToolChoice: ToolChoiceNone,
			})
			if !errors.Is(err, ErrInvalidResponse) || result.Completed || calls != 1 || result.ToolCalls != 0 {
				t.Fatalf("禁用后仍继续执行：%+v，错误 %v，模型请求 %d 次", result, err, calls)
			}
		})
	}
}

func TestChatRejectsUnknownToolChoiceBeforeModelCall(t *testing.T) {
	chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
		t.Fatal("工具策略无效时不应调用模型")
		return nil, nil
	})
	agentCli := newTestClient(t, chatCli, DefaultLimits())
	_, err := agentCli.Chat(t.Context(), ChatReq{
		Question:   "检查无效的工具策略",
		ToolChoice: "unknown",
	})
	if !errors.Is(err, ErrInput) {
		t.Fatalf("实际错误 %v，预期 ErrInput", err)
	}
}

func TestChatPreservesAssistantAndMultipleToolResults(t *testing.T) {
	calls := 0
	chatCli := modelFunc(func(ctx context.Context, req Request) (*Response, error) {
		calls++
		if calls == 1 {
			if len(req.Messages) != 2 || req.Messages[0].Role != RoleSystem || req.Messages[0].Content != "计算助手" || req.Messages[1].Role != RoleUser || req.Messages[1].Content != "问题" {
				t.Fatalf("首轮消息未保留提示词和问题：%+v", req.Messages)
			}
			raw := json.RawMessage(`{"role":"assistant","content":null,"extra":"保留原始扩展","tool_calls":[{"id":"one","type":"function","function":{"name":"multiply","arguments":"{\"a\":17,\"b\":23}"}},{"id":"two","type":"function","function":{"name":"multiply","arguments":"{\"a\":2,\"b\":3}"}}]}`)
			var message Message
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			message.Raw = raw
			return &Response{
				Message:      message,
				FinishReason: "tool_calls",
			}, nil
		}
		if len(req.Messages) != 5 || req.Messages[3].ToolCallID != "one" || req.Messages[4].ToolCallID != "two" {
			t.Fatalf("工具回填与 ID 不匹配：%+v", req.Messages)
		}
		body, err := json.Marshal(req.Messages[2])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"content":null`) || !strings.Contains(string(body), `"extra"`) {
			t.Fatalf("assistant 原始字段丢失：%s", body)
		}
		for i, want := range []float64{
			391,
			6,
		} {
			var out ToolResult
			if err := json.Unmarshal([]byte(req.Messages[i+3].Content), &out); err != nil {
				t.Fatal(err)
			}
			if !out.OK || out.Result != want {
				t.Fatalf("实际工具结果 %v，预期 %v", out, want)
			}
		}
		return &Response{
			Message: Message{
				Role:    "assistant",
				Content: "391 和 6 本机密钥",
			},
			FinishReason: "stop",
		}, nil
	})
	agentCli := newTestClient(t, chatCli, DefaultLimits())
	result, err := agentCli.Chat(t.Context(), ChatReq{
		Operation: OperationRun,
		Tools:     MultiplyTools{},
		Question:  "问题",
		Prompt:    "计算助手",
	})
	if err != nil || result.Answer != "391 和 6 本机密钥" || calls != 2 {
		t.Fatalf("运行结果 %+v/%v，调用 %d 次", result, err, calls)
	}
	if !result.Completed || result.ModelCalls != 2 || result.ToolCalls != 2 || len(result.Calls) != 4 {
		t.Fatalf("模型和工具调用未统一记录：%+v", result)
	}

}

func TestChatRejectsInvalidQuestionsWithoutCallingModelOrTools(t *testing.T) {
	for _, tc := range []struct {
		name     string // 直接调用工具循环时的无效问题类型。
		question string // 业务入口没有预先检查的问题。
	}{
		{
			name: "空问题不能自动换成示例",
		},
		{
			name:     "只有空白",
			question: " \t\n",
		},
		{
			name:     "超过字符上限",
			question: strings.Repeat("字", MaxQuestionChars+1),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
				t.Fatal("问题无效时不应调用模型")
				return nil, nil
			})
			agentCli := newTestClient(t, chatCli, DefaultLimits())
			result, err := agentCli.Chat(t.Context(), ChatReq{
				Operation: OperationRun,
				Tools:     MultiplyTools{},
				Question:  tc.question,
			})
			if !errors.Is(err, ErrInput) || result != nil {
				t.Fatalf("实际结果 %+v、错误 %v，预期 ErrInput 且不启动运行记录", result, err)
			}
		})
	}
}

func TestChatKeepsBudgetOverridesLocal(t *testing.T) {
	chatCli := modelFunc(func(_ context.Context, req Request) (*Response, error) {
		if len(req.Messages) == 2 {
			return &Response{
				Message: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{
							ID:   "multiply-one",
							Type: ToolTypeFunction,
							Function: FunctionCall{
								Name:      ToolNameMultiply,
								Arguments: `{"a":2,"b":3}`,
							},
						},
					},
				},
				FinishReason: FinishReasonToolCalls,
			}, nil
		}
		return &Response{
			Message: Message{
				Role:    RoleAssistant,
				Content: "6",
			},
			FinishReason: FinishReasonStop,
		}, nil
	})
	limits := DefaultLimits()
	limits.ModelCalls = 2
	agentCli := newTestClient(t, chatCli, limits)
	limited, err := agentCli.Chat(t.Context(), ChatReq{
		Operation: DemoReliability,
		Prompt:    AgentPrompt,
		Question:  "2 乘 3",
		Tools:     MultiplyTools{},
		Limits: Limits{
			ModelCalls: 1,
			ToolCalls:  limits.ToolCalls,
			Tokens:     limits.Tokens,
		},
	})
	if !errors.Is(err, ErrBudgetExceeded) || limited.Completed || limited.ModelCalls != 1 || limited.ToolCalls != 1 {
		t.Fatalf("单次预算未生效或丢失执行记录：%+v，错误 %v", limited, err)
	}
	result, err := agentCli.Chat(t.Context(), ChatReq{
		Operation: OperationRun,
		Tools:     MultiplyTools{},
		Question:  "2 乘 3",
		Prompt:    AgentPrompt,
	})
	if err != nil || !result.Completed || result.Answer != "6" || result.ModelCalls != 2 || result.ToolCalls != 1 || agentCli.limits != limits {
		t.Fatalf("前一次预算覆盖影响了后续请求：%+v，错误 %v", result, err)
	}
	if limited.Input.Limits.ModelCalls != 1 || result.Input.Limits != limits {
		t.Fatalf("执行记录未保留各自的实际预算：%+v，%+v", limited.Input, result.Input)
	}
}

func TestToolsRejectInvalidArguments(t *testing.T) {
	for _, tc := range []struct {
		args, code string // 工具参数 JSON 与预期拒绝码。
	}{
		{
			`{"a":17,"b":23}`,
			"",
		},
		{
			`{`,
			"invalid_json",
		},
		{
			`[]`,
			"invalid_arguments",
		},
		{
			`{"a":1}`,
			"invalid_arguments",
		},
		{
			`{"a":1,"b":2,"c":3}`,
			"invalid_arguments",
		},
		{
			`{"a":"1","b":2}`,
			"invalid_arguments",
		},
		{
			`{"a":null,"b":2}`,
			"invalid_arguments",
		},
		{
			`{"a":true,"b":2}`,
			"invalid_arguments",
		},
		{
			`{"A":1,"b":2}`,
			"invalid_arguments",
		},
		{
			`{"a":1,"a":2,"b":3}`,
			"invalid_arguments",
		},
		{
			`{"a":1e999,"b":2}`,
			"invalid_arguments",
		},
		{
			`{"a":1e308,"b":1e308}`,
			"tool_execution_failed",
		},
	} {
		t.Run(tc.args, func(t *testing.T) {
			out, err := (MultiplyTools{}).Dispatch(context.Background(), ToolCall{
				Type: "function",
				Function: FunctionCall{
					Name:      "multiply",
					Arguments: tc.args,
				},
			})
			if err != nil || out.Error != tc.code || out.OK != (tc.code == "") {
				t.Fatalf("实际结果 %+v/%v，预期错误码 %q", out, err, tc.code)
			}
		})
	}
	out, err := (MultiplyTools{}).Dispatch(context.Background(), ToolCall{
		Type: "function",
		Function: FunctionCall{
			Name:      "shell",
			Arguments: `{}`,
		},
	})
	if err != nil || out.Error != "unknown_tool" {
		t.Fatalf("未知工具未拒绝：%+v/%v", out, err)
	}
}

func TestMultiplyStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := (MultiplyTools{}).multiply(ctx, multiplyArgs{
		A: 17,
		B: 23,
	})
	if !errors.Is(err, context.Canceled) || out != nil {
		t.Fatalf("取消后的乘法结果为 %+v/%v，预期返回取消错误且无计算结果", out, err)
	}
}

// chatFailureCase 保存预算与协议错误场景的模型输入和预期。
type chatFailureCase struct {
	name   string   // 场景名称。
	limits Limits   // 本次运行预算。
	ids    []string // 模型提出的工具调用 ID。
	tokens int      // 模型报告的单次 token 用量。
	want   error    // 预期错误。
}

func TestChatBudgetsAndProtocolErrors(t *testing.T) {
	for _, tc := range chatFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
				calls++
				tools := []ToolCall{}
				for _, id := range tc.ids {
					tools = append(tools, ToolCall{
						ID:   id,
						Type: "function",
						Function: FunctionCall{
							Name:      "multiply",
							Arguments: `{"a":1,"b":2}`,
						},
					})
				}
				return &Response{
					Message: Message{
						Role:      "assistant",
						ToolCalls: tools,
					},
					FinishReason: "tool_calls",
					Usage: Usage{
						TotalTokens: &tc.tokens,
					},
				}, nil
			})
			agentCli := newTestClient(t, chatCli, DefaultLimits())
			result, err := agentCli.Chat(t.Context(), ChatReq{
				Operation: OperationRun,
				Tools:     MultiplyTools{},
				Question:  "问题",
				Prompt:    "提示",
				Limits:    tc.limits,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("实际错误 %v，预期 %v", err, tc.want)
			}
			if calls != 1 {
				t.Fatalf("不应继续调用模型，实际 %d 次", calls)
			}
			if result.Completed || result.ModelCalls != 1 || len(result.Calls) == 0 {
				t.Fatalf("失败后没有保留记录或正确收尾：%+v，错误 %v", result, err)
			}
		})
	}
}

func TestChatCanceledBeforeModelCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
		t.Fatal("取消后不应调用模型")
		return nil, nil
	})
	agentCli := newTestClient(t, chatCli, DefaultLimits())
	result, err := agentCli.Chat(ctx, ChatReq{
		Operation: OperationRun,
		Tools:     MultiplyTools{},
		Question:  "问题",
		Prompt:    "提示",
	})
	if !errors.Is(err, context.Canceled) || result.ModelCalls != 0 {
		t.Fatalf("取消后实际结果 %+v、错误 %v，预期不调用模型并结束记录", result, err)
	}
}

func chatFailureCases() []chatFailureCase {
	return []chatFailureCase{
		{
			name: "模型预算",
			limits: Limits{
				ModelCalls: 1,
				ToolCalls:  8,
				Tokens:     8000,
			},
			ids:  []string{"one"},
			want: ErrBudgetExceeded,
		},
		{
			name: "工具预算",
			limits: Limits{
				ModelCalls: 5,
				ToolCalls:  1,
				Tokens:     8000,
			},
			ids: []string{
				"one",
				"two",
			},
			want: ErrBudgetExceeded,
		},
		{
			name: "token 软预算",
			limits: Limits{
				ModelCalls: 5,
				ToolCalls:  8,
				Tokens:     10,
			},
			ids:    []string{"one"},
			tokens: 10,
			want:   ErrBudgetExceeded,
		},
		{
			name: "token 超额仍停止下一轮",
			limits: Limits{
				ModelCalls: 5,
				ToolCalls:  8,
				Tokens:     10,
			},
			ids:    []string{"one"},
			tokens: math.MaxInt,
			want:   ErrBudgetExceeded,
		},
		{
			name:   "重复 ID",
			limits: DefaultLimits(),
			ids: []string{
				"one",
				"one",
			},
			want: ErrInvalidResponse,
		},
		{
			name:   "缺少 ID",
			limits: DefaultLimits(),
			ids:    []string{""},
			want:   ErrInvalidResponse,
		},
	}
}

func TestChatCountsToolBudgetAcrossRounds(t *testing.T) {
	ids := []string{
		"first-tool",
		"second-tool",
	}
	calls := 0
	chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
		if calls >= len(ids) {
			t.Fatal("工具预算用完后不应继续请求模型")
		}
		id := ids[calls]
		calls++
		return &Response{
			Message: Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:   id,
						Type: ToolTypeFunction,
						Function: FunctionCall{
							Name:      ToolNameMultiply,
							Arguments: `{"a":2,"b":3}`,
						},
					},
				},
			},
			FinishReason: FinishReasonToolCalls,
		}, nil
	})
	limits := DefaultLimits()
	limits.ToolCalls = 1
	agentCli := newTestClient(t, chatCli, limits)
	result, err := agentCli.Chat(t.Context(), ChatReq{
		Question: "分两轮计算乘法",
		Tools:    MultiplyTools{},
	})
	if !errors.Is(err, ErrBudgetExceeded) || calls != 2 || result.ToolCalls != 1 || result.Completed {
		t.Fatalf("跨轮次没有正确累计工具预算：%+v，错误 %v，模型请求 %d 次", result, err, calls)
	}
	if len(result.Calls) != 3 {
		t.Fatalf("预算停止时没有保留记录并收尾：%+v", result)
	}
}

func TestChatChecksWholeToolBatchBeforeExecution(t *testing.T) {
	for _, tc := range []struct {
		name  string   // 要检查的整轮工具请求问题。
		ids   []string // 模型提出的工具调用 ID。
		limit int      // 本次最多能执行的工具次数。
		want  error    // 执行前应返回的错误。
	}{
		{
			name: "后一个 ID 为空",
			ids: []string{
				"one",
				"",
			},
			limit: 2,
			want:  ErrInvalidResponse,
		},
		{
			name: "同轮 ID 重复",
			ids: []string{
				"one",
				"one",
			},
			limit: 2,
			want:  ErrInvalidResponse,
		},
		{
			name: "整轮工具数量超限",
			ids: []string{
				"one",
				"two",
			},
			limit: 1,
			want:  ErrBudgetExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatCli := modelFunc(func(context.Context, Request) (*Response, error) {
				var calls []ToolCall
				for _, id := range tc.ids {
					calls = append(calls, ToolCall{
						ID:   id,
						Type: ToolTypeFunction,
						Function: FunctionCall{
							Name:      ToolNameMultiply,
							Arguments: `{"a":2,"b":3}`,
						},
					})
				}
				return &Response{
					Message: Message{
						Role:      RoleAssistant,
						ToolCalls: calls,
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			})
			limits := DefaultLimits()
			limits.ToolCalls = tc.limit
			agentCli := newTestClient(t, chatCli, limits)
			result, err := agentCli.Chat(t.Context(), ChatReq{
				Operation: OperationRun,
				Tools:     MultiplyTools{},
				Question:  "计算乘法",
				Prompt:    "计算助手",
			})
			if !errors.Is(err, tc.want) || result.ToolCalls != 0 {
				t.Fatalf("实际错误 %v，执行了 %d 次工具；预期错误 %v 且不执行任何工具", err, result.ToolCalls, tc.want)
			}
		})
	}
}
