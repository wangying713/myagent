package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDemoStructuredValidatesModelOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string // 被检查的模型输出类型。
		content string // 模型实际返回的 JSON 或错误正文。
		valid   bool   // 是否应通过校验。
	}{
		{
			name:    "有效商品",
			content: `{"product":"笔记本","quantity":3}`,
			valid:   true,
		},
		{
			name:    "重复字段",
			content: `{"product":"笔记本","quantity":3,"quantity":4}`,
		},
		{
			name:    "未知字段",
			content: `{"product":"笔记本","quantity":3,"price":23}`,
		},
		{
			name:    "没有数量",
			content: `{"product":"笔记本"}`,
		},
		{
			name:    "没有商品",
			content: `{"quantity":3}`,
		},
		{
			name:    "错误类型",
			content: `{"product":"笔记本","quantity":"3"}`,
		},
		{
			name:    "数量超限",
			content: `{"product":"笔记本","quantity":101}`,
		},
		{
			name:    "残缺 JSON",
			content: `{"product":"笔记本","quantity":3`,
		},
		{
			name:    "多个对象",
			content: `{"product":"笔记本","quantity":3}{}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatCli := modelFunc(func(_ context.Context, req Request) (*Response, error) {
				if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
					t.Fatal("没有要求模型使用 JSON 输出")
				}
				return &Response{
					Message: Message{
						Role:    RoleAssistant,
						Content: tc.content,
					},
					FinishReason: FinishReasonStop,
				}, nil
			})
			result, err := newTestClient(t, chatCli, DefaultLimits()).DemoStructured(context.Background(), "")
			if (err == nil) != tc.valid || result.Execution.Completed != tc.valid {
				t.Fatalf("实际完成 %v、错误 %v，预期有效 %v", result.Execution.Completed, err, tc.valid)
			}
			if result.Execution.Answer != tc.content || result.Execution.ModelCalls != 1 {
				t.Fatal("校验失败时丢失模型原回答或调用次数")
			}
			if !tc.valid && !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("实际错误 %v，预期模型响应不符合约定", err)
			}
		})
	}
}

func TestDemoWorkflowKeepsCompletedStagesWhenBudgetEnds(t *testing.T) {
	requests := []Request{}
	chatCli := modelFunc(func(_ context.Context, req Request) (*Response, error) {
		requests = append(requests, req)
		return &Response{
			Message: Message{
				Role:    RoleAssistant,
				Content: "第一个实际结果",
			},
			FinishReason: FinishReasonStop,
		}, nil
	})
	limits := DefaultLimits()
	limits.ModelCalls = 2
	agentCli := newTestClient(t, chatCli, limits)
	result, err := agentCli.DemoWorkflow(context.Background(), "")
	if !errors.Is(err, ErrBudgetExceeded) || len(result.Stages) != 2 || result.Execution.Completed || result.Execution.ModelCalls != 2 {
		t.Fatalf("预算失败未保留已完成阶段：%+v，错误 %v", result, err)
	}
	if len(requests) != 2 || !strings.Contains(requests[1].Messages[1].Content, "第一个实际结果") {
		t.Fatal("后一步没有读取前一步实际输出")
	}
}

func TestDemoProductToolRejectsUnknownAndUnsafeArguments(t *testing.T) {
	for _, arguments := range []string{
		`{"product":"未知商品"}`,
		`{"product":"笔记本","product":"其他"}`,
		`{"product":"笔记本","command":"rm"}`,
		`{"product":null}`,
	} {
		result, err := (ProductTools{}).Dispatch(context.Background(), ToolCallReq{
			Type: ToolTypeFunction,
			Function: ToolFunctionCall{
				Name:      ToolNameLookupProduct,
				Arguments: arguments,
			},
		})
		if err != nil || result.OK || result.Error == "" {
			t.Fatalf("实际结果 %+v、错误 %v，预期明确拒绝参数 %s", result, err, arguments)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (ProductTools{}).Dispatch(ctx, ToolCallReq{}); !errors.Is(err, context.Canceled) {
		t.Fatal("取消后仍然执行了商品工具")
	}
}
