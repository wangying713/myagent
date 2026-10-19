package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// testObserver 使用内存句柄验证领域观测，测试不依赖基础设施。
type testObserver struct {
	runs  []*testRunObservation  // 测试期间创建的业务运行观测句柄。
	tools []*testToolObservation // 测试期间创建的工具调用观测句柄。
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
func (o *testObserver) StartRun(ctx context.Context, operation string, req RunDetails) (context.Context, RunObservation) {
	run := &testRunObservation{
		operation: operation,
		details:   req,
	}
	o.runs = append(o.runs, run)
	return ctx, run
}
func (o *testObserver) StartTool(ctx context.Context, _ ToolCall) (context.Context, ToolObservation) {
	tool := &testToolObservation{}
	o.tools = append(o.tools, tool)
	return ctx, tool
}

// testRunObservation 记录业务观测的结束参数与调用状态。
type testRunObservation struct {
	operation string     // 本次观测对应的业务标识。
	details   RunDetails // 启动观测时收到的问题和实际预算。
	result    Result     // 结束观测时收到的业务结果。
	err       error      // 结束观测时收到的业务错误。
	ended     bool       // 是否已调用结束方法。
}

func (*testRunObservation) TraceID() string {
	return "test-trace"
}
func (r *testRunObservation) End(result Result, err error) {
	r.result, r.err, r.ended = result, err, true
}

// testToolObservation 记录工具观测的结束参数与调用状态。
type testToolObservation struct {
	output ToolResult // 结束观测时收到的工具结果。
	err    error      // 结束观测时收到的执行错误。
	ended  bool       // 是否已调用结束方法。
}

func (t *testToolObservation) End(output ToolResult, err error) {
	t.output, t.err, t.ended = output, err, true
}

func newTestClient(t *testing.T, chatCli ChatClient, observer *testObserver, limits Limits) *Client {
	t.Helper()
	agentCli, err := NewClient(chatCli, observer, limits)
	if err != nil {
		t.Fatal(err)
	}
	return agentCli
}

func TestClientChatRecordsSafeCopy(t *testing.T) {
	observer := &testObserver{}
	chatCli := modelFunc(func(ctx context.Context, req Request) (Response, error) {
		if len(req.Messages) != 2 || req.Messages[1].Content != ChatQuestion {
			t.Fatalf("实际默认请求 %+v，预期默认聊天问题", req.Messages)
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
	result, err := newTestClient(t, chatCli, observer, DefaultLimits()).Chat(context.Background(), ChatInput{})
	if err != nil || !result.Completed || result.Answer != "回答包含[REDACTED]" || result.ModelCalls != 1 || len(result.Steps) != 1 || result.UsageComplete {
		t.Fatalf("聊天结果不符合契约：%+v，错误 %v", result, err)
	}
	if strings.Contains(result.Steps[0].Response, "本机密钥") || result.TraceID != "test-trace" || !observer.runs[0].ended || !observer.runs[0].result.Completed {
		t.Fatalf("脱敏或运行结束记录错误：%+v", result)
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
			chatCli := modelFunc(func(ctx context.Context, req Request) (Response, error) {
				calls++
				answer := "记住了豆包。"
				if calls == 2 {
					if len(req.Messages) != 4 || req.Messages[2].Content != "记住了豆包。" || req.Messages[3].Content != SecondQuestion {
						t.Fatalf("第二轮丢失真实历史：%+v", req.Messages)
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
			result, err := newTestClient(t, chatCli, observer, limits).Messages(context.Background(), MessagesInput{})
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

func TestClientRunRecordsTool(t *testing.T) {
	calls := 0
	chatCli := modelFunc(func(ctx context.Context, req Request) (Response, error) {
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
		if len(req.Messages) != 4 || req.Messages[3].ToolCallID != "one" || !strings.Contains(req.Messages[3].Content, "391") {
			t.Fatalf("未回填工具结果：%+v", req.Messages)
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
	result, err := newTestClient(t, chatCli, observer, DefaultLimits()).Run(context.Background(), "")
	if err != nil || !result.Completed || result.ModelCalls != 2 || result.ToolCalls != 1 || len(result.Steps) != 3 || result.Steps[1].Kind != "tool" {
		t.Fatalf("工具执行记录错误：%+v，错误 %v", result, err)
	}
	if len(observer.tools) != 1 || !observer.tools[0].ended || !observer.tools[0].output.OK || observer.tools[0].output.Result != float64(391) {
		t.Fatalf("实际工具观测 %+v，预期一次完成且结果为 float64(391)", observer.tools)
	}
}

func TestClientInputAndCancellation(t *testing.T) {
	observer := &testObserver{}
	agentCli := newTestClient(t, modelFunc(func(context.Context, Request) (Response, error) {
		t.Fatal("无效输入或取消后不应调用模型")
		return Response{}, nil
	}), observer, DefaultLimits())
	for _, question := range []string{
		"  ",
		strings.Repeat("字", 4097),
	} {
		if _, err := agentCli.Chat(context.Background(), ChatInput{
			Question: question,
		}); !errors.Is(err, ErrInput) {
			t.Errorf("聊天实际错误 %v，预期 ErrInput", err)
		}
		if _, err := agentCli.Run(context.Background(), question); !errors.Is(err, ErrInput) {
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
	if len(observer.runs) != 0 {
		t.Fatal("输入校验失败不应开始运行观测")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := agentCli.Chat(ctx, ChatInput{})
	if !errors.Is(err, context.Canceled) || result.ModelCalls != 0 || result.Completed || !observer.runs[0].ended {
		t.Fatalf("取消执行错误：%+v，错误 %v", result, err)
	}
}
