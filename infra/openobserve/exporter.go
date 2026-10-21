package openobserve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	exportTimeout      = 3 * time.Second // 单次 OTLP 导出的最长等待时间。
	batchTimeout       = time.Second     // 观测批次的最长等待时间。
	maxQueueSize       = 256             // 等待上报的最大 span 数。
	maxExportBatchSize = 32              // 每批最多导出的 span 数。
)

// newProvider 组装私有追踪资源，注入的导出器由 provider 接管。
func (c *Client) newProvider(ctx context.Context, cfg Config, exporter sdktrace.SpanExporter, warnings io.Writer) (*sdktrace.TracerProvider, error) {
	if exporter == nil {
		var err error
		exporter, err = c.newHTTPExporter(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}
	if warnings == nil {
		warnings = io.Discard
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", cfg.ServiceName))),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(&reportingExporter{
			inner:    exporter,
			warnings: warnings,
		},
			sdktrace.WithBatchTimeout(batchTimeout),
			sdktrace.WithMaxQueueSize(maxQueueSize),
			sdktrace.WithMaxExportBatchSize(maxExportBatchSize),
		),
	), nil
}

// newHTTPExporter 创建不重试、不使用代理和不跟随重定向的 OTLP 导出器。
// 创建失败时关闭已创建的 HTTP 连接；成功后由 Client.Close 释放。
func (c *Client) newHTTPExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	c.httpCli = &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithHeaders(cfg.Headers),
		otlptracehttp.WithHTTPClient(c.httpCli),
		otlptracehttp.WithTimeout(exportTimeout),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled: false,
		}),
	)
	if err != nil {
		c.httpCli.CloseIdleConnections()
		return nil, fmt.Errorf("创建 OTLP 导出器失败，请检查观测配置")
	}
	return exporter, nil
}

// reportingExporter 隔离观测失败；只输出固定提示，不输出 exporter 的底层正文。
type reportingExporter struct {
	inner    sdktrace.SpanExporter // 实际接收并导出 span 的导出器。
	warnings io.Writer             // 观测失败提示的输出目标。
	warnOnce sync.Once             // 保证导出失败提示只输出一次。
}

func (e *reportingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := e.inner.ExportSpans(ctx, spans); err != nil {
		e.warnOnce.Do(func() {
			fmt.Fprintln(e.warnings, "[观测] OpenObserve 上报失败，模型结果不受影响；请按 trace ID 核查服务配置")
		})
	}
	return nil
}

func (e *reportingExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}
