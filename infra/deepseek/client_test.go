package deepseek_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"sakelei/ai/domain/agent/entity"
	"sakelei/ai/domain/agent/service"
	"sakelei/ai/infra/deepseek"
	"sakelei/ai/infra/openobserve"
)

type keptExporter struct{ *tracetest.InMemoryExporter }

// SDK 的内存 exporter 在关闭时清空数据；测试需要保留已完成的证据。
func (*keptExporter) Shutdown(context.Context) error { return nil }

func telemetry(t *testing.T, capture bool) (*openobserve.Telemetry, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	observer, err := openobserve.New(context.Background(), openobserve.Config{Enabled: true, CaptureContent: capture, ServiceName: "离线测试", MaxChars: 32768, Secrets: []string{"test-secret-key"}}, &keptExporter{exporter}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	return observer, exporter
}

func closeTelemetry(t *testing.T, observer *openobserve.Telemetry) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := observer.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAgentHTTPAndTrace(t *testing.T) {
	observer, exporter := telemetry(t, true)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-secret-key" {
			t.Errorf("HTTP 路径或鉴权不符合配置")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		var request entity.ChatRequest
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		if request.Model != "假模型" || request.MaxTokens != 512 || request.Thinking == nil || request.Thinking.Type != "disabled" {
			t.Errorf("默认请求字段实际 %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			fmt.Fprint(w, `{"id":"first","model":"假模型","choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"应保留的扩展字段","tool_calls":[{"id":"calc","type":"function","function":{"name":"multiply","arguments":"{\"a\":17,\"b\":23}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15},"extra":{"secret":"private"}}`)
			return
		}
		if len(request.Messages) != 4 || request.Messages[3].ToolCallID != "calc" || !strings.Contains(request.Messages[3].Content, "391") {
			t.Errorf("回填请求实际 %+v", request.Messages)
		}
		if !bytes.Contains(body, []byte(`"reasoning_content"`)) || !bytes.Contains(body, []byte(`"content":null`)) {
			t.Error("assistant 原始消息的扩展字段或 null 丢失")
		}
		fmt.Fprint(w, `{"id":"second","model":"假模型","choices":[{"message":{"role":"assistant","content":"391"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":3,"total_tokens":23}}`)
	}))
	defer server.Close()
	client, err := deepseek.New(deepseek.Config{APIKey: "test-secret-key", BaseURL: server.URL + "/v1", Model: "假模型", Timeout: time.Second}, observer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	registry, err := service.NewRegistry([]service.Tool{service.MultiplyTool()})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := service.New(client, &openobserve.Tools{Registry: registry, Telemetry: observer})
	if err != nil {
		t.Fatal(err)
	}
	ctx, run := observer.StartRun(context.Background(), "agent")
	result, err := agent.Run(ctx, "计算 17×23；密钥 test-secret-key", "用工具", service.DefaultLimits())
	run.End(err)
	if err != nil {
		t.Fatal(err)
	}
	if result.Answer != "391" || result.KnownTokens != 38 || requests.Load() != 2 {
		t.Errorf("结果实际 %+v，请求 %d，预期 391 / 38 token / 2 请求", result, requests.Load())
	}
	closeTelemetry(t, observer)
	spans := exporter.GetSpans()
	if len(spans) != 4 {
		t.Fatalf("span 数实际 %d，预期 4", len(spans))
	}
	var rootID string
	for _, span := range spans {
		if span.Name == "agent.run" {
			rootID = span.SpanContext.SpanID().String()
		}
	}
	models, tools := 0, 0
	for _, span := range spans {
		if span.SpanContext.TraceID().String() != run.TraceID {
			t.Error("同一次运行的 trace ID 不一致")
		}
		if span.Name != "agent.run" && span.Parent.SpanID().String() != rootID {
			t.Errorf("%s 父节点实际 %s，预期 %s", span.Name, span.Parent.SpanID(), rootID)
		}
		attrs := map[string]string{}
		for _, attr := range span.Attributes {
			attrs[string(attr.Key)] = attr.Value.Emit()
			if strings.Contains(attr.Value.Emit(), "test-secret-key") || strings.Contains(attr.Value.Emit(), "Bearer ") {
				t.Errorf("属性 %s 泄漏凭据", attr.Key)
			}
		}
		if strings.HasPrefix(span.Name, "chat ") {
			models++
			if attrs["app.request"] == "" || attrs["app.response"] == "" || attrs["gen_ai.input.messages"] == "" || attrs["gen_ai.output.messages"] == "" {
				t.Error("模型请求、响应或预览缺失")
			}
		}
		if span.Name == "Tool call" {
			tools++
			if attrs["app.tool.call.id"] != "calc" || !strings.Contains(attrs["gen_ai.output.messages"], "391") {
				t.Error("工具观测未关联实际调用和结果")
			}
		}
	}
	if models != 2 || tools != 1 {
		t.Errorf("模型/工具 span 实际 %d/%d，预期 2/1", models, tools)
	}
}

func TestChatErrorsKeepResponseAndDoNotRetry(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"HTTP 错误", 429, `{"error":{"message":"限流","api_key":"test-secret-key"}}`},
		{"无效 JSON", 200, `{"未完成`},
		{"缺少 choices", 200, `{}`},
		{"截断回答", 200, `{"choices":[{"message":{"role":"assistant","content":"半句"},"finish_reason":"length"}],"usage":{"total_tokens":3}}`},
		{"空回答", 200, `{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`},
		{"错误用量", 200, `{"choices":[{"message":{"role":"assistant","content":"回答"},"finish_reason":"stop"}],"usage":{"total_tokens":-1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observer, exporter := telemetry(t, true)
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := deepseek.New(deepseek.Config{APIKey: "test-secret-key", BaseURL: server.URL, Model: "假模型", Timeout: time.Second}, observer, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			_, err = client.Chat(context.Background(), entity.ChatRequest{Messages: []entity.Message{{Role: "user", Content: "问题"}}})
			if err == nil || requests.Load() != 1 {
				t.Errorf("错误实际 %v，请求 %d，预期失败且不重试", err, requests.Load())
			}
			closeTelemetry(t, observer)
			spans := exporter.GetSpans()
			if len(spans) != 1 || spans[0].Status.Code != codes.Error {
				t.Fatalf("错误 span 实际 %+v", spans)
			}
			found := false
			for _, attr := range spans[0].Attributes {
				if attr.Key == "app.response" {
					found = true
					if strings.Contains(attr.Value.AsString(), "test-secret-key") {
						t.Error("失败响应泄漏密钥")
					}
				}
			}
			if !found {
				t.Error("错误时实际响应未保留")
			}
		})
	}
}

func TestChatCancellation(t *testing.T) {
	observer, exporter := telemetry(t, false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := deepseek.New(deepseek.Config{APIKey: "test-secret-key", BaseURL: server.URL, Model: "假模型", Timeout: 30 * time.Millisecond}, observer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, err = client.Chat(context.Background(), entity.ChatRequest{Messages: []entity.Message{{Role: "user", Content: "问题"}}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("超时错误实际 %v，预期 context.DeadlineExceeded", err)
	}
	closeTelemetry(t, observer)
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("取消 span 实际 %+v", spans)
	}
	for _, attr := range spans[0].Attributes {
		if attr.Key == "app.request" || attr.Key == "app.response" || attr.Key == "gen_ai.input.messages" {
			t.Error("关闭正文采集后仍记录正文")
		}
	}
}

func TestInvalidFinishReasonIsRedactedWithoutContentCapture(t *testing.T) {
	observer, exporter := telemetry(t, false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"半句"},"finish_reason":"test-secret-key"}],"usage":{"total_tokens":37}}`)
	}))
	defer server.Close()
	client, err := deepseek.New(deepseek.Config{APIKey: "test-secret-key", BaseURL: server.URL, Model: "假模型", Timeout: time.Second}, observer, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	response, err := client.Chat(context.Background(), entity.ChatRequest{Messages: []entity.Message{{Role: "user", Content: "问题"}}})
	if err == nil || response.FinishReason != "test-secret-key" {
		t.Fatalf("业务响应应保留原始结束原因并报错，实际 %+v / %v", response, err)
	}
	closeTelemetry(t, observer)
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Status.Code != codes.Error {
		t.Fatalf("错误 span 实际 %+v", spans)
	}
	found := false
	for _, attr := range spans[0].Attributes {
		if strings.Contains(attr.Value.Emit(), "test-secret-key") {
			t.Errorf("属性 %s 泄漏密钥", attr.Key)
		}
		if attr.Key == "gen_ai.response.finish_reasons" {
			found = strings.Contains(attr.Value.Emit(), "[REDACTED]")
		}
		if attr.Key == "app.request" || attr.Key == "app.response" || attr.Key == "gen_ai.input.messages" || attr.Key == "gen_ai.output.messages" {
			t.Errorf("关闭正文采集后仍记录 %s", attr.Key)
		}
	}
	if !found {
		t.Error("缺少脱敏后的结束原因")
	}
}
