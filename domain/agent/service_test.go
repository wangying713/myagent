package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// testObserver 使用内存句柄验证领域观测，测试不依赖基础设施。
type testObserver struct {
	runs  []*testRunObservation
	tools []*testToolObservation
}

func (*testObserver) Enabled() bool {
	return true
}
func (o *testObserver) Safe(raw []byte) string {
	return o.Clean(string(raw))
}
func (*testObserver) Clean(text string) string {
	return strings.ReplaceAll(text, "本机密钥", "[REDACTED]")
}
func (o *testObserver) StartRun(ctx context.Context, _ string, _ []string) (context.Context, RunObservation) {
	run := &testRunObservation{}
	o.runs = append(o.runs, run)
	return ctx, run
}
func (o *testObserver) StartTool(ctx context.Context, _ ToolCall) (context.Context, ToolObservation) {
	tool := &testToolObservation{}
	o.tools = append(o.tools, tool)
	return ctx, tool
}

type testRunObservation struct {
	result ExecutionResult
	err    error
	ended  bool
}

func (*testRunObservation) TraceID() string {
	return "test-trace"
}
func (r *testRunObservation) End(result ExecutionResult, err error) {
	r.result, r.err, r.ended = result, err, true
}

type testToolObservation struct {
	output ToolResult
	err    error
	ended  bool
}

func (t *testToolObservation) End(output ToolResult, err error) {
	t.output, t.err, t.ended = output, err, true
}

func newTestService(t *testing.T, model ChatClient, observer *testObserver, limits Limits) *Service {
	t.Helper()
	service, err := NewService(model, observer, limits)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestServiceChatRecordsSafeCopy(t *testing.T) {
	observer := &testObserver{}
	model := modelFunc(func(ctx context.Context, request Request) (Response, error) {
		if len(request.Messages) != 2 || request.Messages[1].Content != ChatQuestion {
			t.Fatalf("实际默认请求 %+v，预期默认聊天问题", request.Messages)
		}
		return Response{
			Message: Message{
				Role:    "assistant",
				Content: "回答包含本机密钥",
			},
			FinishReason: "stop",
			Raw:          []byte(`{"answer":"本机密钥"}`),
		}, nil
	})
	result, err := newTestService(t, model, observer, DefaultLimits()).Chat(context.Background(), ChatInput{})
	if err != nil || !result.Completed || result.Answer != "回答包含[REDACTED]" || result.ModelCalls != 1 || len(result.Steps) != 1 || result.UsageComplete {
		t.Fatalf("聊天结果不符合契约：%+v，错误 %v", result, err)
	}
	if strings.Contains(result.Steps[0].Response, "本机密钥") || result.TraceID != "test-trace" || !observer.runs[0].ended || !observer.runs[0].result.Completed {
		t.Fatalf("脱敏或运行结束记录错误：%+v", result)
	}
}

func TestServiceMessagesPreservesHistoryAndPartialAnswer(t *testing.T) {
	for _, limited := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "完整两轮",
			true:  "预算停止",
		}[limited], func(t *testing.T) {
			calls := 0
			model := modelFunc(func(ctx context.Context, request Request) (Response, error) {
				calls++
				answer := "记住了豆包。"
				if calls == 2 {
					if len(request.Messages) != 4 || request.Messages[2].Content != "记住了豆包。" || request.Messages[3].Content != SecondQuestion {
						t.Fatalf("第二轮丢失真实历史：%+v", request.Messages)
					}
					answer = "豆包"
				}
				return Response{
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
			observer := &testObserver{}
			result, err := newTestService(t, model, observer, limits).Messages(context.Background(), MessagesInput{})
			if limited {
				if !errors.Is(err, ErrBudgetExceeded) || result.Completed || result.Answer != "记住了豆包。" || calls != 1 {
					t.Fatalf("预算停止丢失部分结果：%+v，错误 %v", result, err)
				}
			} else if err != nil || !result.Completed || result.Answer != "豆包" || calls != 2 {
				t.Fatalf("两轮执行错误：%+v，错误 %v", result, err)
			}
			if !observer.runs[0].ended || !errors.Is(observer.runs[0].err, err) {
				t.Error("根运行记录未收到最终错误")
			}
		})
	}
}

func TestServiceRunRecordsTool(t *testing.T) {
	calls := 0
	model := modelFunc(func(ctx context.Context, request Request) (Response, error) {
		calls++
		if calls == 1 {
			return Response{
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
		if len(request.Messages) != 4 || request.Messages[3].ToolCallID != "one" || !strings.Contains(request.Messages[3].Content, "391") {
			t.Fatalf("未回填工具结果：%+v", request.Messages)
		}
		return Response{
			Message: Message{
				Role:    "assistant",
				Content: "391",
			},
			FinishReason: "stop",
		}, nil
	})
	observer := &testObserver{}
	result, err := newTestService(t, model, observer, DefaultLimits()).Run(context.Background(), RunInput{})
	if err != nil || !result.Completed || result.ModelCalls != 2 || result.ToolCalls != 1 || len(result.Steps) != 3 || result.Steps[1].Kind != "tool" {
		t.Fatalf("工具执行记录错误：%+v，错误 %v", result, err)
	}
	if len(observer.tools) != 1 || !observer.tools[0].ended || !observer.tools[0].output.OK || observer.tools[0].output.Result != float64(391) {
		t.Fatalf("实际工具观测 %+v，预期一次完成且结果为 float64(391)", observer.tools)
	}
}

func TestServiceInputAndCancellation(t *testing.T) {
	observer := &testObserver{}
	service := newTestService(t, modelFunc(func(context.Context, Request) (Response, error) {
		t.Fatal("无效输入或取消后不应调用模型")
		return Response{}, nil
	}), observer, DefaultLimits())
	for _, question := range []string{
		"  ",
		strings.Repeat("字", 4097),
	} {
		if _, err := service.Chat(context.Background(), ChatInput{Question: question}); !errors.Is(err, ErrInput) {
			t.Errorf("聊天实际错误 %v，预期 ErrInput", err)
		}
		if _, err := service.Run(context.Background(), RunInput{Question: question}); !errors.Is(err, ErrInput) {
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
		if _, err := service.Messages(context.Background(), MessagesInput{Questions: questions}); !errors.Is(err, ErrInput) {
			t.Errorf("历史消息实际错误 %v，预期 ErrInput", err)
		}
	}
	if len(observer.runs) != 0 {
		t.Fatal("输入校验失败不应开始运行观测")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := service.Chat(ctx, ChatInput{})
	if !errors.Is(err, context.Canceled) || result.ModelCalls != 0 || result.Completed || !observer.runs[0].ended {
		t.Fatalf("取消执行错误：%+v，错误 %v", result, err)
	}
}
