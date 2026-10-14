package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"sakelei/ai/domain/agent/entity"
)

type modelFunc func(context.Context, entity.ChatRequest) (entity.ChatResponse, error)

func (f modelFunc) Chat(ctx context.Context, request entity.ChatRequest) (entity.ChatResponse, error) {
	return f(ctx, request)
}

func call(id, name, args string) entity.ToolCall {
	return entity.ToolCall{ID: id, Type: "function", Function: entity.FunctionCall{Name: name, Arguments: args}}
}

func toolResponse(calls ...entity.ToolCall) entity.ChatResponse {
	return entity.ChatResponse{Message: entity.Message{Role: "assistant", ToolCalls: calls}, FinishReason: "tool_calls"}
}

func answer(text string) entity.ChatResponse {
	return entity.ChatResponse{Message: entity.Message{Role: "assistant", Content: text}, FinishReason: "stop"}
}

func registry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry([]Tool{MultiplyTool()})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRegistryValidation(t *testing.T) {
	r := registry(t)
	cases := []struct {
		name string
		call entity.ToolCall
		code string
	}{
		{"整数", call("1", "multiply", `{"a":17,"b":23}`), ""},
		{"小数", call("1", "multiply", `{"a":1.5,"b":2}`), ""},
		{"未知工具", call("1", "shell", `{}`), "unknown_tool"},
		{"无效 JSON", call("1", "multiply", `{`), "invalid_json"},
		{"缺失参数", call("1", "multiply", `{"a":1}`), "invalid_arguments"},
		{"额外参数", call("1", "multiply", `{"a":1,"b":2,"c":3}`), "invalid_arguments"},
		{"错误大小写", call("1", "multiply", `{"A":1,"b":2}`), "invalid_arguments"},
		{"字符串", call("1", "multiply", `{"a":"1","b":2}`), "invalid_arguments"},
		{"布尔值", call("1", "multiply", `{"a":true,"b":2}`), "invalid_arguments"},
		{"空值", call("1", "multiply", `{"a":null,"b":2}`), "invalid_arguments"},
		{"数组", call("1", "multiply", `[1,2]`), "invalid_arguments"},
		{"溢出参数", call("1", "multiply", `{"a":1e999,"b":2}`), "invalid_arguments"},
		{"溢出结果", call("1", "multiply", `{"a":1e308,"b":1e308}`), "invalid_arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Dispatch(context.Background(), tc.call)
			if err != nil {
				t.Fatal(err)
			}
			if got.Error != tc.code || got.OK != (tc.code == "") {
				t.Errorf("分发 %s 实际为 %+v，预期错误码 %q", tc.name, got, tc.code)
			}
			if tc.name == "整数" && got.Result != float64(391) {
				t.Errorf("乘积实际为 %v，预期 391", got.Result)
			}
		})
	}
	if _, err := NewRegistry([]Tool{MultiplyTool(), MultiplyTool()}); err == nil {
		t.Error("重名工具未被拒绝")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Dispatch(ctx, call("1", "multiply", `{"a":1,"b":2}`)); !errors.Is(err, context.Canceled) {
		t.Errorf("取消后分发实际错误 %v，预期 context.Canceled", err)
	}
}

func TestAgentMultipleToolsAndMessagePairing(t *testing.T) {
	requests := 0
	model := modelFunc(func(ctx context.Context, req entity.ChatRequest) (entity.ChatResponse, error) {
		requests++
		if requests == 1 {
			if len(req.Tools) != 1 || len(req.Messages) != 2 {
				t.Errorf("首轮请求缺少工具或消息：%+v", req)
			}
			return toolResponse(call("a", "multiply", `{"a":17,"b":23}`), call("b", "multiply", `{"a":12,"b":8}`)), nil
		}
		if len(req.Messages) != 5 {
			t.Fatalf("回填后消息数实际 %d，预期 5", len(req.Messages))
		}
		if len(req.Messages[2].ToolCalls) != 2 {
			t.Error("assistant 工具消息未完整保留")
		}
		for i, id := range []string{"a", "b"} {
			m := req.Messages[3+i]
			if m.Role != "tool" || m.ToolCallID != id || !strings.Contains(m.Content, `"ok":true`) {
				t.Errorf("工具回填实际为 %+v，预期对应 %q", m, id)
			}
		}
		return answer("391 和 96"), nil
	})
	agent, err := New(model, registry(t))
	if err != nil {
		t.Fatal(err)
	}
	result, err := agent.Run(context.Background(), "算两个乘积", "用工具", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if result.ModelCalls != 2 || result.ToolCalls != 2 || len(result.ToolExecutions) != 2 || result.Answer != "391 和 96" {
		t.Errorf("运行结果实际 %+v，预期两次请求、两次工具和完整回答", result)
	}
	if result.UsageComplete {
		t.Error("未提供用量时不应表示完整用量")
	}
}

func TestAgentLimitsAndInvalidIDs(t *testing.T) {
	cases := []struct {
		name    string
		limits  Limits
		calls   []entity.ToolCall
		tokens  int
		budget  bool
		wantRun int
	}{
		{"模型预算", Limits{1, 8, 8000}, []entity.ToolCall{call("1", "multiply", `{"a":1,"b":2}`)}, 0, true, 1},
		{"工具预算整轮拒绝", Limits{5, 1, 8000}, []entity.ToolCall{call("1", "multiply", `{}`), call("2", "multiply", `{}`)}, 0, true, 0},
		{"token 软预算", Limits{5, 8, 10}, []entity.ToolCall{call("1", "multiply", `{"a":1,"b":2}`)}, 10, true, 1},
		{"重复 ID 整轮拒绝", DefaultLimits(), []entity.ToolCall{call("1", "multiply", `{}`), call("1", "multiply", `{}`)}, 0, false, 0},
		{"空 ID", DefaultLimits(), []entity.ToolCall{call("", "multiply", `{}`)}, 0, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modelCalls := 0
			model := modelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) {
				modelCalls++
				r := toolResponse(tc.calls...)
				r.Usage.TotalTokens = &tc.tokens
				return r, nil
			})
			agent, err := New(model, registry(t))
			if err != nil {
				t.Fatal(err)
			}
			result, err := agent.Run(context.Background(), "问题", "规则", tc.limits)
			if err == nil || errors.Is(err, ErrBudgetExceeded) != tc.budget {
				t.Errorf("实际错误 %v，预期预算错误=%v", err, tc.budget)
			}
			if result.ToolCalls != tc.wantRun || modelCalls != 1 {
				t.Errorf("实际工具/模型=%d/%d，预期 %d/1", result.ToolCalls, modelCalls, tc.wantRun)
			}
		})
	}
}

