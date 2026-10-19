package app

import (
	"context"
	"encoding/json"
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

// demosModel 是只在本机运行的协议替身，根据实际历史决定下一步，不连接外部模型。
func demosModel(t *testing.T, calls *atomic.Int32) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req agent.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"你好\"},\"finish_reason\":null}]}\n\n")
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"，世界\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		message := agent.Message{
			Role:    agent.RoleAssistant,
			Content: "这是实际模拟模型的回答：demo-notebook-001。",
		}
		reason := agent.FinishReasonStop
		if req.ResponseFormat != nil {
			if req.ResponseFormat.Type != "json_object" {
				t.Error("结构化请求没有使用 json_object")
			}
			message.Content = `{"product":"笔记本","quantity":3}`
		} else if len(req.Tools) > 0 {
			question := req.Messages[1].Content
			lookupDone, multiplyDone := false, false
			answer := ""
			for _, item := range req.Messages {
				if item.Role != agent.RoleTool {
					continue
				}
				if strings.Contains(item.Content, `"product"`) {
					lookupDone = true
				} else {
					multiplyDone = true
					var output struct {
						Result json.Number `json:"result"` // 实际工具返回的数字。
					}
					if err := json.Unmarshal([]byte(item.Content), &output); err == nil {
						answer = output.Result.String()
					}
				}
			}
			if !multiplyDone {
				reason = agent.FinishReasonToolCalls
				function := agent.FunctionCall{
					Name:      agent.ToolNameMultiply,
					Arguments: `{"a":17,"b":23}`,
				}
				id := "multiply-1"
				if len(req.Tools) > 1 && !lookupDone {
					id = "lookup-1"
					function = agent.FunctionCall{
						Name:      agent.ToolNameLookupProduct,
						Arguments: `{"product":"笔记本"}`,
					}
				} else if len(req.Tools) > 1 || strings.Contains(question, "4×23") {
					function.Arguments = `{"a":4,"b":23}`
				} else if strings.Contains(question, "8×0") {
					function.Arguments = `{"a":8,"b":0}`
				}
				message.Content = ""
				message.ToolCalls = []agent.ToolCall{
					{
						ID:       id,
						Type:     agent.ToolTypeFunction,
						Function: function,
					},
				}
			} else {
				message.Content = "结果为 " + answer + "。资料来源 demo-notebook-001。"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message":       message,
					"finish_reason": reason,
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}
}

func TestAgentDemosEveryChapter(t *testing.T) {
	var calls atomic.Int32
	router, _ := setup(t, demosModel(t, &calls), routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: 10 * time.Second,
	}, &memoryExporter{})
	for _, tc := range []struct {
		chapter  string // 二级菜单对应的独立接口。
		status   int    // 预期状态，预算演示会有意停止。
		evidence string // 必须来自真实执行的证据。
	}{
		{
			chapter:  "overview",
			status:   200,
			evidence: `"withTools"`,
		},
		{
			chapter:  "chat",
			status:   200,
			evidence: `"model_calls":1`,
		},
		{
			chapter:  "messages",
			status:   200,
			evidence: `"model_calls":2`,
		},
		{
			chapter:  "tools",
			status:   200,
			evidence: "391",
		},
		{
			chapter:  "structured",
			status:   200,
			evidence: `"quantity":3`,
		},
		{
			chapter:  "multiple-tools",
			status:   200,
			evidence: "92",
		},
		{
			chapter:  "stream",
			status:   200,
			evidence: `"delta":"你好"`,
		},
		{
			chapter:  "reliability",
			status:   409,
			evidence: `"tool_calls":1`,
		},
		{
			chapter:  "retrieval",
			status:   200,
			evidence: "demo-notebook-001",
		},
		{
			chapter:  "workflow",
			status:   200,
			evidence: "汇总",
		},
		{
			chapter:  "observation",
			status:   200,
			evidence: `"telemetry_enabled":true`,
		},
		{
			chapter:  "evaluation",
			status:   200,
			evidence: `"passed":3`,
		},
		{
			chapter:  "delivery",
			status:   200,
			evidence: "92",
		},
	} {
		t.Run(tc.chapter, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/"+tc.chapter, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != tc.status || !strings.Contains(out.Body.String(), tc.evidence) {
				t.Fatalf("实际 HTTP %d、%s，预期 HTTP %d 且包含 %s", out.Code, out.Body.String(), tc.status, tc.evidence)
			}
			if tc.chapter == "stream" {
				if !strings.Contains(out.Body.String(), `"done":true`) || !strings.HasPrefix(out.Header().Get("Content-Type"), "text/event-stream") {
					t.Fatal("流式演示没有返回最终事件或 SSE 类型")
				}
				return
			}
			var envelope entity.Rsp[json.RawMessage]
			if err := json.Unmarshal(out.Body.Bytes(), &envelope); err != nil || envelope.TraceID == "" {
				t.Fatalf("响应不是统一 JSON 或没有沿用 HTTP trace：%s", out.Body.String())
			}
		})
	}
	// 新页面不能改变旧接口的路径及响应外层。
	out, old := invoke(router, "chat", `{}`)
	if out.Code != 200 || !old.Data.Completed {
		t.Fatal("旧聊天接口没有保留")
	}
}

