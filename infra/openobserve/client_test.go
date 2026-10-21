package openobserve

import (
	"context"
	"errors"
	"testing"
	"time"

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
			if err := observeCli.Flush(t.Context()); err != nil {
				t.Fatal(err)
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
			if observeCli.Enabled() != tc.enabled || observeCli.ContentLimit() != tc.limit {
				t.Errorf("实际观测状态 %v、采集上限 %d，预期 %v、%d", observeCli.Enabled(), observeCli.ContentLimit(), tc.enabled, tc.limit)
			}
			if got := observeCli.Safe([]byte("configured-secret")); got != "[REDACTED]" {
				t.Errorf("实际展示副本 %q，预期采集开关不影响展示脱敏", got)
			}
		})
	}
}
