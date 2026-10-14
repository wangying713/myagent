package openobserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type retainedExporter struct{ *tracetest.InMemoryExporter }

func (*retainedExporter) Shutdown(context.Context) error { return nil }

func TestPayloadRedactsStringEncodedJSON(t *testing.T) {
	observer, err := New(context.Background(), Config{ServiceName: "测试", CaptureContent: true, MaxChars: 32768}, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	_, span := observer.StartCheck(context.Background())
	defer span.End()
	encoded := `{"\u0061pi_key":"private-value","count":9007199254740993}`
	for range 3 {
		body, err := json.Marshal(encoded)
		if err != nil {
			t.Fatal(err)
		}
		encoded = string(body)
	}
	for _, arguments := range []string{
		`{"\u0061pi_key":"private-value","count":9007199254740993}`,
		`[{"pass\u0077ord":{"value":"private-value"}}]`,
		encoded,
	} {
		t.Run(arguments, func(t *testing.T) {
			raw, err := json.Marshal(map[string]string{"arguments": arguments})
			if err != nil {
				t.Fatal(err)
			}
			original := string(raw)
			cleaned := observer.Payload(span, "app.request", raw)
			var outer map[string]string
			if err := json.Unmarshal([]byte(cleaned), &outer); err != nil {
				t.Fatalf("脱敏后应保留字符串参数类型：%v", err)
			}
			inner := outer["arguments"]
			if strings.Contains(inner, "private-value") || !strings.Contains(inner, "[REDACTED]") {
				t.Errorf("嵌套 JSON 未脱敏：%s", inner)
			}
			if strings.Contains(arguments, "9007199254740993") && !strings.Contains(inner, "9007199254740993") {
				t.Error("脱敏改变了大整数精度")
			}
			if string(raw) != original {
				t.Error("脱敏修改了实际业务正文")
			}
		})
	}
	plain := `{"arguments":"  { \"count\": 9007199254740993 }  ","text":"普通中文"}`
	if got := observer.Payload(span, "app.request", []byte(plain)); got != plain {
		t.Errorf("不含敏感信息的 JSON 字符串不应改写，实际 %s", got)
	}
}

func TestPayloadRedactsOverlyDeepJSON(t *testing.T) {
	observer, err := New(context.Background(), Config{ServiceName: "测试", CaptureContent: true, MaxChars: 32768}, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	_, span := observer.StartCheck(context.Background())
	defer span.End()
	inner := `{"\u0061pi_key":"deep-private-value"}`
	for range 80 {
		inner = `{"nested":` + inner + `}`
	}
	data, err := json.Marshal(map[string]string{"arguments": inner})
	if err != nil {
		t.Fatal(err)
	}
	cleaned := observer.Payload(span, "app.request", data)
	if strings.Contains(cleaned, "deep-private-value") || !strings.Contains(cleaned, "[REDACTED]") || !json.Valid([]byte(cleaned)) {
		t.Errorf("超深正文应保守脱敏并保持 JSON 有效，实际 %s", cleaned)
	}
}

func TestPayloadRedactionAndUnicodeLimit(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	observer, err := New(context.Background(), Config{Enabled: true, ServiceName: "测试", CaptureContent: true, MaxChars: 256,
		Secrets: []string{"known-secret"}, Headers: map[string]string{"Authorization": "Basic encoded-secret", "stream-name": "default"}}, &retainedExporter{exporter}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	_, span := observer.StartCheck(context.Background())
	data := []byte(`{"api_key":"hidden","nested":{"password":"another"},"arguments":"{\"secret\":\"nested-secret\"}","text":"known-secret Basic encoded-secret default"}`)
	text := observer.Payload(span, "app.request", data)
	for _, secret := range []string{"hidden", "another", "nested-secret", "known-secret", "encoded-secret"} {
		if strings.Contains(text, secret) {
			t.Errorf("正文保留了应脱敏的值 %q", secret)
		}
	}
	if !strings.Contains(text, "default") {
		t.Error("普通 stream 名不应被当成秘密去替换正文")
	}
	long, err := json.Marshal(strings.Repeat("文", 300))
	if err != nil {
		t.Fatal(err)
	}
	truncated := observer.Payload(span, "app.response", long)
	if !strings.HasSuffix(truncated, "…[TRUNCATED]") || strings.Count(truncated, "文") != 255 {
		t.Errorf("Unicode 限长实际保留 %d 个中文字，预期 255（加开头引号）", strings.Count(truncated, "文"))
	}
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := observer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := observer.Close(ctx); err != nil {
		t.Errorf("重复关闭应该无副作用：%v", err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span 数实际 %d，预期 1", len(spans))
	}
	found := false
	for _, attr := range spans[0].Attributes {
		if attr.Key == "app.response.truncated" {
			found = attr.Value.AsBool()
		}
	}
	if !found {
		t.Error("缺少正文截断标记")
	}
}

func TestOTLPFailureIsIsolatedAndDoesNotRetry(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/default/v1/traces" || r.Header.Get("Content-Type") != "application/x-protobuf" || r.Header.Get("Authorization") != "Basic fake" {
			t.Errorf("OTLP 请求的路径、格式或鉴权不正确")
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	var warnings bytes.Buffer
	observer, err := New(context.Background(), Config{Enabled: true, ServiceName: "测试", MaxChars: 256, Endpoint: server.URL + "/api/default/v1/traces", Headers: map[string]string{"Authorization": "Basic fake"}}, nil, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	ctx, run := observer.StartRun(context.Background(), "chat")
	_, model := observer.StartModel(ctx, "假模型")
	model.End()
	run.End(nil)
	flushCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := observer.Close(flushCtx); err != nil {
		t.Errorf("导出失败不应覆盖业务成功：%v", err)
	}
	if requests.Load() != 1 || observer.FailedSpans() != 2 || strings.Count(warnings.String(), "上报失败") != 1 {
		t.Errorf("实际请求=%d，失败 span=%d，提示=%q", requests.Load(), observer.FailedSpans(), warnings.String())
	}
	if strings.Contains(warnings.String(), "Basic fake") {
		t.Error("上报提示泄漏鉴权")
	}
}

func TestTraceQueryAndActualRelationship(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/default/_search" || r.URL.Query().Get("type") != "traces" {
			t.Error("查询没有指向 traces 搜索端点")
		}
		var body struct {
			Query struct {
				SQL string `json:"sql"`
			} `json:"query"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || !strings.Contains(body.Query.SQL, id) {
			t.Error("查询没有筛选指定 trace ID")
		}
		fmt.Fprintf(w, `{"hits":[{"trace_id":"%s","span_id":"1111111111111111","reference_parent_span_id":"0000000000000000","operation_name":"observability.check"}]}`, id)
	}))
	defer server.Close()
	cfg := Config{Endpoint: server.URL + "/api/default/v1/traces"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	hits, err := WaitTrace(ctx, cfg, id, 1, server.Client())
	if err != nil || len(hits) != 1 {
		t.Fatalf("查询实际 %d/%v，预期查回一个 span", len(hits), err)
	}
	hits = append(hits, map[string]any{"trace_id": id, "span_id": "2222222222222222", "reference_parent_span_id": "1111111111111111"})
	if err := ValidateTrace(hits, id); err != nil {
		t.Fatal(err)
	}
	hits[1]["reference_parent_span_id"] = "3333333333333333"
	if err := ValidateTrace(hits, id); err == nil {
		t.Error("孤立 span 被当成完整轨迹")
	}
	if _, err := QueryTrace(ctx, cfg, "' OR true", server.Client()); err == nil {
		t.Error("无效 trace ID 未被拒绝")
	}
	cfg.Headers = map[string]string{"stream-name": `unsafe"stream`}
	if _, err := QueryTrace(ctx, cfg, id, server.Client()); err == nil {
		t.Error("无效 stream 未被拒绝")
	}
}

func TestTraceQueryHTTPFailureAndEmptyResults(t *testing.T) {
	for _, status := range []int{401, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); fmt.Fprint(w, `{"hits":[]}`) }))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if _, err := WaitTrace(ctx, Config{Endpoint: server.URL + "/api/default/v1/traces"}, "0123456789abcdef0123456789abcdef", 1, server.Client()); err == nil {
				t.Error("服务鉴权失败或未查回记录时不应验收成功")
			}
		})
	}
}
