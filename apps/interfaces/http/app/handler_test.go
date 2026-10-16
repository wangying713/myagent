package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
)

func TestHTTPRspEnvelopeAndTraceBeforeDecode(t *testing.T) {
	exporter := &memoryExporter{}
	handler, telemetry := setup(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]string{"role": "assistant", "content": "回答"}, "stop")
	}, agent.DefaultLimits(), time.Second, exporter)
	handler.GET("/bad-json", func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusOK, make(chan int), nil)
	})
	for _, tc := range []struct {
		name, method, path, body, code string
		status                         int
		hasData                        bool
	}{
		{name: "成功", method: "POST", path: "/api/agent/chat", body: `{}`, code: "ok", status: 200, hasData: true},
		{name: "解码前拒绝", method: "POST", path: "/api/agent/chat", body: `{"unknown":1}`, code: "invalid_input", status: 400},
		{name: "空问题", method: "POST", path: "/api/agent/chat", body: `{"question":""}`, code: "invalid_input", status: 400},
		{name: "领域拒绝空白", method: "POST", path: "/api/agent/chat", body: `{"question":" "}`, code: "invalid_input", status: 400},
		{name: "领域拒绝空数组", method: "POST", path: "/api/agent/messages", body: `{"questions":[]}`, code: "invalid_input", status: 400},
		{name: "工具输入拒绝", method: "POST", path: "/api/agent/run", body: `{"question":" "}`, code: "invalid_input", status: 400},
		{name: "路由错误", method: "GET", path: "/missing", code: "not_found", status: 404},
		{name: "方法错误", method: "GET", path: "/api/agent/chat", code: "method_not_allowed", status: 405},
		{name: "编码失败", method: "GET", path: "/bad-json", code: "internal_error", status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			var rsp entity.Rsp[json.RawMessage]
			if err := json.Unmarshal(out.Body.Bytes(), &rsp); err != nil {
				t.Fatal(err)
			}
			if out.Code != tc.status || rsp.Code != tc.code || rsp.Msg == "" || len(rsp.TraceID) != 32 {
				t.Fatalf("实际响应 %d %s，预期 HTTP %d、code=%s 且有说明与真实 trace ID", out.Code, out.Body.String(), tc.status, tc.code)
			}
			if (string(rsp.Data) != "null") != tc.hasData {
				t.Fatalf("实际 data=%s，预期有结果=%v", rsp.Data, tc.hasData)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(out.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 4 || fields["code"] == nil || fields["data"] == nil || fields["traceID"] == nil || fields["msg"] == nil {
				t.Errorf("实际外层字段 %v，预期恰好 code/data/traceID/msg", fields)
			}
			if tc.hasData {
				var data map[string]json.RawMessage
				if err := json.Unmarshal(rsp.Data, &data); err != nil {
					t.Fatal(err)
				}
				if data["trace_id"] != nil || data["traceID"] != nil {
					t.Error("业务 data 中不应重复携带 HTTP trace ID")
				}
			}
			if err := telemetry.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
			exporter.mu.Lock()
			defer exporter.mu.Unlock()
			found := false
			for _, span := range exporter.spans {
				if span.Name() == "http.request" && span.SpanContext().TraceID().String() == rsp.TraceID {
					found = true
					if tc.status >= 400 && span.Status().Code != codes.Error {
						t.Error("HTTP 失败未标记到请求 span")
					}
				}
			}
			if !found {
				t.Error("响应 trace ID 未对应到实际 HTTP span")
			}
		})
	}
}

func TestHTTPRspEncodingFallbackAndDisabledTrace(t *testing.T) {
	telemetry, err := openobserve.New(context.Background(), openobserve.Config{ServiceName: "test", MaxChars: 256}, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := telemetry.Close(t.Context()); err != nil {
			t.Error(err)
		}
	})
	handler := gin.New()
	handler.Use(traceHTTP(telemetry))
	handler.GET("/bad", func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusOK, make(chan int), nil)
	})
	out := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	handler.ServeHTTP(out, req)
	var rsp entity.Rsp[any]
	if err := json.Unmarshal(out.Body.Bytes(), &rsp); err != nil || out.Code != 500 || rsp.Code != "internal_error" || rsp.Data != nil || rsp.TraceID != "" || rsp.Msg != "响应编码失败" {
		t.Fatalf("实际回退响应 %d %s，预期同格式的 JSON 500，关闭观测时 trace ID 为空", out.Code, out.Body.String())
	}
}

func TestHTTPTraceDoesNotCaptureRawRequestMetadata(t *testing.T) {
	exporter := &memoryExporter{}
	handler, telemetry := setup(t, func(http.ResponseWriter, *http.Request) {
		t.Error("元数据检查不应调用模型")
	}, agent.DefaultLimits(), time.Second, exporter)
	handler.GET("/metadata/:id", func(c *gin.Context) {
		if c.Param("id") != "path-secret" || c.Query("token") != "query-secret" || c.Request.Header.Get("User-Agent") != "agent-secret" {
			t.Error("观测中间件修改了处理方法收到的请求")
		}
		writeHTTPRsp(c, http.StatusOK, nil, nil)
	})
	req := httptest.NewRequest(http.MethodGet, "http://host-secret/metadata/path-secret?token=query-secret", nil)
	req.Header.Set("User-Agent", "agent-secret")
	req.Header.Set("X-Forwarded-For", "forwarded-secret")
	req.Header.Set("Authorization", "Bearer header-secret")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != http.StatusOK {
		t.Fatalf("元数据检查实际 HTTP %d，预期 200", out.Code)
	}
	if err := telemetry.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	if len(exporter.spans) != 1 {
		t.Fatalf("实际 %d 个 span，预期只有 HTTP span", len(exporter.spans))
	}
	span := exporter.spans[0]
	metadata, err := json.Marshal(span.Attributes())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"host-secret", "path-secret", "query-secret", "agent-secret", "forwarded-secret", "header-secret"} {
		if strings.Contains(string(metadata), secret) {
			t.Errorf("HTTP span 泄漏请求值 %q：%s", secret, metadata)
		}
	}
	if span.Name() != "http.request" || !strings.Contains(string(metadata), "/metadata/:id") {
		t.Errorf("实际 span 名称 %q、属性 %s，预期保留固定名称与路由模板", span.Name(), metadata)
	}
}
