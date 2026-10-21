package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"saklay/ai/domain/agent"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestMessagesRedactsOutputWithoutChangingHistory(t *testing.T) {
	var calls atomic.Int32
	exporter := &memoryExporter{}
	handler, observeCli := setup(t, func(w http.ResponseWriter, r *http.Request) {
		var req agent.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if calls.Add(1) == 2 {
			if len(req.Messages) != 4 || req.Messages[2].Content != "第一轮 test-api-key" {
				t.Errorf("实际历史 %+v，预期保留第一轮原始回答", req.Messages)
			}
		}
		reply(w, agent.Message{
			Role:    agent.RoleAssistant,
			Content: "第一轮 test-api-key",
		}, agent.FinishReasonStop)
	}, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, exporter)
	out, rsp := invoke(handler, "messages", `{"questions":["原问题 test-api-key","继续回答"]}`)
	if out.Code != http.StatusOK || calls.Load() != 2 || rsp.Data.Answer != "第一轮 [REDACTED]" {
		t.Fatalf("实际 HTTP %d、响应 %s、调用 %d 次，预期历史完整且回答脱敏", out.Code, out.Body.String(), calls.Load())
	}
	if strings.Contains(out.Body.String(), "test-api-key") {
		t.Fatal("HTTP 回答或调用记录泄漏原始内容")
	}
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatal(err)
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	for _, span := range exporter.spans {
		for _, attr := range span.Attributes() {
			if strings.Contains(attr.Value.AsString(), "test-api-key") {
				t.Errorf("span %s 的 %s 泄漏原始内容", span.Name(), attr.Key)
			}
		}
	}
}

func TestWorkflowRedactsStagesWithoutChangingNextInput(t *testing.T) {
	var calls atomic.Int32
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		var req agent.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if calls.Add(1) > 1 && (len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "阶段原文 test-api-key")) {
			t.Errorf("实际下一阶段输入 %+v，预期保留上一阶段原文", req.Messages)
		}
		reply(w, agent.Message{
			Role:    agent.RoleAssistant,
			Content: "阶段原文 test-api-key",
		}, agent.FinishReasonStop)
	}, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, &memoryExporter{})
	req := httptest.NewRequest(http.MethodPost, "/api/agent-demos/workflow", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != http.StatusOK || calls.Load() != 3 || strings.Contains(out.Body.String(), "test-api-key") || !strings.Contains(out.Body.String(), "[REDACTED]") {
		t.Fatalf("实际 HTTP %d、调用 %d 次、响应 %s，预期保留真实阶段输入并对展示脱敏", out.Code, calls.Load(), out.Body.String())
	}
}

func TestToolDemosIncludeExecutionSpans(t *testing.T) {
	for _, tc := range []struct {
		path  string // 工具案例的请求地址。
		runs  int    // 应建立的独立运行 span 数量。
		tools int    // 实际执行并建立 span 的工具调用次数。
	}{
		{
			path:  "/api/agent-demos/overview",
			runs:  2,
			tools: 1,
		},
		{
			path:  "/api/agent-demos/evaluation",
			runs:  3,
			tools: 3,
		},
		{
			path:  "/api/agent-demos/multiple-tools",
			runs:  1,
			tools: 2,
		},
		{
			path:  "/api/agent-demos/delivery",
			runs:  1,
			tools: 2,
		},
		{
			path:  "/api/agent-demos/tools",
			runs:  1,
			tools: 1,
		},
		{
			path:  "/api/agent-demos/observation",
			runs:  1,
			tools: 1,
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var calls atomic.Int32
			exporter := &memoryExporter{}
			handler, observeCli := setup(t, demosModel(t, &calls), routerTestConfig{
				Limits:  agent.DefaultLimits(),
				Timeout: time.Second,
			}, exporter)
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != http.StatusOK {
				t.Fatalf("实际 HTTP %d、响应 %s，预期案例成功", out.Code, out.Body.String())
			}
			if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
				t.Fatal(err)
			}
			exporter.mu.Lock()
			defer exporter.mu.Unlock()
			assertToolDemoSpans(t, exporter.spans, toolDemoSpanCounts{
				runs:   tc.runs,
				models: int(calls.Load()),
				tools:  tc.tools,
			})
		})
	}
}

// toolDemoSpanCounts 是一次工具案例应产生的各类执行节点数。
type toolDemoSpanCounts struct {
	runs   int // 独立业务运行数。
	models int // 实际模型请求数。
	tools  int // 实际工具执行数。
}

// assertToolDemoSpans 检查 HTTP、运行、模型与工具节点的数量和父子关系。
func assertToolDemoSpans(t *testing.T, spans []sdktrace.ReadOnlySpan, want toolDemoSpanCounts) {
	t.Helper()
	runs := make(map[trace.SpanID]bool)
	var requestID trace.SpanID
	toolCount := 0
	for _, span := range spans {
		if span.Name() == "agent.run" {
			runs[span.SpanContext().SpanID()] = true
		}
		if span.SpanKind() == trace.SpanKindServer {
			requestID = span.SpanContext().SpanID()
		}
		for _, attr := range span.Attributes() {
			if attr.Key == "gen_ai.operation.name" && attr.Value.AsString() == "execute_tool" {
				toolCount++
			}
		}
	}
	if len(runs) != want.runs || !requestID.IsValid() || toolCount != want.tools {
		t.Fatalf("实际运行 %d、工具 %d，预期运行 %d、工具 %d 且有 HTTP 父链路", len(runs), toolCount, want.runs, want.tools)
	}
	if count := 1 + want.runs + want.models + want.tools; len(spans) != count {
		t.Fatalf("实际 span %d 个，预期包含 HTTP、运行、模型和工具的 %d 个 span", len(spans), count)
	}
	for _, span := range spans {
		if span.Name() == "agent.run" && span.Parent().SpanID() != requestID {
			t.Error("独立运行未挂在 HTTP 请求下")
		}
		if span.SpanKind() != trace.SpanKindServer && span.Name() != "agent.run" && !runs[span.Parent().SpanID()] {
			t.Errorf("执行步骤 %s 未挂在实际运行下", span.Name())
		}
	}
}
