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

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

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
		if err := observeCli.Flush(t.Context()); err != nil {
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
	if err := observeCli.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
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
	if err := observeCli.Flush(t.Context()); err != nil {
		t.Fatalf("上报失败影响了刷新结果：%v", err)
	}
	if requests.Load() != 1 {
		t.Errorf("实际上报次数 %d，预期只尝试 1 次", requests.Load())
	}
	if !strings.Contains(warnings.String(), "[观测]") || strings.Contains(warnings.String(), "configured-secret") {
		t.Errorf("实际上报提示 %q，预期固定提示且不含上报响应", warnings.String())
	}
}
