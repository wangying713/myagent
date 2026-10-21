// Package openobserve 提供私有 Traces provider、运行观测和展示正文脱敏，不覆盖进程全局配置。
package openobserve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Client 管理私有 TracerProvider 和正文脱敏，可供并发请求共享。
// 应用退出且请求结束后调用 Close，完成有界刷新并释放观测资源。
type Client struct {
	Tracer         trace.Tracer             // 业务与模型共用的追踪器，由 New 初始化，构造后只读。
	provider       *sdktrace.TracerProvider // 私有追踪提供方，关闭观测时为 nil。
	captureContent bool                     // 是否允许采集正在记录的 span 正文。
	maxChars       int                      // 展示正文的最大 Unicode 字符数，不含截断标记。
	secrets        []string                 // 已去重的脱敏词，优先匹配较长内容。
	httpCli        *http.Client             // 内部创建的 OTLP HTTP 客户端；未创建时为 nil。
}

// New 校验配置并创建私有观测客户端；关闭观测时仍提供正文脱敏。
// 启用观测且 exporter 为 nil 时创建 OTLP/HTTP 导出器；否则接管传入的导出器。
// warnings 为 nil 时不输出上报失败提示；调用方负责通过 Close 释放资源。
func New(ctx context.Context, cfg Config, exporter sdktrace.SpanExporter, warnings io.Writer) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	observeCli := &Client{
		captureContent: cfg.CaptureContent,
		maxChars:       cfg.MaxChars,
		secrets:        collectSecrets(cfg),
	}
	if cfg.Enabled {
		var err error
		observeCli.provider, err = observeCli.newProvider(ctx, cfg, exporter, warnings)
		if err != nil {
			return nil, err
		}
	}
	observeCli.Tracer = observeCli.TracerProvider().Tracer(cfg.TracerName)
	return observeCli, nil
}

// Enabled 报告配置是否启用观测，不表示导出器可达或数据已入库。
func (c *Client) Enabled() bool {
	return c.provider != nil
}

// Flush 在 ctx 允许的时间内刷新队列，保留 provider 供后续请求使用。
// 上报失败通过 warnings 提示，返回 nil 不能证明观测数据已入库。
func (c *Client) Flush(ctx context.Context) error {
	if c.provider == nil {
		return nil
	}
	return c.provider.ForceFlush(ctx)
}

// Close 在 ctx 允许的时间内关闭 provider 及导出器，并释放空闲 HTTP 连接。
func (c *Client) Close(ctx context.Context) error {
	if c.httpCli != nil {
		defer c.httpCli.CloseIdleConnections()
	}
	if c.provider == nil {
		return nil
	}
	return c.provider.Shutdown(ctx)
}

// ContentLimit 返回启用正文采集时的字符上限，关闭观测或正文采集时为零。
func (c *Client) ContentLimit() int {
	if !c.Enabled() || !c.captureContent {
		return 0
	}
	return c.maxChars
}

// TracerProvider 供入口中间件显式使用私有 provider；关闭观测时返回 noop。
func (c *Client) TracerProvider() trace.TracerProvider {
	if c.provider == nil {
		return noop.NewTracerProvider()
	}
	return c.provider
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
