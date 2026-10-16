package openobserve

import (
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ContentLimit 返回启用正文采集时的字符上限，关闭正文采集时为零。
func (t *Telemetry) ContentLimit() int {
	if !t.config.CaptureContent {
		return 0
	}
	return t.config.MaxChars
}

// TracerProvider 供入口中间件显式使用私有 provider；关闭观测时返回 noop。
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	if t.provider == nil {
		return noop.NewTracerProvider()
	}
	return t.provider
}
