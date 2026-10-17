// Package openobserve 提供私有 Traces provider 和展示正文的脱敏，不覆盖进程全局配置。
package openobserve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config 保存私有观测设置及展示正文的脱敏规则，由启动入口显式传入。
type Config struct {
	Enabled        bool
	ServiceName    string
	Endpoint       string
	Headers        map[string]string
	CaptureContent bool // 只控制 span 正文采集，执行记录仍可生成脱敏展示副本。
	MaxChars       int  // 限制展示正文的 Unicode 字符数，额外的截断标记不计入该上限。
	Secrets        []string
}

// Telemetry 管理私有 TracerProvider 和正文脱敏，可供并发请求共享。
// 调用方应在业务请求结束后调用 Close，完成有界刷新并释放观测资源。
type Telemetry struct {
	config   Config
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	secrets  []string
	client   *http.Client
}

// reportingExporter 隔离观测失败；只输出固定提示，不输出 exporter 的底层正文。
type reportingExporter struct {
	inner    sdktrace.SpanExporter
	warnings io.Writer
	warnOnce sync.Once
	failed   atomic.Int64
}

func (e *reportingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := e.inner.ExportSpans(ctx, spans); err != nil {
		e.failed.Add(int64(len(spans)))
		e.warnOnce.Do(func() {
			fmt.Fprintln(e.warnings, "[观测] OpenObserve 上报失败，模型结果不受影响；请按 trace ID 核查服务配置")
		})
	}
	return nil
}

func (e *reportingExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}

// New 校验服务名和正文上限，创建私有观测组件；关闭观测时仍提供正文脱敏。
// 启用观测且 exporter 为 nil 时使用标准 OTLP/HTTP，也可注入内存实现供离线测试。
// 启用观测时接管导出器并在 Telemetry.Close 中关闭；warnings 为 nil 时不输出失败提示。
func New(ctx context.Context, cfg Config, exporter sdktrace.SpanExporter, warnings io.Writer) (*Telemetry, error) {
	if cfg.MaxChars < 256 || cfg.ServiceName == "" {
		return nil, fmt.Errorf("观测服务名不能为空，正文上限至少为 256 字符")
	}
	if warnings == nil {
		warnings = io.Discard
	}
	t := &Telemetry{
		config: cfg,
		tracer: noop.NewTracerProvider().Tracer("go-demos"),
	}
	t.secrets = append(t.secrets, cfg.Secrets...)
	for _, value := range cfg.Headers {
		t.secrets = append(t.secrets, value)
		if _, token, ok := strings.Cut(value, " "); ok {
			t.secrets = append(t.secrets, token)
		}
	}
	sort.Slice(t.secrets, func(i, j int) bool {
		return len(t.secrets[i]) > len(t.secrets[j])
	})
	if !cfg.Enabled {
		return t, nil
	}
	if exporter == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		t.client = &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		var err error
		exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint), otlptracehttp.WithHeaders(cfg.Headers), otlptracehttp.WithHTTPClient(t.client), otlptracehttp.WithTimeout(3*time.Second), otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}))
		if err != nil {
			t.client.CloseIdleConnections()
			return nil, fmt.Errorf("创建 OTLP 导出器失败，请检查观测配置")
		}
	}
	t.provider = sdktrace.NewTracerProvider(sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", cfg.ServiceName))), sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithBatcher(&reportingExporter{
		inner:    exporter,
		warnings: warnings,
	}, sdktrace.WithBatchTimeout(time.Second), sdktrace.WithMaxQueueSize(256), sdktrace.WithMaxExportBatchSize(32)))
	t.tracer = t.provider.Tracer("go-demos")
	return t, nil
}

// Start 沿用 ctx 的父链路创建 span；调用方负责结束返回的 span。
func (t *Telemetry) Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartClient 创建客户端 span；调用方负责结束返回的 span。
func (t *Telemetry) StartClient(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
}