func TestAgentDemosRejectInvalidRequestsWithoutCallingModel(t *testing.T) {
	var calls atomic.Int32
	router, _ := setup(t, demosModel(t, &calls), routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, &memoryExporter{})
	for _, tc := range []struct {
		chapter string // 接口名称。
		body    string // 应当被拒绝的请求正文。
	}{
		{
			chapter: "chat",
			body:    `{"question":""}`,
		},
		{
			chapter: "messages",
			body:    `{"questions":[]}`,
		},
		{
			chapter: "messages",
			body:    `{"questions":["只有一句"]}`,
		},
		{
			chapter: "structured",
			body:    `{"extra":1}`,
		},
		{
			chapter: "stream",
			body:    `{"question":"   "}`,
		},
		{
			chapter: "stream",
			body:    `{"question":"a","question":"b"}`,
		},
		{
			chapter: "reliability",
			body:    `{"model_calls":0}`,
		},
		{
			chapter: "reliability",
			body:    `{"model_calls":99999999999}`,
		},
		{
			chapter: "evaluation",
			body:    `{"unknown":true}`,
		},
	} {
		t.Run(tc.chapter+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/"+tc.chapter, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != 400 || strings.HasPrefix(out.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatalf("无效参数实际返回 HTTP %d、%s，预期流建立前返回 400 JSON", out.Code, out.Body.String())
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("无效输入仍调用模型 %d 次", calls.Load())
	}
}

func TestAgentDemosRetrievalWithoutEvidenceDoesNotCallModel(t *testing.T) {
	var calls atomic.Int32
	router, _ := setup(t, demosModel(t, &calls), routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, &memoryExporter{})
	req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/retrieval", strings.NewReader(`{"question":"今天月球天气如何？"}`))
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if out.Code != 200 || calls.Load() != 0 || !strings.Contains(out.Body.String(), `"sources":[]`) || !strings.Contains(out.Body.String(), "没有相关内容") {
		t.Fatalf("未命中时实际 HTTP %d、%s、调用 %d 次，预期明确无资料且不调用模型", out.Code, out.Body.String(), calls.Load())
	}
}

func TestAgentDemosStreamCancellationReachesModel(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	model := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"已收到\"}}]}\n\n")
		w.(http.Flusher).Flush()
		close(started)
		<-req.Context().Done()
		close(stopped)
	}
	router, _ := setup(t, model, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: 5 * time.Second,
	}, &memoryExporter{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/stream", strings.NewReader(`{}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	out := httptest.NewRecorder()
	go func() {
		router.ServeHTTP(out, req)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("流式模型请求没有开始")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("浏览器取消没有传到模型连接")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("取消后接口未结束")
	}
	if strings.Contains(out.Body.String(), `"completed":true`) {
		t.Fatal("被取消的流误报为完整成功")
	}
}
