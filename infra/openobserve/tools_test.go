package openobserve

import (
	"context"
	"errors"
	"testing"

	"saklay/ai/domain/agent"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// testTools 提供可检查上下文、参数和返回结果的工具执行替身。
type testTools struct {
	dispatch func(context.Context, agent.ToolCall) (*agent.ToolResult, error) // 当前用例的执行行为。
}

// Definitions 返回用例使用的真实乘法定义。
func (testTools) Definitions() []agent.ToolDefinition {
	return agent.MultiplyTools{}.Definitions()
}

// Dispatch 把工具输入原样交给用例检查。
func (t testTools) Dispatch(ctx context.Context, req agent.ToolCall) (*agent.ToolResult, error) {
	return t.dispatch(ctx, req)
}

func TestToolsPreservesResultContextAndCancellation(t *testing.T) {
	for _, cancelDuringCall := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "结果原文不被脱敏",
			true:  "执行中取消并结束观测",
		}[cancelDuringCall], func(t *testing.T) {
			observeCli, exporter := newTestClient(t, true, 4096)
			parentCtx, parent := observeCli.Start(t.Context(), "parent")
			defer parent.End()
			ctx, cancel := context.WithCancel(parentCtx)
			defer cancel()
			req := agent.ToolCall{
				ID:   "one",
				Type: agent.ToolTypeFunction,
				Function: agent.FunctionCall{
					Name:      agent.ToolNameMultiply,
					Arguments: `{"a":2,"b":3}`,
				},
			}
			var toolID trace.SpanID
			tools := Tools{
				ObserveCli: observeCli,
				Tools: testTools{
					dispatch: func(ctx context.Context, actual agent.ToolCall) (*agent.ToolResult, error) {
						toolID = trace.SpanFromContext(ctx).SpanContext().SpanID()
						if actual != req || !toolID.IsValid() || toolID == parent.SpanContext().SpanID() {
							t.Fatalf("实际工具输入 %+v，预期原样传递且使用工具子上下文", actual)
						}
						if cancelDuringCall {
							cancel()
							return nil, ctx.Err()
						}
						return &agent.ToolResult{
							OK:     true,
							Result: "configured-secret",
						}, nil
					},
				},
			}
			result, err := tools.Dispatch(ctx, req)
			if cancelDuringCall {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("实际错误 %v，预期传播取消", err)
				}
			} else if err != nil || result.Result != "configured-secret" {
				t.Fatalf("实际工具结果 %+v、错误 %v，预期保留原始结果", result, err)
			}
			attrs := toolAttributes(t, observeCli, exporter)
			span := exporter.GetSpans()[0]
			if span.Parent.SpanID() != parent.SpanContext().SpanID() || span.SpanContext.SpanID() != toolID || (span.Status.Code == codes.Error) != cancelDuringCall {
				t.Fatal("工具的父链路、执行上下文或取消状态不符合预期")
			}
			if !cancelDuringCall && attrs["app.response"].AsString() != `{"ok":true,"result":"[REDACTED]"}` {
				t.Fatalf("实际观测输出 %s，预期仅对展示副本脱敏", attrs["app.response"].AsString())
			}
		})
	}
}

func TestToolsRejectsMissingResultAndEndsObservation(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	tools := Tools{
		ObserveCli: observeCli,
		Tools: testTools{
			dispatch: func(context.Context, agent.ToolCall) (*agent.ToolResult, error) {
				return nil, nil
			},
		},
	}
	result, err := tools.Dispatch(t.Context(), agent.ToolCall{
		ID:   "one",
		Type: agent.ToolTypeFunction,
		Function: agent.FunctionCall{
			Name:      agent.ToolNameMultiply,
			Arguments: `{"a":2,"b":3}`,
		},
	})
	if result != nil || !errors.Is(err, agent.ErrInvalidResponse) {
		t.Fatalf("实际结果 %+v、错误 %v，预期缺失结果转换为协议错误", result, err)
	}
	toolAttributes(t, observeCli, exporter)
	if exporter.GetSpans()[0].Status.Code != codes.Error {
		t.Fatal("缺失工具结果没有结束为错误观测")
	}
}
