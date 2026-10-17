package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type modelFunc func(context.Context, Request) (Response, error)

func (f modelFunc) Chat(ctx context.Context, r Request) (Response, error) {
	return f(ctx, r)
}

func TestLoopPreservesAssistantAndMultipleToolResults(t *testing.T) {
	calls := 0
	model := modelFunc(func(ctx context.Context, r Request) (Response, error) {
		calls++
		if calls == 1 {
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
		if len(r.Messages) != 5 || r.Messages[3].ToolCallID != "one" || r.Messages[4].ToolCallID != "two" {
			t.Fatalf("工具回填与 ID 不匹配：%+v", r.Messages)
		}
		body, err := json.Marshal(r.Messages[2])
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
			if err := json.Unmarshal([]byte(r.Messages[i+3].Content), &out); err != nil {
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
	answer, err := Run(context.Background(), model, MultiplyTools{}, "问题", "计算助手", DefaultLimits())
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
			model := modelFunc(func(context.Context, Request) (Response, error) {
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
					Usage:        Usage{TotalTokens: &tc.tokens},
				}, nil
			})
			_, err := Run(context.Background(), model, MultiplyTools{}, "问题", "提示", tc.limits)
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
	_, err := Run(ctx, modelFunc(func(context.Context, Request) (Response, error) {
		t.Fatal("取消后不应调用模型")
		return Response{}, nil
	}), MultiplyTools{}, "问题", "提示", DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
