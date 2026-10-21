package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// toolTraceStub 保留真实工具定义，只替换执行结果以覆盖失败路径。
type toolTraceStub struct {
	agent.MultiplyTools                                                                      // 提供真实的乘法工具定义。
	dispatch            func(context.Context, agent.ToolCallReq) (*agent.ToolCallRsp, error) // 执行替身，不允许为 nil。
}

// Dispatch 执行当前用例的工具替身。
func (s toolTraceStub) Dispatch(ctx context.Context, req agent.ToolCallReq) (*agent.ToolCallRsp, error) {
	return s.dispatch(ctx, req)
}

// newToolTraceClient 创建使用内存导出器的观测客户端，不访问外部服务。
func newToolTraceClient(t *testing.T, capture bool) (*openobserve.Client, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	observeCli, err := openobserve.New(t.Context(), openobserve.Config{
		Enabled:        true,
		ServiceName:    "test",
		TracerName:     "test",
		CaptureContent: capture,
		MaxChars:       256,
		Secrets:        []string{"test-secret"},
	}, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := observeCli.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return observeCli, exporter
}

// toolSpanAttributes 按属性名读取观测断言，预期值仍使用独立协议样本。
func toolSpanAttributes(attrs []attribute.KeyValue) map[string]attribute.Value {
	values := make(map[string]attribute.Value, len(attrs))
	for _, attr := range attrs {
		values[string(attr.Key)] = attr.Value
	}
	return values
}

func TestToolTracePreservesOutcomeAndContext(t *testing.T) {
	for _, tc := range []struct {
		name   string             // 工具完成、业务拒绝或执行失败的场景。
		result *agent.ToolCallRsp // 工具原始返回值，允许 nil。
		err    error              // 工具原始错误，允许 nil。
		status codes.Code         // 工具节点应记录的状态。
	}{
		{
			name: "执行成功",
			result: &agent.ToolCallRsp{
				OK:     true,
				Result: 391,
			},
		},
		{
			name: "业务拒绝",
			result: &agent.ToolCallRsp{
				Error: "拒绝 test-secret",
			},
			status: codes.Error,
		},
		{
			name:   "执行失败",
			err:    errors.New("失败 test-secret"),
			status: codes.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeCli, exporter := newToolTraceClient(t, true)
			ctx, parent := observeCli.Tracer.Start(t.Context(), "agent.run")
			req := agent.ToolCallReq{
				ID:   "call-1",
				Type: agent.ToolTypeFunction,
				Function: agent.ToolFunctionCall{
					Name:      "multiply",
					Arguments: `{"password":"test-secret","a":17,"b":23}`,
				},
			}
			tools := &tracedTools{
				Tools: toolTraceStub{
					dispatch: func(toolCtx context.Context, actual agent.ToolCallReq) (*agent.ToolCallRsp, error) {
						child := trace.SpanContextFromContext(toolCtx)
						if actual != req || child.TraceID() != parent.SpanContext().TraceID() || child.SpanID() == parent.SpanContext().SpanID() {
							t.Error("工具参数被改写或未沿同一链路传入子 span")
						}
						return tc.result, tc.err
					},
				},
				observeCli: observeCli,
			}
			result, err := tools.Dispatch(ctx, req)
			parent.End()
			if result != tc.result || err != tc.err {
				t.Fatal("工具观测改变了原始结果或错误")
			}
			if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
				t.Fatal(err)
			}
			spans := exporter.GetSpans()
			if len(spans) != 2 || spans[0].Name != "multiply" || spans[0].Parent.SpanID() != parent.SpanContext().SpanID() || spans[0].Status.Code != tc.status {
				t.Fatalf("实际节点 %+v，预期工具节点挂在运行下且状态为 %v", spans, tc.status)
			}
			attrs := toolSpanAttributes(spans[0].Attributes)
			if attrs["gen_ai.operation.name"].AsString() != "execute_tool" || attrs["app.tool.name"].AsString() != "multiply" || attrs["app.tool.call.id"].AsString() != "call-1" {
				t.Error("工具节点未保留旧版名称与调用 ID 属性")
			}
			if _, ok := attrs["app.elapsed_ms"]; !ok {
				t.Error("工具节点缺少原有的毫秒耗时属性")
			}
			if tc.result != nil && !tc.result.OK && (attrs["app.tool.error"].AsString() != "拒绝 [REDACTED]" || tc.result.Error != "拒绝 test-secret") {
				t.Error("工具业务拒绝未保留脱敏后的 app.tool.error")
			}
			for _, attr := range spans[0].Attributes {
				if strings.Contains(attr.Value.AsString(), "test-secret") {
					t.Fatal("工具节点泄漏了参数、结果或错误中的敏感值")
				}
			}
		})
	}
}

