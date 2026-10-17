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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestHTTPRspEnvelopeAndTraceBeforeDecode(t *testing.T) {
	exporter := &memoryExporter{}
	handler, telemetry := setup(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]string{
			"role":    "assistant",
			"content": "回答",
		}, "stop")
	}, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, exporter)
	handler.GET("/bad-json", func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusOK, make(chan int), nil)
	})
	for _, tc := range []struct {
		name, method, path, body, code string // 用例名称、HTTP 方法、路径、请求体及预期响应码。
		status                         int    // 预期 HTTP 状态码。
		hasData                        bool   // 预期响应是否包含业务数据。
	}{
		{
			name:    "成功",
			method:  "POST",
			path:    "/api/agent/chat",
			body:    `{}`,
			code:    "ok",
			status:  200,
			hasData: true,
		},
		{
			name:   "解码前拒绝",
			method: "POST",
			path:   "/api/agent/chat",
			body:   `{"unknown":1}`,
			code:   "invalid_input",
			status: 400,
		},
		{
			name:   "空问题",
			method: "POST",
			path:   "/api/agent/chat",
			body:   `{"question":""}`,
			code:   "invalid_input",
			status: 400,
		},
		{
			name:   "领域拒绝空白",
			method: "POST",
			path:   "/api/agent/chat",
			body:   `{"question":" "}`,
			code:   "invalid_input",
			status: 400,
		},
		{
			name:   "领域拒绝空数组",
			method: "POST",
			path:   "/api/agent/messages",
			body:   `{"questions":[]}`,
			code:   "invalid_input",
			status: 400,
		},
		{
			name:   "工具输入拒绝",
			method: "POST",
			path:   "/api/agent/run",
			body:   `{"question":" "}`,
			code:   "invalid_input",
			status: 400,
		},
		{
			name:   "路由错误",
			method: "GET",
			path:   "/missing",
			code:   "not_found",
			status: 404,
		},
		{
			name:   "方法错误",
			method: "GET",
			path:   "/api/agent/chat",
			code:   "method_not_allowed",
			status: 405,
		},
		{
			name:   "编码失败",
			method: "GET",
			path:   "/bad-json",
			code:   "internal_error",
			status: 500,
		},
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
				if span.SpanKind() == trace.SpanKindServer && span.SpanContext().TraceID().String() == rsp.TraceID {
					found = true
					wantName := "[unmatched]"
					if tc.status != 404 && tc.status != 405 {
						wantName = tc.path
					}
					if span.Name() != wantName {
						t.Errorf("实际入口名称 %q，预期 %q", span.Name(), wantName)
					}
					if tc.status >= 400 && span.Status().Code != codes.Error {
						t.Error("HTTP 失败未标记到请求 span")
					}
					checkHTTPPreview(t, span, tc.body, out)
				}
			}
			if !found {
				t.Error("响应 trace ID 未对应到实际 HTTP span")
			}
		})
	}
}

// checkHTTPPreview 核对根 span 预览指向本次 HTTP 请求和最终写出的外层响应。
func checkHTTPPreview(t *testing.T, span sdktrace.ReadOnlySpan, body string, out *httptest.ResponseRecorder) {
	t.Helper()
	attrs := make(map[string]attribute.Value)
	for _, attr := range span.Attributes() {
		attrs[string(attr.Key)] = attr.Value
	}
	if request := attrs["app.request"].AsString(); request != body {
		t.Errorf("实际请求预览 %q，预期只有真实正文 %q", request, body)
	}
	// 脱敏会重新编码并排序对象字段，按保留数值精度后的内容比较。
	var expected any
	d := json.NewDecoder(strings.NewReader(out.Body.String()))
	d.UseNumber()
	if err := d.Decode(&expected); err != nil {
		t.Fatal(err)
	}
	wantResponse, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if response := attrs["app.response"].AsString(); response != string(wantResponse) {
		t.Errorf("实际响应预览 %s，预期 HTTP %d、正文 %s", attrs["app.response"].AsString(), out.Code, out.Body.String())
	}
	for _, tc := range []struct {
		key, payload string // 需要对照的语义属性名与正文属性名。
	}{
		{
			key:     "gen_ai.input.messages",
			payload: "app.request",
		},
		{
			key:     "gen_ai.output.messages",
			payload: "app.response",
		},
	} {
		if attrs[tc.key].AsString() != attrs[tc.payload].AsString() {
			t.Errorf("实际预览字段 %s=%s，预期直接展示 %s 的正文，不添加角色", tc.key, attrs[tc.key].AsString(), tc.payload)
		}
	}
}

