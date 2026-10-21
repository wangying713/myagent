package openobserve

import (
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ContentLimit 返回启用正文采集时的字符上限，关闭正文采集时为零。
func (c *Client) ContentLimit() int {
	if !c.config.CaptureContent {
		return 0
	}
	return c.config.MaxChars
}

// TracerProvider 供入口中间件显式使用私有 provider；关闭观测时返回 noop。
func (c *Client) TracerProvider() trace.TracerProvider {
	if c.provider == nil {
		return noop.NewTracerProvider()
	}
	return c.provider
}
