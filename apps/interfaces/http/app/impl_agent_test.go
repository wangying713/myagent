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

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"

	"github.com/gin-gonic/gin"
)

func TestAgentQuestionPassThroughAndDefaults(t *testing.T) {
	for _, endpoint := range []struct {
		path     string // 普通 API 或 Demo 的请求地址。
		fallback string // 该接口在问题为空时使用的学习示例。
	}{
		{
			path:     "/api/agent/chat",
			fallback: agent.DemoChatQuestion,
		},
		{
			path:     "/api/agent/run",
			fallback: agent.AgentQuestion,
		},
		{
			path:     "/api/agent-demos/chat",
			fallback: agent.DemoChatQuestion,
		},
		{
			path:     "/api/agent-demos/tools",
			fallback: agent.AgentQuestion,
		},
		{
			path:     "/api/agent-demos/observation",
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
				name:     "省略问题使用接口默认示例",
				body:     `{}`,
				question: endpoint.fallback,
			},
			{
				name:     "空字符串使用接口默认示例",
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
					if endpoint.fallback == agent.AgentQuestion {
						if len(request.Messages) == 0 || request.Messages[0].Content != agent.AgentPrompt || request.ToolChoice != agent.ToolChoiceAuto || len(request.Tools) != 1 || request.Tools[0].Function.Name != agent.ToolNameMultiply {
							t.Errorf("工具接口未保留计算助手的提示词、乘法工具或自动调用策略：%+v", request)
						}
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

// responseCase 保存三个独立 API 的响应及其共同的 JSON 预期。
type responseCase struct {
	name     string              // 场景名称。
	chat     *entity.ChatRsp     // 聊天接口的模拟响应；nil 表示无结果。
	messages *entity.MessagesRsp // 消息历史接口的模拟响应；nil 表示无结果。
	run      *entity.RunRsp      // 工具循环接口的模拟响应；nil 表示无结果。
	want     string              // 预期 HTTP JSON 正文。
}

func TestResponseJSONCompatibility(t *testing.T) {
	cases := basicResponseCases()
	cases = append(cases, fullResponseCase(), largeIntegerResponseCase())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(requestTimeout(time.Second))
			router.POST("/chat", handle(func(context.Context, *entity.ChatReq) (*entity.ChatRsp, error) {
				return tc.chat, nil
			}))
			router.POST("/messages", handle(func(context.Context, *entity.MessagesReq) (*entity.MessagesRsp, error) {
				return tc.messages, nil
			}))
			router.POST("/run", handle(func(context.Context, *entity.RunReq) (*entity.RunRsp, error) {
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

func basicResponseCases() []responseCase {
	return []responseCase{
		{
			name: "空响应",
			want: `{"code":"ok","traceID":"","msg":"成功","data":null}`,
		},
		{
			name:     "零值响应",
			chat:     &entity.ChatRsp{},
			messages: &entity.MessagesRsp{},
			run:      &entity.RunRsp{},
			want:     `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
		},
		{
			name: "业务开始后的空步骤",
			chat: &entity.ChatRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []entity.AgentExecutionStep{},
			},
			messages: &entity.MessagesRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []entity.AgentExecutionStep{},
			},
			run: &entity.RunRsp{
				Lesson:        "chat",
				UsageComplete: true,
				Steps:         []entity.AgentExecutionStep{},
			},
			want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"chat","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":0,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":true,"steps":[]}}`,
		},
	}
}

func fullResponseCase() responseCase {
	zero, total := 0, 12
	steps := []entity.AgentExecutionStep{
		{
			Kind:         "model",
			Name:         "模型",
			Request:      "请求",
			Response:     "回答",
			ElapsedMS:    11,
			FinishReason: "stop",
			Usage: &entity.AgentUsage{
				PromptTokens: &zero,
				TotalTokens:  &total,
			},
		},
		{
			Kind:      "tool",
			Name:      "multiply",
			Request:   `{"a":17,"b":23}`,
			Response:  "391",
			ElapsedMS: 1,
			Error:     "步骤说明",
		},
	}
	return responseCase{
		name: "完整字段与部分未知用量",
		chat: &entity.ChatRsp{
			Lesson:           "agent",
			Completed:        true,
			Answer:           "结果为 391",
			TelemetryEnabled: true,
			ElapsedMS:        42,
			ModelCalls:       2,
			ToolCalls:        1,
			KnownTokens:      12,
			Steps:            steps,
		},
		messages: &entity.MessagesRsp{
			Lesson:           "agent",
			Completed:        true,
			Answer:           "结果为 391",
			TelemetryEnabled: true,
			ElapsedMS:        42,
			ModelCalls:       2,
			ToolCalls:        1,
			KnownTokens:      12,
			Steps:            steps,
		},
		run: &entity.RunRsp{
			Lesson:           "agent",
			Completed:        true,
			Answer:           "结果为 391",
			TelemetryEnabled: true,
			ElapsedMS:        42,
			ModelCalls:       2,
			ToolCalls:        1,
			KnownTokens:      12,
			Steps:            steps,
		},
		want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"agent","completed":true,"answer":"结果为 391","telemetry_enabled":true,"elapsed_ms":42,"model_calls":2,"tool_calls":1,"known_tokens":12,"usage_complete":false,"steps":[{"kind":"model","name":"模型","request":"请求","response":"回答","elapsed_ms":11,"finish_reason":"stop","usage":{"prompt_tokens":0,"completion_tokens":null,"total_tokens":12}},{"kind":"tool","name":"multiply","request":"{\"a\":17,\"b\":23}","response":"391","elapsed_ms":1,"error":"步骤说明"}]}}`,
	}
}

func largeIntegerResponseCase() responseCase {
	return responseCase{
		name: "大整数保留 JSON 数字",
		chat: &entity.ChatRsp{
			ElapsedMS: 9007199254740993,
		},
		messages: &entity.MessagesRsp{
			ElapsedMS: 9007199254740993,
		},
		run: &entity.RunRsp{
			ElapsedMS: 9007199254740993,
		},
		want: `{"code":"ok","traceID":"","msg":"成功","data":{"lesson":"","completed":false,"answer":"","telemetry_enabled":false,"elapsed_ms":9007199254740993,"model_calls":0,"tool_calls":0,"known_tokens":0,"usage_complete":false,"steps":null}}`,
	}
}