func TestHTTPPreviewContentControlAndRedaction(t *testing.T) {
	for _, capture := range []bool{
		true,
		false,
	} {
		t.Run(map[bool]string{
			true:  "采集正文",
			false: "关闭正文",
		}[capture], func(t *testing.T) {
			exporter := &memoryExporter{}
			telemetry, err := openobserve.New(t.Context(), openobserve.Config{
				Enabled:        true,
				ServiceName:    "test",
				MaxChars:       256,
				CaptureContent: capture,
				Secrets:        []string{"configured-secret"},
			}, exporter, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := telemetry.Close(ctx); err != nil {
					t.Error(err)
				}
			})
			handler := gin.New()
			handler.Use(traceHTTP(telemetry), previewHTTP(telemetry))
			const request = `{"password":"input-secret","id":9007199254740993}`
			response := `{"secret":"output-secret","text":"configured-secret ` + strings.Repeat("中", 2000) + `"}`
			handler.POST("/preview", func(c *gin.Context) {
				raw, err := io.ReadAll(c.Request.Body)
				if err != nil || string(raw) != request {
					t.Fatalf("实际收到 %q、错误 %v，预期原始请求", raw, err)
				}
				c.Data(http.StatusOK, "application/json", []byte(response))
			})
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/preview", strings.NewReader(request)))
			if out.Body.String() != response {
				t.Fatal("正文预览改变了实际 HTTP 响应")
			}
			if err := telemetry.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
			exporter.mu.Lock()
			defer exporter.mu.Unlock()
			if len(exporter.spans) != 1 {
				t.Fatalf("实际 span 数 %d，预期 1", len(exporter.spans))
			}
			attrs := make(map[string]attribute.Value)
			for _, attr := range exporter.spans[0].Attributes() {
				attrs[string(attr.Key)] = attr.Value
			}
			if !capture {
				for key := range attrs {
					if strings.HasPrefix(key, "app.request") || strings.HasPrefix(key, "app.response") || strings.HasPrefix(key, "gen_ai.") {
						t.Errorf("关闭正文时仍采集 %s", key)
					}
				}
				return
			}
			raw, err := json.Marshal(exporter.spans[0].Attributes())
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{
				"input-secret",
				"output-secret",
				"configured-secret",
			} {
				if strings.Contains(string(raw), secret) {
					t.Errorf("HTTP 预览泄漏 %q", secret)
				}
			}
			if !strings.Contains(attrs["app.request"].AsString(), "9007199254740993") || !attrs["app.response.truncated"].AsBool() || !strings.HasSuffix(attrs["app.response"].AsString(), "…[TRUNCATED]") {
				t.Error("HTTP 预览未保留数值精度或缺少截断标记")
			}
		})
	}
}

func TestHTTPRspEncodingFallbackAndDisabledTrace(t *testing.T) {
	telemetry, err := openobserve.New(context.Background(), openobserve.Config{
		ServiceName: "test",
		MaxChars:    256,
	}, nil, io.Discard)
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
	}, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: time.Second,
	}, exporter)
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
	for _, secret := range []string{
		"host-secret",
		"path-secret",
		"query-secret",
		"agent-secret",
		"forwarded-secret",
		"header-secret",
	} {
		if strings.Contains(string(metadata), secret) {
			t.Errorf("HTTP span 泄漏请求值 %q：%s", secret, metadata)
		}
	}
	if span.Name() != "/metadata/:id" || !strings.Contains(string(metadata), "/metadata/:id") {
		t.Errorf("实际 span 名称 %q、属性 %s，预期只使用路由模板", span.Name(), metadata)
	}
}
