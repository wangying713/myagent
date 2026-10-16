package openobserve

import (
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerProvider 供入口中间件显式使用私有 provider；关闭观测时返回 noop。
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	if t.provider == nil {
		return noop.NewTracerProvider()
	}
	return t.provider
}
