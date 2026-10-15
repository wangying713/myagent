package agentapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

func TestResponseJSONCompatibility(t *testing.T) {
	adapter, err := NewAgentAdapter(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	zero, total := 0, 12
	for _, tc := range []struct {
		name   string
		result agent.ExecutionResult
		want   string
	}{
		{
			name: "解析失败的零值数据",
			want: `{"data":{"lesson":"","completed":false,"answer":"","trace_id":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
		},
		{
			name:   "业务开始后的空步骤",
			result: agent.ExecutionResult{Operation: "chat", UsageComplete: true, Steps: []agent.ExecutionStep{}},
			want:   `{"data":{"lesson":"chat","completed":false,"answer":"","trace_id":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":true,"steps":[]}}`,
		},
		{
			name: "完整字段与部分未知用量",
			result: agent.ExecutionResult{
				Operation: "agent", Completed: true, Answer: "结果为 391", TraceID: "trace-id", TelemetryEnabled: true,
				ElapsedMS: 42, ModelCalls: 2, ToolCalls: 1, KnownTokens: 12, UsageComplete: false,
				Steps: []agent.ExecutionStep{
					{Kind: "model", Name: "模型", Request: "请求", Response: "回答", ElapsedMS: 11, FinishReason: "stop", Usage: &agent.Usage{PromptTokens: &zero, TotalTokens: &total}},
					{Kind: "tool", Name: "multiply", Request: `{"a":17,"b":23}`, Response: "391", ElapsedMS: 1, Error: "步骤说明"},
				},
			},
			want: `{"data":{"lesson":"agent","completed":true,"answer":"结果为 391","trace_id":"trace-id","telemetry_enabled":true,"elapsed_ms":42,"model_calls":2,"tool_calls":1,"known_tokens":12,"usage_complete":false,"steps":[{"kind":"model","name":"模型","request":"请求","response":"回答","elapsed_ms":11,"finish_reason":"stop","usage":{"prompt_tokens":0,"completion_tokens":null,"total_tokens":12}},{"kind":"tool","name":"multiply","request":"{\"a\":17,\"b\":23}","response":"391","elapsed_ms":1,"error":"步骤说明"}]}}`,
		},
		{
			name:   "大整数保留 JSON 数字",
			result: agent.ExecutionResult{ElapsedMS: 9007199254740993},
			want:   `{"data":{"lesson":"","completed":false,"answer":"","trace_id":"","telemetry_enabled":false,"elapsed_ms":9007199254740993,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/chat", httpx.Adapt(adapter, func(context.Context, *entity.ChatRequest) (*entity.ExecutionResult, error) {
				return resultMessage(tc.result), nil
			}))
			out := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(out, request)
			if out.Code != http.StatusOK {
				t.Fatalf("实际 HTTP %d，响应 %s，预期 200", out.Code, out.Body.String())
			}
			// 使用 Number 避免测试自身的 float64 转换掩盖整数精度问题。
			var got, want any
			actual := json.NewDecoder(bytes.NewReader(out.Body.Bytes()))
			actual.UseNumber()
			if err := actual.Decode(&got); err != nil {
				t.Fatal(err)
			}
			expected := json.NewDecoder(strings.NewReader(tc.want))
			expected.UseNumber()
			if err := expected.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("实际 JSON %s，预期 %s", out.Body.String(), tc.want)
			}
		})
	}
}