func TestAgentToolFailureIsReturnedToModel(t *testing.T) {
	n := 0
	model := modelFunc(func(ctx context.Context, req entity.ChatRequest) (entity.ChatResponse, error) {
		n++
		if n == 1 {
			return toolResponse(call("1", "multiply", `{"a":"错误","b":2}`)), nil
		}
		var output entity.ToolResult
		if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &output); err != nil {
			t.Fatal(err)
		}
		if output.OK || output.Error != "invalid_arguments" {
			t.Errorf("错误回填实际 %+v，预期 invalid_arguments", output)
		}
		return answer("参数有误，请修改"), nil
	})
	agent, err := New(model, registry(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Run(context.Background(), "问题", "规则", DefaultLimits()); err != nil {
		t.Fatal(err)
	}
}

func TestAgentCancellationAndMalformedCompletion(t *testing.T) {
	for _, response := range []entity.ChatResponse{answer(""), {Message: entity.Message{Role: "assistant", Content: "半句"}, FinishReason: "length"}} {
		agent, err := New(modelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) { return response, nil }), registry(t))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := agent.Run(context.Background(), "问题", "规则", DefaultLimits()); err == nil {
			t.Error("不完整回答被当成最终答案")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	agent, err := New(modelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) {
		return answer("不应调用"), fmt.Errorf("不应执行")
	}), registry(t))
	if err != nil {
		t.Fatal(err)
	}
	result, err := agent.Run(ctx, "问题", "规则", DefaultLimits())
	if !errors.Is(err, context.Canceled) || result.ModelCalls != 0 {
		t.Errorf("取消结果实际 %+v/%v，预期零调用并返回取消", result, err)
	}
}

func TestAgentCountsUsageBeforeReturningModelErrors(t *testing.T) {
	tokens, negative := 37, -1
	modelErr := errors.New("模型调用失败")
	cases := []struct {
		name     string
		usage    entity.Usage
		callErr  error
		want     int
		complete bool
	}{
		{"失败响应有用量", entity.Usage{TotalTokens: &tokens}, modelErr, 37, true},
		{"校验失败有用量", entity.Usage{TotalTokens: &tokens}, nil, 37, true},
		{"失败响应缺少用量", entity.Usage{}, modelErr, 0, false},
		{"失败响应负数用量", entity.Usage{TotalTokens: &negative}, modelErr, 0, false},
		{"总量有效但分项非法", entity.Usage{TotalTokens: &tokens, PromptTokens: &negative}, nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent, err := New(modelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) {
				return entity.ChatResponse{Message: entity.Message{Role: "assistant", Content: "半句"}, FinishReason: "length", Usage: tc.usage}, tc.callErr
			}), registry(t))
			if err != nil {
				t.Fatal(err)
			}
			result, err := agent.Run(context.Background(), "问题", "规则", DefaultLimits())
			if err == nil || (tc.callErr != nil && !errors.Is(err, tc.callErr)) {
				t.Errorf("应返回原始调用错误或校验错误，实际 %v", err)
			}
			if result.ModelCalls != 1 || result.KnownTokens != tc.want || result.UsageComplete != tc.complete || result.Answer != "" || result.ToolCalls != 0 {
				t.Errorf("失败调用统计实际 %+v，预期已知用量 %d、完整=%v、一次模型请求且无工具和最终答案", result, tc.want, tc.complete)
			}
		})
	}
}

func TestAgentSecondFailurePreservesEarlierUsage(t *testing.T) {
	first, second := 10, 37
	for _, usage := range []entity.Usage{{TotalTokens: &second}, {}} {
		requests := 0
		agent, err := New(modelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) {
			requests++
			if requests == 1 {
				response := toolResponse(call("1", "multiply", `{"a":1,"b":2}`))
				response.Usage.TotalTokens = &first
				return response, nil
			}
			return entity.ChatResponse{Usage: usage}, context.DeadlineExceeded
		}), registry(t))
		if err != nil {
			t.Fatal(err)
		}
		result, err := agent.Run(context.Background(), "问题", "规则", DefaultLimits())
		want, complete := 10, false
		if usage.TotalTokens != nil {
			want, complete = 47, true
		}
		if !errors.Is(err, context.DeadlineExceeded) || result.ModelCalls != 2 || result.ToolCalls != 1 || result.KnownTokens != want || result.UsageComplete != complete {
			t.Errorf("第二轮失败应保留两轮用量和已执行工具，实际 %+v / %v", result, err)
		}
	}
}
