package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
)

// resultStreamFunc 提供支持同步回调的离线文本流替身。
type resultStreamFunc func(context.Context, agent.StreamInput) (*agent.Response, error)

// Chat 只用于满足领域客户端的非流式依赖，本测试不应调用。
func (resultStreamFunc) Chat(context.Context, agent.Request) (*agent.Response, error) {
	return nil, agent.ErrInvalidResponse
}

// Stream 按测试场景同步发送模型文本。
func (f resultStreamFunc) Stream(ctx context.Context, req agent.StreamInput) (*agent.Response, error) {
	return f(ctx, req)
}

func TestStreamTailHonorsConsumerFailureAndCancellation(t *testing.T) {
	for _, canceled := range []bool{
		true,
		false,
	} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			service := newResultTestService(t, resultStreamFunc(func(_ context.Context, req agent.StreamInput) (*agent.Response, error) {
				// 使用尚未完整的鉴权标记验证尾部刷新，普通单词应实时输出。
				if err := req.Emit("Bear"); err != nil {
					t.Fatalf("尾部边界未确定时不应写出：%v", err)
				}
				if canceled {
					cancel()
				}
				return &agent.Response{
					Message: agent.Message{
						Role:    agent.RoleAssistant,
						Content: "Bear",
					},
					FinishReason: agent.FinishReasonStop,
				}, ctx.Err()
			}))
			writes := 0
			result, err := service.DemoStream(ctx, agent.DemoStreamInput{
				Emit: func(string) error {
					writes++
					return context.Canceled
				},
			})
			if !errors.Is(err, context.Canceled) || result == nil || result.Completed || result.Answer != "Bear" {
				t.Fatalf("实际结果 %+v、错误 %v，预期保留内容并标记未完成", result, err)
			}
			if canceled && writes != 0 || !canceled && writes != 1 {
				t.Fatalf("实际写入 %d 次，预期取消时不刷新、否则只刷新一次", writes)
			}
		})
	}
}

func TestStreamRedactsSplitSecretsAndKeepsPartialAnswer(t *testing.T) {
	for _, complete := range []bool{
		true,
		false,
	} {
		t.Run(fmt.Sprint(complete), func(t *testing.T) {
			handler, _ := setup(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, text := range []string{
					"普通内容。test-api-",
					"key",
				} {
					encoded, _ := json.Marshal(text)
					fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
				}
				if complete {
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			}, routerTestConfig{
				Limits:  agent.DefaultLimits(),
				Timeout: time.Second,
			}, &memoryExporter{})
			req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/stream", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			var text string
			var final entity.Rsp[entity.DemosStreamRsp]
			for _, event := range strings.Split(out.Body.String(), "\n\n") {
				if event == "" {
					continue
				}
				var rsp entity.Rsp[entity.DemosStreamRsp]
				if err := json.Unmarshal([]byte(strings.TrimPrefix(event, "data: ")), &rsp); err != nil {
					t.Fatal(err)
				}
				text += rsp.Data.Delta
				if rsp.Data.Done {
					final = rsp
				}
			}
			if text != "普通内容。[REDACTED]" || !final.Data.Done || final.Data.Execution.Answer != text {
				t.Fatalf("实际实时回答 %q、最终事件 %+v，预期脱敏并保留完整或部分回答", text, final)
			}
			if (final.Code == entity.CodeOK) != complete || final.Data.Execution.Completed != complete {
				t.Fatalf("实际完成状态 %+v，预期成功=%v", final, complete)
			}
			if strings.Contains(final.Data.Execution.Calls[0].Response, "test-api-") {
				t.Fatal("最终执行记录泄漏跨片段的敏感值")
			}
		})
	}
}

func TestReliabilityUsesConfiguredLimit(t *testing.T) {
	limits := agent.DefaultLimits()
	limits.ModelCalls = 10
	var calls atomic.Int32
	handler, _ := setup(t, demosModel(t, &calls), routerTestConfig{
		Limits:  limits,
		Timeout: time.Second,
	}, &memoryExporter{})
	for _, tc := range []struct {
		body   string // 本次请求明确指定的模型调用上限。
		status int    // 根据实际服务配置决定的响应状态。
	}{
		{
			body:   `{"model_calls":6}`,
			status: http.StatusOK,
		},
		{
			body:   `{"model_calls":11}`,
			status: http.StatusBadRequest,
		},
		{
			body:   `{"model_calls":0}`,
			status: http.StatusBadRequest,
		},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/reliability", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != tc.status {
			t.Errorf("请求 %s 实际 HTTP %d，预期 %d", tc.body, out.Code, tc.status)
		}
	}
}
