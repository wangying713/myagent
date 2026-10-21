package openobserve

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"saklay/ai/domain/agent"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func traceAttributes(t *testing.T, observeCli *Client, exporter *tracetest.InMemoryExporter, spanName string) (map[string]attribute.Value, codes.Code) {
	t.Helper()
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatal(err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Name != spanName {
		t.Fatalf("实际导出 %v，预期一个名称为 %q 的 span", spans, spanName)
	}
	attrs := make(map[string]attribute.Value)
	for _, attr := range spans[0].Attributes {
		attrs[string(attr.Key)] = attr.Value
	}
	return attrs, spans[0].Status.Code
}

func TestTracePreviewInputsAndPartialOutput(t *testing.T) {
	for _, tc := range []struct {
		name, operation, wantInput string   // 用例名称、业务操作标识与预期输入预览。
		questions                  []string // 传给运行观测的问题列表。
		err                        error    // 模拟业务执行结束时的错误。
	}{
		{
			name:      "单轮问题",
			operation: "chat",
			questions: []string{"问题 configured-secret"},
			wantInput: `{"question":"问题 [REDACTED]"}`,
		},
		{
			name:      "历史消息失败保留部分回答",
			operation: "messages",
			questions: []string{
				`{"password":"nested-secret","a":9007199254740993}`,
				"第二轮 configured-secret",
			},
			wantInput: `{"questions":["{\"a\":9007199254740993,\"password\":\"[REDACTED]\"}","第二轮 [REDACTED]"]}`,
			err:       errors.New("底层错误 error-secret"),
		},
		{
			name:      "工具循环问题",
			operation: "agent",
			questions: []string{"乘法 configured-secret"},
			wantInput: `{"question":"乘法 [REDACTED]"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeCli, exporter := newTestClient(t, true, 4096)
			original := slices.Clone(tc.questions)
			_, runTrace := observeCli.StartTrace(t.Context(), "agent.run", tc.operation)
			input := agent.RunInput{
				Questions: tc.questions,
				Prompt:    "提示 configured-secret",
				Limits: agent.Limits{
					ModelCalls: 2,
					ToolCalls:  3,
					Tokens:     100,
				},
			}
			result := &agent.ChatRsp{
				Input:         input,
				Completed:     tc.err == nil,
				Answer:        "已有回答 configured-secret",
				ModelCalls:    1,
				KnownTokens:   10,
				UsageComplete: true,
			}
			runTrace.End(result, tc.err)
			attrs, status := traceAttributes(t, observeCli, exporter, "agent.run")
			assertTracePreviews(t, attrs, tc.wantInput, result)

			if (status == codes.Error) != (tc.err != nil) {
				t.Errorf("实际 span 状态 %v，预期失败=%v", status, tc.err != nil)
			}
			if attrs["app.llm_calls"].AsInt64() != 1 || attrs["app.total_tokens"].AsInt64() != 10 || attrs["app.lesson"].AsString() != tc.operation {
				t.Error("运行预览改变了既有统计或操作属性")
			}
			for _, pair := range [][2]string{
				{
					"gen_ai.input.messages",
					"app.request",
				},
				{
					"gen_ai.output.messages",
					"app.response",
				},
			} {
				if attrs[pair[0]].AsString() != attrs[pair[1]].AsString() {
					t.Errorf("预览 %s 未直接复用 %s", pair[0], pair[1])
				}
			}
			for _, value := range attrs {
				if strings.Contains(value.AsString(), "error-secret") || strings.Contains(value.AsString(), "configured-secret") || strings.Contains(value.AsString(), "nested-secret") {
					t.Error("运行属性泄漏敏感正文或底层错误")
				}
			}
			if !slices.Equal(tc.questions, original) || result.Answer != "已有回答 [REDACTED]" {
				t.Error("结束观测应保留实际输入，并对返回回答脱敏")
			}
		})
	}
}

// assertTracePreviews 检查运行输入预览和脱敏后的输出预览。
func assertTracePreviews(t *testing.T, attrs map[string]attribute.Value, wantQuestion string, result *agent.ChatRsp) {
	t.Helper()
	var input map[string]any
	if err := json.Unmarshal([]byte(wantQuestion), &input); err != nil {
		t.Fatal(err)
	}
	input["prompt"] = "提示 [REDACTED]"
	input["tools"] = []any{}
	input["limits"] = map[string]int{
		"model_calls": 2,
		"tool_calls":  3,
		"tokens":      100,
	}
	wantInput, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if attrs["gen_ai.input.messages"].AsString() != string(wantInput) {
		t.Errorf("实际输入预览 %s，预期 %s", attrs["gen_ai.input.messages"].AsString(), wantInput)
	}
	var output struct {
		Completed bool   `json:"completed"` // 从预览中解码的业务完成状态。
		Answer    string `json:"answer"`    // 从预览中解码的脱敏回答。
	}
	if err := json.Unmarshal([]byte(attrs["gen_ai.output.messages"].AsString()), &output); err != nil {
		t.Fatal(err)
	}
	if output.Completed != result.Completed || output.Answer != "已有回答 [REDACTED]" {
		t.Errorf("实际输出预览 %+v，预期保留完成状态与脱敏后的已有回答", output)
	}
}

func TestTracePreviewContentControlAndTruncation(t *testing.T) {
	for _, capture := range []bool{
		true,
		false,
	} {
		t.Run(map[bool]string{
			true:  "采集并截断",
			false: "关闭正文采集",
		}[capture], func(t *testing.T) {
			observeCli, exporter := newTestClient(t, capture, 256)
			_, runTrace := observeCli.StartTrace(t.Context(), "agent.run", "chat")
			runTrace.End(&agent.ChatRsp{
				Input: agent.RunInput{
					Questions: []string{strings.Repeat("输入", 200)},
				},
				Completed:   true,
				Answer:      strings.Repeat("输出", 200),
				ModelCalls:  1,
				ToolCalls:   2,
				KnownTokens: 10,
			}, nil)
			attrs, _ := traceAttributes(t, observeCli, exporter, "agent.run")
			if attrs["app.llm_calls"].AsInt64() != 1 || attrs["app.tool_calls"].AsInt64() != 2 || attrs["app.total_tokens"].AsInt64() != 10 {
				t.Error("正文采集开关改变了运行统计")
			}
			for _, pair := range [][2]string{
				{
					"gen_ai.input.messages",
					"app.request",
				},
				{
					"gen_ai.output.messages",
					"app.response",
				},
			} {
				preview, exists := attrs[pair[0]]
				payload, payloadExists := attrs[pair[1]]
				truncated, truncatedExists := attrs[pair[1]+".truncated"]
				if !capture {
					if exists || payloadExists || truncatedExists {
						t.Error("关闭正文采集仍记录了运行预览")
					}
					continue
				}
				text := preview.AsString()
				if !exists || !payloadExists || text != payload.AsString() || !truncated.AsBool() || !strings.HasSuffix(text, "…[TRUNCATED]") || utf8.RuneCountInString(text) != 256+utf8.RuneCountInString("…[TRUNCATED]") {
					t.Errorf("实际预览 %q，预期统一截断至 256 字符并保留标记", text)
				}
			}
		})
	}
}

func TestTraceUsesCallerSpanNameWithoutResult(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	_, runTrace := observeCli.StartTrace(t.Context(), "chat.failed", agent.OperationChat)
	runTrace.End(nil, agent.ErrInput)
	attrs, status := traceAttributes(t, observeCli, exporter, "chat.failed")
	if status != codes.Error {
		t.Fatalf("实际状态 %v，预期没有结果的失败仍结束观测并标记错误", status)
	}
	if _, ok := attrs["app.response"]; ok {
		t.Fatal("缺少结果时不应生成虚假的输出预览")
	}
}

func TestTraceCapturesDeadline(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	_, runTrace := observeCli.StartTrace(ctx, "agent.run", agent.OperationChat)
	runTrace.End(&agent.ChatRsp{}, nil)
	attrs, _ := traceAttributes(t, observeCli, exporter, "agent.run")
	var input struct {
		Deadline string `json:"deadline"` // 观测记录中的 UTC 截止时间。
	}
	if err := json.Unmarshal([]byte(attrs["app.request"].AsString()), &input); err != nil {
		t.Fatal(err)
	}
	if input.Deadline != deadline.UTC().Format(time.RFC3339Nano) {
		t.Errorf("实际截止时间 %q，预期 %q", input.Deadline, deadline.UTC().Format(time.RFC3339Nano))
	}
}

func TestTraceCallPreviewPreservesJSONAndPartialText(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	_, runTrace := observeCli.StartTrace(t.Context(), "agent.run", agent.OperationRun)
	result := &agent.ChatRsp{
		Input: agent.RunInput{
			Tools: []agent.ToolDefinition{
				{
					Type: agent.ToolTypeFunction,
					Function: agent.FunctionDefinition{
						Name:       "test-tool",
						Parameters: json.RawMessage(`{"minimum":9007199254740993}`),
					},
				},
			},
		},
		Calls: []agent.CallRecord{
			{
				Kind:      agent.CallKindModel,
				Name:      "test-model",
				Request:   `{"count":9007199254740993,"password":"secret"}`,
				Response:  "响应片段 configured-secret",
				ElapsedMS: 12,
			},
		},
	}
	runTrace.End(result, agent.ErrInvalidResponse)
	attrs, _ := traceAttributes(t, observeCli, exporter, "agent.run")
	input := attrs["app.request"].AsString()
	if !strings.Contains(input, `"parameters":{"minimum":9007199254740993}`) {
		t.Errorf("实际输入 %s，预期工具参数保持对象形式及大整数精度", input)
	}
	output := attrs["app.response"].AsString()
	for _, want := range []string{
		`"request":{"count":9007199254740993,"password":"[REDACTED]"}`,
		`"response":"响应片段 [REDACTED]"`,
		`"elapsed_ms":12`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("实际输出 %s，预期包含 %s", output, want)
		}
	}
	if result.Calls[0].Response != "响应片段 [REDACTED]" {
		t.Error("结束观测未对返回调用记录脱敏")
	}
}

func TestTracePreviewWithoutCallsUsesEmptyArray(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	_, runTrace := observeCli.StartTrace(t.Context(), "agent.run", agent.OperationChat)
	runTrace.End(&agent.ChatRsp{}, nil)
	attrs, _ := traceAttributes(t, observeCli, exporter, "agent.run")
	if !strings.Contains(attrs["app.response"].AsString(), `"steps":[]`) {
		t.Errorf("实际输出 %s，预期无调用时 steps 为空数组", attrs["app.response"].AsString())
	}
}

func TestTraceEndRedactsResultRegardlessOfCapture(t *testing.T) {
	for _, tc := range []struct {
		name    string // 观测及正文采集开关的组合名称。
		enabled bool   // 是否启用链路上报。
		capture bool   // 是否采集 span 正文。
	}{
		{
			name:    "开启观测和正文",
			enabled: true,
			capture: true,
		},
		{
			name:    "关闭正文采集",
			enabled: true,
		},
		{
			name: "关闭观测",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeCli, err := New(t.Context(), Config{
				Enabled:        tc.enabled,
				ServiceName:    "test",
				TracerName:     "test",
				CaptureContent: tc.capture,
				MaxChars:       256,
				Secrets:        []string{"configured-secret"},
			}, tracetest.NewInMemoryExporter(), nil)
			if err != nil {
				t.Fatal(err)
			}
			registerClientCleanup(t, observeCli)
			answer := strings.Repeat("回答", 200) + " configured-secret"
			original := []agent.CallRecord{
				{
					Kind:     agent.CallKindModel,
					Name:     "configured-secret",
					Request:  `{"password":"secret","count":9007199254740993}`,
					Response: strings.Repeat("响应", 200),
					Error:    "configured-secret",
				},
			}
			result := &agent.ChatRsp{
				Input: agent.RunInput{
					Questions: []string{"configured-secret"},
				},
				Answer:     answer,
				Calls:      original,
				ModelCalls: 1,
			}
			_, runTrace := observeCli.StartTrace(t.Context(), "agent.run", agent.OperationChat)
			runTrace.End(result, agent.ErrInvalidResponse)
			if result.Answer != strings.Repeat("回答", 200)+" [REDACTED]" {
				t.Error("返回回答应脱敏并保留超过正文上限的完整内容")
			}
			record := result.Calls[0]
			if record.Name != "[REDACTED]" || record.Error != "[REDACTED]" || record.Request != `{"count":9007199254740993,"password":"[REDACTED]"}` {
				t.Errorf("返回调用记录未正确脱敏或丢失数值精度：%+v", record)
			}
			if record.Response != strings.Repeat("响应", 128)+"…[TRUNCATED]" {
				t.Errorf("实际正文 %q，预期截断至 256 字符并保留标记", record.Response)
			}
			if original[0].Name != "configured-secret" || original[0].Response != strings.Repeat("响应", 200) {
				t.Error("结束观测覆盖了原始调用记录")
			}
			if result.Input.Questions[0] != "configured-secret" || result.ModelCalls != 1 {
				t.Error("展示脱敏改变了实际输入或调用统计")
			}
		})
	}
}
