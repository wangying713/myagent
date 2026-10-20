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

	"go.opentelemetry.io/otel/trace"
)

func TestMessagesRedactsOutputWithoutChangingHistory(t *testing.T) {
	var calls atomic.Int32
	exporter := &memoryExporter{}
	handler, telemetry := setup(t, func(w http.ResponseWriter, r *http.Request) {
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
	if err := telemetry.Flush(t.Context()); err != nil {
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

func TestCompositeDemosKeepIndividualRunSpans(t *testing.T) {
	for _, tc := range []struct {
		path string // 包含多次业务执行的案例地址。
		runs int    // 应建立的独立运行 span 数量。
	}{
		{
			path: "/api/agent-demos/overview",
			runs: 2,
		},
		{
			path: "/api/agent-demos/evaluation",
			runs: 3,
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var calls atomic.Int32
			exporter := &memoryExporter{}
			handler, telemetry := setup(t, demosModel(t, &calls), routerTestConfig{
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
			if err := telemetry.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
			exporter.mu.Lock()
			defer exporter.mu.Unlock()
			runs := make(map[trace.SpanID]bool)
			var requestID trace.SpanID
			for _, span := range exporter.spans {
				if span.Name() == "agent.run" {
					runs[span.SpanContext().SpanID()] = true
				}
				if span.SpanKind() == trace.SpanKindServer {
					requestID = span.SpanContext().SpanID()
				}
			}
			if len(runs) != tc.runs || !requestID.IsValid() {
				t.Fatalf("实际运行 %d 个，预期 %d 个且有 HTTP 父链路", len(runs), tc.runs)
			}
			for _, span := range exporter.spans {
				if span.Name() == "agent.run" && span.Parent().SpanID() != requestID {
					t.Error("独立运行未挂在 HTTP 请求下")
				}
				if span.SpanKind() != trace.SpanKindServer && span.Name() != "agent.run" && !runs[span.Parent().SpanID()] {
					t.Errorf("模型或工具 %s 未挂在实际运行下", span.Name())
				}
			}
		})
	}
}
