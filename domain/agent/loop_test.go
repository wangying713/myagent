package agent

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

type modelFunc func(context.Context, Request) (Response, error)

func (f modelFunc) Chat(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}

func TestLoopPreservesAssistantAndMultipleToolResults(t *testing.T) {
	calls := 0
	modelCli := modelFunc(func(ctx context.Context, req Request) (Response, error) {
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
			return Response{
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
		return Response{
			Message: Message{
				Role:    "assistant",
				Content: "391 和 6",
			},
			FinishReason: "stop",
		}, nil
	})
	answer, err := Run(context.Background(), LoopInput{
		ModelCli: modelCli,
		Tools:    MultiplyTools{},
		Question: "问题",
		Prompt:   "计算助手",
		Limits:   DefaultLimits(),
	})
	if err != nil || answer != "391 和 6" || calls != 2 {
		t.Fatalf("运行结果 %q/%v，调用 %d 次", answer, err, calls)
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
	if !errors.Is(err, context.Canceled) || out.OK || out.Result != nil {
		t.Fatalf("取消后的乘法结果为 %+v/%v，预期返回取消错误且无计算结果", out, err)
	}
}

func TestLoopBudgetsAndProtocolErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string   // 用例名称。
		limits Limits   // 本用例采用的运行预算。
		ids    []string // 模拟模型提出的工具调用 ID 列表。
		tokens int      // 模拟模型报告的单次 token 用量。
		want   error    // 预期返回的预算或协议错误。
	}{
		{
			"模型预算",
			Limits{
				1,
				8,
				8000,
			},
			[]string{"one"},
			0,
			ErrBudgetExceeded,
		},
		{
			"工具预算",
			Limits{
				5,
				1,
				8000,
			},
			[]string{
				"one",
				"two",
			},
			0,
			ErrBudgetExceeded,
		},
		{
			"token 软预算",
			Limits{
				5,
				8,
				10,
			},
			[]string{"one"},
			10,
			ErrBudgetExceeded,
		},
		{
			"token 超额仍停止下一轮",
			Limits{
				5,
				8,
				10,
			},
			[]string{"one"},
			math.MaxInt,
			ErrBudgetExceeded,
		},
		{
			"重复 ID",
			DefaultLimits(),
			[]string{
				"one",
				"one",
			},
			0,
			ErrInvalidResponse,
		},
		{
			"缺少 ID",
			DefaultLimits(),
			[]string{""},
			0,
			ErrInvalidResponse,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			modelCli := modelFunc(func(context.Context, Request) (Response, error) {
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
				return Response{
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
			_, err := Run(context.Background(), LoopInput{
				ModelCli: modelCli,
				Tools:    MultiplyTools{},
				Question: "问题",
				Prompt:   "提示",
				Limits:   tc.limits,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("实际错误 %v，预期 %v", err, tc.want)
			}
			if calls != 1 {
				t.Fatalf("不应继续调用模型，实际 %d 次", calls)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	modelCli := modelFunc(func(context.Context, Request) (Response, error) {
		t.Fatal("取消后不应调用模型")
		return Response{}, nil
	})
	_, err := Run(ctx, LoopInput{
		ModelCli: modelCli,
		Tools:    MultiplyTools{},
		Question: "问题",
		Prompt:   "提示",
		Limits:   DefaultLimits(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestLoopChecksWholeToolBatchBeforeExecution(t *testing.T) {
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
			modelCli := modelFunc(func(context.Context, Request) (Response, error) {
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
				return Response{
					Message: Message{
						Role:      RoleAssistant,
						ToolCalls: calls,
					},
					FinishReason: FinishReasonToolCalls,
				}, nil
			})
			result := &ExecutionResult{}
			observer := &testObserver{}
			limits := DefaultLimits()
			limits.ToolCalls = tc.limit
			_, err := Run(t.Context(), LoopInput{
				ModelCli: modelCli,
				Tools: &recordingTools{
					tools:    MultiplyTools{},
					observer: observer,
					result:   result,
				},
				Question: "计算乘法",
				Prompt:   "计算助手",
				Limits:   limits,
			})
			if !errors.Is(err, tc.want) || result.ToolCalls != 0 || len(observer.tools) != 0 {
				t.Fatalf("实际错误 %v，执行了 %d 次工具；预期错误 %v 且不执行任何工具", err, result.ToolCalls, tc.want)
			}
		})
	}
}