// Enabled 报告配置是否启用观测，不表示导出器可达或数据已入库。
func (t *Telemetry) Enabled() bool {
	return t.config.Enabled
}

// Flush 在 ctx 允许的时间内刷新队列，保留 provider 供后续请求使用。
// 上报失败通过 warnings 提示，返回 nil 不能证明观测数据已入库。
func (t *Telemetry) Flush(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	return t.provider.ForceFlush(ctx)
}

// Close 在 ctx 允许的时间内关闭 provider 及导出器，并释放空闲 HTTP 连接。
func (t *Telemetry) Close(ctx context.Context) error {
	if t.client != nil {
		defer t.client.CloseIdleConnections()
	}
	if t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// End 记录耗时和固定错误状态后结束 span，错误正文不会写入属性。
func End(span trace.Span, started time.Time, err error) {
	span.SetAttributes(attribute.Float64("app.elapsed_ms", float64(time.Since(started).Microseconds())/1000))
	if err != nil {
		span.SetStatus(codes.Error, "步骤未完成")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
	}
	span.End()
}

var sensitiveKey = regexp.MustCompile(`(?i)authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token`)
var authValue = regexp.MustCompile(`(?i)\b(?:Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+|\bsk-[A-Za-z0-9_-]{8,}`)
var jsonSecret = regexp.MustCompile(`(?i)("(?:authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token)"\s*:\s*)("(?:\\.|[^"\\])*"|[^,}\]\s]+)`)

// Clean 遮盖已配置密钥及常见鉴权文本，返回展示副本，不限制正文长度。
func (t *Telemetry) Clean(text string) string {
	for _, secret := range t.secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	return jsonSecret.ReplaceAllString(authValue.ReplaceAllString(text, "[REDACTED]"), `${1}"[REDACTED]"`)
}

func (t *Telemetry) clean(value any, depth int) any {
	if depth >= 32 {
		return "[REDACTED]"
	}
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if sensitiveKey.MatchString(key) {
				v[key] = "[REDACTED]"
			} else {
				v[key] = t.clean(child, depth+1)
			}
		}
	case []any:
		for i, child := range v {
			v[i] = t.clean(child, depth+1)
		}
	case string:
		var nested any
		d := json.NewDecoder(strings.NewReader(v))
		d.UseNumber()
		if d.Decode(&nested) == nil && d.Decode(new(any)) == io.EOF {
			encoded, err := json.Marshal(t.clean(nested, depth+1))
			if err == nil {
				return string(encoded)
			}
		}
		return t.Clean(v)
	}
	return value
}

// Safe 返回脱敏、限长的展示副本，不修改实际请求、响应或回填消息。
func (t *Telemetry) Safe(raw []byte) string {
	var value any
	text := string(raw)
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	if d.Decode(&value) == nil && d.Decode(new(any)) == io.EOF {
		if encoded, err := json.Marshal(t.clean(value, 0)); err == nil {
			text = string(encoded)
		}
	}
	text = t.Clean(text)
	runes := []rune(text)
	if len(runes) > t.config.MaxChars {
		return string(runes[:t.config.MaxChars]) + "…[TRUNCATED]"
	}
	return text
}

// Payload 在启用正文采集时记录脱敏、限长的正文和截断标记，不修改 raw。
func (t *Telemetry) Payload(span trace.Span, key string, raw []byte) {
	if !t.config.CaptureContent {
		return
	}
	text := t.Safe(raw)
	span.SetAttributes(attribute.String(key, text), attribute.Bool(key+".truncated", strings.HasSuffix(text, "…[TRUNCATED]")))
	if key == "app.request" || key == "app.response" {
		previewKey := "gen_ai.input.messages"
		if key == "app.response" {
			previewKey = "gen_ai.output.messages"
		}
		// 直接交给 OpenObserve 的 JSON 预览，不添加对话角色或消息包装。
		span.SetAttributes(attribute.String(previewKey, text))
	}
}