func TestToolTraceInvalidResultStillEndsSpan(t *testing.T) {
	for _, result := range []*agent.ToolCallRsp{
		nil,
		{
			OK:     true,
			Result: make(chan int),
		},
	} {
		observeCli, exporter := newToolTraceClient(t, true)
		tools := &tracedTools{
			Tools: toolTraceStub{
				dispatch: func(context.Context, agent.ToolCallReq) (*agent.ToolCallRsp, error) {
					return result, nil
				},
			},
			observeCli: observeCli,
		}
		actual, err := tools.Dispatch(t.Context(), agent.ToolCallReq{})
		if actual != result || err != nil {
			t.Fatal("观测编码失败不应覆盖工具原始返回值")
		}
		if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
			t.Fatal(err)
		}
		spans := exporter.GetSpans()
		if len(spans) != 1 || spans[0].Status.Code != codes.Error {
			t.Fatalf("实际节点 %+v，预期无效结果仍结束节点并标记错误", spans)
		}
	}
}

func TestToolTraceCaptureSwitchAndCancellation(t *testing.T) {
	observeCli, exporter := newToolTraceClient(t, false)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	deadline, _ := ctx.Deadline()
	tools := &tracedTools{
		Tools: toolTraceStub{
			dispatch: func(toolCtx context.Context, _ agent.ToolCallReq) (*agent.ToolCallRsp, error) {
				if actual, ok := toolCtx.Deadline(); !ok || actual != deadline {
					t.Fatal("工具执行丢失请求截止时间")
				}
				cancel()
				return nil, toolCtx.Err()
			},
		},
		observeCli: observeCli,
	}
	_, err := tools.Dispatch(ctx, agent.ToolCallReq{
		Function: agent.ToolFunctionCall{
			Name:      "multiply",
			Arguments: `{"a":17,"b":23}`,
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("实际错误 %v，预期取消沿上下文传播", err)
	}
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatal(err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("实际节点 %+v，预期关闭正文后仍记录工具失败节点", spans)
	}
	attrs := toolSpanAttributes(spans[0].Attributes)
	if attrs["gen_ai.operation.name"].AsString() != "execute_tool" || attrs["app.tool.name"].AsString() != "multiply" {
		t.Fatal("关闭正文采集后工具节点仍应保留操作类型和名称")
	}
	if _, ok := attrs["app.request"]; ok {
		t.Fatal("关闭正文采集时仍记录了工具参数")
	}
}

func TestToolTraceInputPreviewPreservesArguments(t *testing.T) {
	for _, tc := range []struct {
		name      string // 参数格式、数值精度或正文截断场景。
		arguments string // 模型返回的原始参数字符串。
		want      string // 输入预览应保留的 JSON 片段，截断场景不比较片段。
		truncated bool   // 输入预览是否应带截断标记。
	}{
		{
			name:      "结构化参数保留精度并脱敏",
			arguments: `{"count":9007199254740993,"password":"test-secret"}`,
			want:      `"arguments":{"count":9007199254740993,"password":"[REDACTED]"}`,
		},
		{
			name:      "无效参数片段保留为文本",
			arguments: `{"a":17`,
			want:      `"arguments":"{\"a\":17"`,
		},
		{
			name:      "过长输入按字符截断",
			arguments: `{"text":"` + strings.Repeat("参数", 200) + `"}`,
			truncated: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeCli, exporter := newToolTraceClient(t, true)
			tools := &tracedTools{
				Tools: toolTraceStub{
					dispatch: func(context.Context, agent.ToolCallReq) (*agent.ToolCallRsp, error) {
						return &agent.ToolCallRsp{
							OK:     true,
							Result: 391,
						}, nil
					},
				},
				observeCli: observeCli,
			}
			_, err := tools.Dispatch(t.Context(), agent.ToolCallReq{
				ID:   "call-1",
				Type: agent.ToolTypeFunction,
				Function: agent.ToolFunctionCall{
					Name:      "multiply",
					Arguments: tc.arguments,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
				t.Fatal(err)
			}
			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("实际节点 %d，预期一个工具节点", len(spans))
			}
			attrs := toolSpanAttributes(spans[0].Attributes)
			preview := attrs["app.request"].AsString()
			if preview != attrs["gen_ai.input.messages"].AsString() || attrs["app.request.truncated"].AsBool() != tc.truncated {
				t.Error("输入预览未复用原始上报字段或截断标记不正确")
			}
			if tc.truncated {
				if !strings.HasSuffix(preview, "…[TRUNCATED]") || utf8.RuneCountInString(preview) != 256+utf8.RuneCountInString("…[TRUNCATED]") {
					t.Errorf("实际预览 %q，预期按 256 字符截断", preview)
				}
			} else if !strings.Contains(preview, tc.want) || !strings.Contains(preview, `"id":"call-1","name":"multiply","type":"function"`) || strings.Contains(preview, "test-secret") {
				t.Errorf("实际预览 %s，预期保留调用标识和参数片段 %s 并脱敏", preview, tc.want)
			}
		})
	}
}

func TestToolTraceSkipsExecutionAfterCancellation(t *testing.T) {
	observeCli, exporter := newToolTraceClient(t, true)
	tools := &tracedTools{
		Tools: toolTraceStub{
			dispatch: func(context.Context, agent.ToolCallReq) (*agent.ToolCallRsp, error) {
				t.Fatal("取消后的请求不应继续执行工具")
				return nil, nil
			},
		},
		observeCli: observeCli,
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := tools.Dispatch(ctx, agent.ToolCallReq{})
	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("实际结果 %+v、错误 %v，预期 nil 和 context.Canceled", result, err)
	}
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Fatalf("实际节点 %+v，预期未执行工具时不生成节点", spans)
	}
}

func TestToolSpanBetweenModelCalls(t *testing.T) {
	exporter := &memoryExporter{}
	handler, observeCli := setup(t, lessonModel(t, "agent"), routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, exporter)
	out, rsp := invoke(handler, "agent", `{}`)
	if out.Code != http.StatusOK || rsp.Data.ToolCalls != 1 {
		t.Fatalf("实际响应 %s，预期执行一次乘法工具", out.Body.String())
	}
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatal(err)
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	steps := make([]sdktrace.ReadOnlySpan, 0, 3)
	for _, span := range exporter.spans {
		if span.SpanKind() == trace.SpanKindClient || span.Name() == "multiply" {
			steps = append(steps, span)
		}
	}
	slices.SortFunc(steps, func(a, b sdktrace.ReadOnlySpan) int {
		return a.StartTime().Compare(b.StartTime())
	})
	if len(exporter.spans) != 5 || len(steps) != 3 || steps[0].SpanKind() != trace.SpanKindClient || steps[1].Name() != "multiply" || steps[2].SpanKind() != trace.SpanKindClient {
		t.Fatalf("实际执行节点 %+v，预期模型、multiply、模型依次展示", steps)
	}
	if steps[0].EndTime().After(steps[1].StartTime()) || steps[1].EndTime().After(steps[2].StartTime()) {
		t.Fatal("工具节点没有覆盖两次模型请求之间的实际执行区间")
	}
	attrs := toolSpanAttributes(steps[1].Attributes())
	if attrs["gen_ai.input.messages"].AsString() != `{"arguments":{"a":17,"b":23},"id":"multiply-1","name":"multiply","type":"function"}` || attrs["gen_ai.output.messages"].AsString() != `{"ok":true,"result":391}` || attrs["app.tool.call.id"].AsString() != "multiply-1" {
		t.Fatalf("实际工具预览 %+v，预期参数、调用 ID 及 391 结果完整", attrs)
	}
	if attrs["app.request"].AsString() != attrs["gen_ai.input.messages"].AsString() || attrs["app.response"].AsString() != attrs["gen_ai.output.messages"].AsString() {
		t.Error("工具预览未直接复用原有输入输出上报字段")
	}
}
