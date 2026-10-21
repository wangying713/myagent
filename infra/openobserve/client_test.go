package openobserve

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func newTestClient(t *testing.T, capture bool, maxChars int) (*Client, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	observeCli, err := New(t.Context(), Config{
		Enabled:        true,
		ServiceName:    "test",
		TracerName:     "test",
		CaptureContent: capture,
		MaxChars:       maxChars,
		Secrets:        []string{"configured-secret"},
	}, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	registerClientCleanup(t, observeCli)
	return observeCli, exporter
}

// registerClientCleanup 在用例结束时有界关闭观测资源。
func registerClientCleanup(t *testing.T, observeCli *Client) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := observeCli.Close(ctx); err != nil {
			t.Error(err)
		}
	})
}

func TestTracerConfigurationAndContext(t *testing.T) {
	for _, enabled := range []bool{
		true,
		false,
	} {
		t.Run(map[bool]string{
			true:  "启用追踪",
			false: "关闭追踪",
		}[enabled], func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			observeCli, err := New(t.Context(), Config{
				Enabled:     enabled,
				ServiceName: "test-service",
				TracerName:  "custom-tracer",
				MaxChars:    256,
			}, exporter, nil)
			if err != nil {
				t.Fatal(err)
			}
			registerClientCleanup(t, observeCli)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx, parent := observeCli.Tracer.Start(ctx, "parent")
			childCtx, child := observeCli.Tracer.Start(ctx, "child", trace.WithSpanKind(trace.SpanKindClient))
			if child.IsRecording() != enabled {
				t.Errorf("实际采集状态 %v，预期 %v", child.IsRecording(), enabled)
			}
			cancel()
			if !errors.Is(childCtx.Err(), context.Canceled) {
				t.Errorf("实际子上下文错误 %v，预期 context.Canceled", childCtx.Err())
			}
			child.End()
			parent.End()
			if observeCli.Provider != nil {
				if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			spans := exporter.GetSpans()
			if !enabled {
				if len(spans) != 0 {
					t.Fatalf("关闭追踪实际导出 %d 个 span，预期 0", len(spans))
				}
				return
			}
			if len(spans) != 2 {
				t.Fatalf("实际导出 %d 个 span，预期 2", len(spans))
			}
			for _, span := range spans {
				if span.InstrumentationScope.Name != "custom-tracer" {
					t.Errorf("实际追踪器名称 %q，预期 custom-tracer", span.InstrumentationScope.Name)
				}
				if span.Name == "child" && span.Parent.SpanID() != parent.SpanContext().SpanID() {
					t.Error("客户端 span 丢失父链路")
				}
			}
		})
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	for _, cfg := range []Config{
		{
			TracerName: "test",
			MaxChars:   256,
		},
		{
			ServiceName: "test",
			MaxChars:    256,
		},
		{
			ServiceName: "test",
			TracerName:  "test",
			MaxChars:    255,
		},
	} {
		observeCli, err := New(t.Context(), cfg, nil, nil)
		if observeCli != nil || err == nil {
			t.Errorf("实际客户端 %v、错误 %v，预期配置无效时返回 nil 和错误", observeCli, err)
		}
	}
}

func TestObservationStateAndContentLimit(t *testing.T) {
	for _, tc := range []struct {
		name    string // 用例名称。
		enabled bool   // 是否启用观测资源。
		capture bool   // 是否允许正文采集。
		limit   int    // 对外提供的正文字符采集上限。
	}{
		{
			name:    "观测和正文采集均启用",
			enabled: true,
			capture: true,
			limit:   256,
		},
		{
			name:    "只关闭正文采集",
			enabled: true,
		},
		{
			name:    "关闭观测但配置允许正文采集",
			capture: true,
		},
		{
			name: "观测和正文采集均关闭",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeCli, err := New(t.Context(), Config{
				Enabled:        tc.enabled,
				CaptureContent: tc.capture,
				ServiceName:    "test",
				TracerName:     "test",
				MaxChars:       256,
				Secrets:        []string{"configured-secret"},
			}, tracetest.NewInMemoryExporter(), nil)
			if err != nil {
				t.Fatal(err)
			}
			registerClientCleanup(t, observeCli)
			if (observeCli.Provider != nil) != tc.enabled || observeCli.ContentLimit != tc.limit {
				t.Errorf("实际观测状态 %v、采集上限 %d，预期 %v、%d", observeCli.Provider != nil, observeCli.ContentLimit, tc.enabled, tc.limit)
			}
			if got := observeCli.Safe([]byte("configured-secret")); got != "[REDACTED]" {
				t.Errorf("实际展示副本 %q，预期采集开关不影响展示脱敏", got)
			}
		})
	}
}

