// Package openobserve 提供私有 Traces provider、运行观测和展示正文脱敏，不覆盖进程全局配置。
package openobserve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// MinContentMaxChars 是观测正文字符上限允许的最小值。
const MinContentMaxChars = 256

// Config 保存私有观测设置及展示正文的脱敏规则，由启动入口显式传入。
type Config struct {
	Enabled     bool              // 是否创建并上报链路观测数据。
	ServiceName string            // 链路数据中的服务名称。
	TracerName  string            // 创建业务、模型与工具 span 的追踪器名称，由应用显式提供。
	Endpoint    string            // OTLP HTTP Traces 上报地址。
	Headers     map[string]string // OTLP 上报请求头，可包含鉴权信息。
	// CaptureContent 只控制 span 正文采集，执行记录仍可生成脱敏展示副本。
	CaptureContent bool
	// MaxChars 限制展示正文的 Unicode 字符数，额外的截断标记不计入该上限。
	MaxChars int
	Secrets  []string // 展示正文时需要遮盖的额外敏感值。
}

const (
	exportTimeout      = 3 * time.Second // 单次 OTLP 导出的最长等待时间。
	batchTimeout       = time.Second     // 观测批次的最长等待时间。
	maxQueueSize       = 256             // 等待上报的最大 span 数。
	maxExportBatchSize = 32              // 每批最多导出的 span 数。
	requestPayloadKey  = "app.request"   // 输入正文的观测属性名。
	responsePayloadKey = "app.response"  // 输出正文的观测属性名。
)

// Client 管理私有 TracerProvider 和正文脱敏，可供并发请求共享。
// 应用退出且请求结束后调用 Close，完成有界刷新并释放观测资源。
type Client struct {
	Tracer       trace.Tracer             // 业务、模型与工具共用的追踪器，由 New 初始化，构造后只读。
	Provider     *sdktrace.TracerProvider // 本组件专用的追踪提供方；关闭观测时为 nil，构造后只读。
	ContentLimit int                      // span 正文采集的 Unicode 字符上限；零表示不采集，构造后只读。
	maxChars     int                      // 展示正文的最大 Unicode 字符数，不含截断标记。
	secrets      []string                 // 已去重的脱敏词，优先匹配较长内容。
	httpCli      *http.Client             // 内部创建的 OTLP HTTP 客户端；未创建时为 nil。
}

// New 校验配置并创建私有观测客户端；关闭观测时仍提供正文脱敏。
// 启用观测且 exporter 为 nil 时创建 OTLP/HTTP 导出器；否则接管传入的导出器。
// warnings 为 nil 时不输出上报失败提示；调用方负责通过 Close 释放资源。
func New(ctx context.Context, cfg Config, exporter sdktrace.SpanExporter, warnings io.Writer) (*Client, error) {
	if cfg.ServiceName == "" || cfg.TracerName == "" {
		return nil, fmt.Errorf("观测服务名和追踪器名称不能为空")
	}
	if cfg.MaxChars < MinContentMaxChars {
		return nil, fmt.Errorf("观测正文上限至少为 %d 字符", MinContentMaxChars)
	}
	observeCli := &Client{
		maxChars: cfg.MaxChars,
		secrets:  collectSecrets(cfg),
	}
	var provider trace.TracerProvider = noop.NewTracerProvider()
	if cfg.Enabled {
		if exporter == nil {
			var err error
			exporter, err = observeCli.newHTTPExporter(ctx, cfg)
			if err != nil {
				return nil, err
			}
		}
		if warnings == nil {
			warnings = io.Discard
		}
		observeCli.Provider = sdktrace.NewTracerProvider(
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
		)
		provider = observeCli.Provider
		if cfg.CaptureContent {
			observeCli.ContentLimit = cfg.MaxChars
		}
	}
	observeCli.Tracer = provider.Tracer(cfg.TracerName)
	return observeCli, nil
}

// Close 在 ctx 允许的时间内关闭 provider 及导出器，并释放空闲 HTTP 连接。
func (c *Client) Close(ctx context.Context) error {
	if c.httpCli != nil {
		defer c.httpCli.CloseIdleConnections()
	}
	if c.Provider == nil {
		return nil
	}
	return c.Provider.Shutdown(ctx)
}

// End 记录耗时和固定错误状态后结束 span，错误正文不会写入属性。
func End(span trace.Span, started time.Time, err error) {
	defer span.End()
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attribute.Float64("app.elapsed_ms", float64(time.Since(started).Microseconds())/1000))
	if err != nil {
		span.SetStatus(codes.Error, "步骤未完成")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
	}
}

// Payload 在启用正文采集时记录脱敏、限长的正文和截断标记，不修改 raw。
func (c *Client) Payload(span trace.Span, key string, raw []byte) {
	if c.ContentLimit == 0 || !span.IsRecording() {
		return
	}
	text := c.Safe(raw)
	span.SetAttributes(attribute.String(key, text), attribute.Bool(key+".truncated", strings.HasSuffix(text, truncationSuffix)))
	if key == requestPayloadKey || key == responsePayloadKey {
		previewKey := "gen_ai.input.messages"
		if key == responsePayloadKey {
			previewKey = "gen_ai.output.messages"
		}
		// 直接交给 OpenObserve 的 JSON 预览，不添加对话角色或消息包装。
		span.SetAttributes(attribute.String(previewKey, text))
	}
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
