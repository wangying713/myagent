package app

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

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/domain/agent"

	"github.com/gin-gonic/gin"
)

func TestAgentQuestionPassThroughAndDefaults(t *testing.T) {
	for _, endpoint := range []struct {
		path     string // 普通 API 或 Demo 的请求地址。
		fallback string // 该接口在问题为空时使用的领域默认值。
	}{
		{
			path:     "/api/agent/chat",
			fallback: agent.ChatQuestion,
		},
		{
			path:     "/api/agent/run",
			fallback: agent.AgentQuestion,
		},
		{
			path:     "/api/agent-demos/chat",
			fallback: agent.ChatQuestion,
		},
		{
			path:     "/api/agent-demos/tools",
			fallback: agent.AgentQuestion,
		},
	} {
		for _, input := range []struct {
			name     string // 输入场景。
			body     string // 客户端实际发送的 JSON。
			question string // 模型应收到的完整问题。
		}{
			{
				name:     "保留问题原文",
				body:     `{"question":"  请计算 17×23，保留这句话。  "}`,
				question: "  请计算 17×23，保留这句话。  ",
			},
			{
				name:     "省略问题交给领域默认值",
				body:     `{}`,
				question: endpoint.fallback,
			},
			{
				name:     "空字符串交给领域默认值",
				body:     `{"question":""}`,
				question: endpoint.fallback,
			},
		} {
			t.Run(endpoint.path+"/"+input.name, func(t *testing.T) {
				calls := 0
				router, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					var request agent.Request
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
					}
					if len(request.Messages) != 2 || request.Messages[1].Content != input.question {
						t.Errorf("实际模型消息 %+v，预期问题原文 %q", request.Messages, input.question)
					}
					reply(w, map[string]string{
						"role":    "assistant",
						"content": "完成",
					}, "stop")
				}, routerTestConfig{
					Limits:  agent.DefaultLimits(),
					Timeout: time.Second,
				}, &memoryExporter{})
				req := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(input.body))
				req.Header.Set("Content-Type", "application/json")
				out := httptest.NewRecorder()
				router.ServeHTTP(out, req)
				if out.Code != http.StatusOK || calls != 1 {
					t.Fatalf("实际 HTTP %d、调用 %d 次、响应 %s，预期成功调用模型一次", out.Code, calls, out.Body.String())
				}
			})
		}
	}
}

func TestResponseJSONCompatibility(t *testing.T) {
	zero, total := int64(0), int64(12)
	steps := []*proto.ExecutionStep{
		{
			Kind:         "model",
			Name:         "模型",
			Request:      "请求",
			Response:     "回答",
			ElapsedMs:    11,
			FinishReason: "stop",
			Usage: &proto.Usage{
				PromptTokens: &zero,
				TotalTokens:  &total,
			},
		},
		{
			Kind:      "tool",
			Name:      "multiply",
			Request:   `{"a":17,"b":23}`,
			Response:  "391",
			ElapsedMs: 1,
			Error:     "步骤说明",
		},
	}
	for _, tc := range []struct {
		name     string             // 用例名称。
		chat     *proto.ChatRsp     // 聊天接口的模拟 Proto 响应。
		messages *proto.MessagesRsp // 消息历史接口的模拟 Proto 响应。
		run      *proto.RunRsp      // 工具循环接口的模拟 Proto 响应。
		want     string             // 预期 HTTP JSON 响应正文。
	}{
		{
			name: "空响应",
			want: `{"code":"ok","traceID":"","msg":"成功","data":null}`,
		},
		{
			name:     "零值响应",
			chat:     &proto.ChatRsp{},
			messages: &proto.MessagesRsp{},
			run:      &proto.RunRsp{},
			want:     `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
		},
		{
			name: "业务开始后的空步骤",
			chat: &proto.ChatRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []*proto.ExecutionStep{},
			},
			messages: &proto.MessagesRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []*proto.ExecutionStep{},
			},
			run: &proto.RunRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []*proto.ExecutionStep{},
			},
			want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"chat","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":true,"steps":[]}}`,
		},
		{
			name: "完整字段与部分未知用量",
			chat: &proto.ChatRsp{
				Lesson:           "agent",
				Completed:        true,
				Answer:           "结果为 391",
				TelemetryEnabled: true,
				ElapsedMs:        42,
				ModelCalls:       2,
				ToolCalls:        1,
				KnownTokens:      12,
				Steps:            steps,
			},
			messages: &proto.MessagesRsp{
				Lesson:           "agent",
				Completed:        true,
				Answer:           "结果为 391",
				TelemetryEnabled: true,
				ElapsedMs:        42,
				ModelCalls:       2,
				ToolCalls:        1,
				KnownTokens:      12,
				Steps:            steps,
			},
			run: &proto.RunRsp{
				Lesson:           "agent",
				Completed:        true,
				Answer:           "结果为 391",
				TelemetryEnabled: true,
				ElapsedMs:        42,
				ModelCalls:       2,
				ToolCalls:        1,
				KnownTokens:      12,
				Steps:            steps,
			},
			want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"agent","completed":true,"answer":"结果为 391","telemetry_enabled":true,"elapsed_ms":42,"model_calls":2,"tool_calls":1,"known_tokens":12,"usage_complete":false,"steps":[{"kind":"model","name":"模型","request":"请求","response":"回答","elapsed_ms":11,"finish_reason":"stop","usage":{"prompt_tokens":0,"completion_tokens":null,"total_tokens":12}},{"kind":"tool","name":"multiply","request":"{\"a\":17,\"b\":23}","response":"391","elapsed_ms":1,"error":"步骤说明"}]}}`,
		},
		{
			name: "大整数保留 JSON 数字",
			chat: &proto.ChatRsp{
				ElapsedMs: 9007199254740993,
			},
			messages: &proto.MessagesRsp{
				ElapsedMs: 9007199254740993,
			},
			run: &proto.RunRsp{
				ElapsedMs: 9007199254740993,
			},
			want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":9007199254740993,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(requestTimeout(time.Second))
			router.POST("/chat", handle(func(context.Context, *proto.ChatReq) (*proto.ChatRsp, error) {
				return tc.chat, nil
			}))
			router.POST("/messages", handle(func(context.Context, *proto.MessagesReq) (*proto.MessagesRsp, error) {
				return tc.messages, nil
			}))
			router.POST("/run", handle(func(context.Context, *proto.RunReq) (*proto.RunRsp, error) {
				return tc.run, nil
			}))
			for _, path := range []string{
				"/chat",
				"/messages",
				"/run",
			} {
				t.Run(path, func(t *testing.T) {
					out := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
					req.Header.Set("Content-Type", "application/json")
					router.ServeHTTP(out, req)
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
		})
	}
}