// failingExporter 模拟上报失败并记录生命周期调用次数。
type failingExporter struct {
	exports   atomic.Int32 // 实际收到的导出批次数。
	shutdowns atomic.Int32 // 实际收到的关闭次数。
}

// exportTransport 在内存中模拟 OTLP 的 HTTP 响应，不监听端口或访问网络。
type exportTransport func(*http.Request) (*http.Response, error)

func (transport exportTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func (e *failingExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	e.exports.Add(1)
	return errors.New("模拟底层错误 configured-secret")
}

func (e *failingExporter) Shutdown(context.Context) error {
	e.shutdowns.Add(1)
	return nil
}

func TestExporterFailureIsReportedOnceAndShutdownIsOwned(t *testing.T) {
	exporter := &failingExporter{}
	var warnings bytes.Buffer
	observeCli, err := New(t.Context(), Config{
		Enabled:     true,
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
	}, exporter, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	registerClientCleanup(t, observeCli)
	for range 2 {
		_, span := observeCli.Tracer.Start(t.Context(), "test")
		span.End()
		if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
			t.Fatalf("上报失败影响了刷新结果：%v", err)
		}
	}
	if exporter.exports.Load() != 2 {
		t.Errorf("实际导出 %d 批，预期 2 批且不重试", exporter.exports.Load())
	}
	if strings.Count(warnings.String(), "[观测]") != 1 || strings.Contains(warnings.String(), "configured-secret") {
		t.Errorf("实际上报提示 %q，预期只输出一次固定提示且不含底层错误正文", warnings.String())
	}
	if err := observeCli.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if exporter.shutdowns.Load() != 1 {
		t.Errorf("实际导出器关闭次数 %d，预期 1", exporter.shutdowns.Load())
	}
}

func TestDisabledClientDoesNotTakeExporterOwnership(t *testing.T) {
	exporter := &failingExporter{}
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
	}, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, span := observeCli.Tracer.Start(t.Context(), "test")
	span.End()
	if err := observeCli.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if exporter.exports.Load() != 0 || exporter.shutdowns.Load() != 0 {
		t.Error("关闭观测时不应使用或关闭调用方传入的导出器")
	}
}

func TestOTLPHTTPFailureDoesNotRetryOrExposeResponse(t *testing.T) {
	var requests atomic.Int32
	var warnings bytes.Buffer
	observeCli, err := New(t.Context(), Config{
		Enabled:     true,
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
		Endpoint:    "http://observe.invalid/v1/traces",
		Headers: map[string]string{
			"Authorization": "Basic test-token",
		},
	}, nil, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	registerClientCleanup(t, observeCli)
	observeCli.httpCli.Transport = exportTransport(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		if req.URL.Path != "/v1/traces" || req.Header.Get("Authorization") != "Basic test-token" {
			t.Error("OTLP 请求未沿用配置中的路径或鉴权头")
		}
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("模拟上报响应 configured-secret")),
		}, nil
	})
	_, span := observeCli.Tracer.Start(t.Context(), "test")
	span.End()
	if err := observeCli.Provider.ForceFlush(t.Context()); err != nil {
		t.Fatalf("上报失败影响了刷新结果：%v", err)
	}
	if requests.Load() != 1 {
		t.Errorf("实际上报次数 %d，预期只尝试 1 次", requests.Load())
	}
	if !strings.Contains(warnings.String(), "[观测]") || strings.Contains(warnings.String(), "configured-secret") {
		t.Errorf("实际上报提示 %q，预期固定提示且不含上报响应", warnings.String())
	}
}
