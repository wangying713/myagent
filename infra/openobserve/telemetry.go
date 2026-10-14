// Package openobserve 封装 trace 的创建、正文处理、OTLP 导出与入库检查。
package openobserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

	"sakelei/ai/domain/agent/entity"
	"sakelei/ai/domain/agent/service"
)

// Config 配置教学观测。Headers 和 Secrets 只用于鉴权或脱敏，不写入 span。
type Config struct {
	Enabled        bool
	ServiceName    string
	Endpoint       string
	Headers        map[string]string
	CaptureContent bool
	MaxChars       int
	Secrets        []string
}

// Telemetry 拥有独立 provider，不覆盖进程全局观测配置。
type Telemetry struct {
	config    Config
	tracer    trace.Tracer
	provider  *sdktrace.TracerProvider
	exporter  *reportingExporter
	warnings  io.Writer
	secrets   []string
	closeOnce sync.Once
	closeErr  error
	http      *http.Client
}

type reportingExporter struct {
	inner    sdktrace.SpanExporter
	failed   atomic.Int64
	warnOnce sync.Once
	warnings io.Writer
}

func (e *reportingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := e.inner.ExportSpans(ctx, spans); err != nil {
		e.failed.Add(int64(len(spans)))
		e.warnOnce.Do(func() {
			fmt.Fprintln(e.warnings, "[观测] OpenObserve 上报失败，业务继续；请运行 observe 检查。")
		})
		// 已计数并提示，不再让 SDK 的全局错误处理器重复打印或泄漏响应信息。
		return nil
	}
	return nil
}

func (e *reportingExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}

// New 创建 OTLP/HTTP 观测器。可注入内存 exporter，使离线测试不访问网络。
func New(ctx context.Context, config Config, exporter sdktrace.SpanExporter, warnings io.Writer) (*Telemetry, error) {
	if config.MaxChars < 256 || config.ServiceName == "" {
		return nil, errors.New("观测服务名不能为空，正文上限至少为 256 字符")
	}
	if warnings == nil {
		warnings = os.Stderr
	}
	t := &Telemetry{config: config, warnings: warnings, tracer: noop.NewTracerProvider().Tracer("go-agent-learning")}
	t.secrets = append([]string(nil), config.Secrets...)
	for key, value := range config.Headers {
		if !sensitiveKey.MatchString(key) {
			continue
		}
		t.secrets = append(t.secrets, value)
		if _, credential, ok := strings.Cut(value, " "); ok {
			t.secrets = append(t.secrets, credential)
		}
	}
	sort.Slice(t.secrets, func(i, j int) bool { return len(t.secrets[i]) > len(t.secrets[j]) })
	if !config.Enabled {
		return t, nil
	}
	if exporter == nil {
		var err error
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		t.http = client
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(config.Endpoint),
			otlptracehttp.WithHeaders(config.Headers),
			otlptracehttp.WithTimeout(3*time.Second),
			otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}),
			otlptracehttp.WithHTTPClient(client),
		)
		if err != nil {
			client.CloseIdleConnections()
			return nil, errors.New("创建 OTLP 导出器失败，请检查端点配置")
		}
	}
	t.exporter = &reportingExporter{inner: exporter, warnings: warnings}
	t.provider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", config.ServiceName))),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(t.exporter,
			sdktrace.WithMaxQueueSize(256), sdktrace.WithMaxExportBatchSize(32),
			sdktrace.WithBatchTimeout(time.Second), sdktrace.WithExportTimeout(3*time.Second)),
	)
	t.tracer = t.provider.Tracer("go-agent-learning")
	return t, nil
}

type runKey struct{}

// Run 统计一次运行。计数器独立于业务返回值，失败时仍能展示实际调用。
type Run struct {
	span     trace.Span
	models   atomic.Int64
	tools    atomic.Int64
	tokens   atomic.Int64
	unknown  atomic.Bool
	reported atomic.Int64
	started  time.Time
	TraceID  string
	Lesson   string
}

// StartRun 建立根 span，并在返回上下文中携带本次统计。
func (t *Telemetry) StartRun(ctx context.Context, lesson string) (context.Context, *Run) {
	ctx, span := t.tracer.Start(ctx, "agent.run", trace.WithAttributes(attribute.String("app.lesson", lesson)))
	r := &Run{span: span, started: time.Now(), Lesson: lesson, TraceID: "观测已关闭"}
	if span.SpanContext().IsValid() {
		r.TraceID = span.SpanContext().TraceID().String()
	}
	return context.WithValue(ctx, runKey{}, r), r
}

// End 在关闭 provider 前结束根 span。
func (r *Run) End(err error) {
	r.span.SetAttributes(attribute.Int64("app.llm_calls", r.models.Load()), attribute.Int64("app.tool_calls", r.tools.Load()),
		attribute.Int64("app.total_tokens", r.tokens.Load()), attribute.Bool("app.usage.complete", !r.unknown.Load() && r.reported.Load() == r.models.Load()),
		attribute.Float64("app.elapsed_ms", float64(time.Since(r.started).Microseconds())/1000))
	MarkError(r.span, err)
	r.span.End()
}

// StartModel 创建模型子 span；每个 span 对应一次实际 HTTP 请求。
func (t *Telemetry) StartModel(ctx context.Context, model string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.provider.name", "deepseek"), attribute.String("gen_ai.request.model", t.CleanText(model))}
	if r, ok := ctx.Value(runKey{}).(*Run); ok {
		attrs = append(attrs, attribute.Int64("app.llm_call", r.models.Add(1)), attribute.String("app.lesson", r.Lesson))
	}
	return t.tracer.Start(ctx, "chat "+t.CleanText(model), trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
}

// RecordUsage 只记录服务端提供的用量，缺失时保留未知标记。
func (t *Telemetry) RecordUsage(ctx context.Context, span trace.Span, response entity.ChatResponse) {
	for key, value := range map[string]*int{"gen_ai.usage.input_tokens": response.Usage.PromptTokens,
		"gen_ai.usage.output_tokens": response.Usage.CompletionTokens, "app.usage.total_tokens": response.Usage.TotalTokens} {
		if value != nil && *value >= 0 {
			span.SetAttributes(attribute.Int(key, *value))
		}
	}
	span.SetAttributes(attribute.String("gen_ai.response.model", t.CleanText(response.Model)),
		attribute.String("gen_ai.response.id", t.CleanText(response.ID)),
		attribute.StringSlice("gen_ai.response.finish_reasons", []string{t.CleanText(response.FinishReason)}))
	if r, ok := ctx.Value(runKey{}).(*Run); ok {
		if total, known := response.Usage.Total(); known && total >= 0 {
			r.tokens.Add(int64(total))
			r.reported.Add(1)
		} else {
			r.unknown.Store(true)
		}
	}
}

// StartCheck 创建不调用模型的观测检查 span。
func (t *Telemetry) StartCheck(ctx context.Context) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, "observability.check")
}

// FailedSpans 是未确认整批导出成功的数量，不是服务器精确丢失计数。
func (t *Telemetry) FailedSpans() int64 {
	if t.exporter == nil {
		return 0
	}
	return t.exporter.failed.Load()
}

// Close 使用调用方提供的有界上下文刷新并关闭队列。
func (t *Telemetry) Close(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		if t.http != nil {
			defer t.http.CloseIdleConnections()
		}
		t.closeErr = errors.Join(t.provider.ForceFlush(ctx), t.provider.Shutdown(ctx))
		if t.closeErr != nil {
			fmt.Fprintln(t.warnings, "[观测] 刷新或关闭超时，记录可能不完整；请按 trace_id 查询。")
		}
	})
	return t.closeErr
}

// MarkError 不采集异常正文、堆栈或鉴权信息。
func MarkError(span trace.Span, err error) {
	if err != nil {
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.SetStatus(codes.Error, "步骤失败")
	}
}

var sensitiveKey = regexp.MustCompile(`(?i)authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token`)
var authValue = regexp.MustCompile(`(?i)\b(?:Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+`)
var modelKey = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)
var jsonSecret = regexp.MustCompile(`(?i)("(?:authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token)"\s*:\s*)("(?:\\.|[^"\\])*"|[^,}\]\s]+)`)

// CleanText 清理已知密钥和常见凭据形式，也适用于无效 JSON 的错误响应。
func (t *Telemetry) CleanText(text string) string {
	for _, secret := range t.secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	text = authValue.ReplaceAllString(text, "[REDACTED]")
	text = modelKey.ReplaceAllString(text, "[REDACTED]")
	return jsonSecret.ReplaceAllString(text, `${1}"[REDACTED]"`)
}

// 同时限制对象层级和字符串内 JSON 的解码层级，超限部分整体脱敏。
const maxSanitizeDepth = 64

func (t *Telemetry) clean(value any, depth int) any {
	if depth >= maxSanitizeDepth {
		return "[REDACTED]"
	}
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if sensitiveKey.MatchString(key) {
				value[key] = "[REDACTED]"
			} else {
				value[key] = t.clean(child, depth+1)
			}
		}
	case []any:
		for i, child := range value {
			value[i] = t.clean(child, depth+1)
		}
	case string:
		trimmed := strings.TrimSpace(value)
		if len(trimmed) > 0 && strings.ContainsRune(`{["`, rune(trimmed[0])) {
			if nested, ok := decodeJSON(value); ok {
				original, err := json.Marshal(nested)
				if err != nil {
					return "[REDACTED]"
				}
				encoded, err := json.Marshal(t.clean(nested, depth+1))
				if err != nil {
					return "[REDACTED]"
				}
				// 未发生脱敏时保留原有空白和转义；参数仍然是字符串。
				if string(encoded) != string(original) {
					return string(encoded)
				}
			}
		}
		return t.CleanText(value)
	}
	return value
}

func decodeJSON(text string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, false
	}
	return value, true
}

func (t *Telemetry) safeJSON(raw []byte) string {
	if value, ok := decodeJSON(string(raw)); ok {
		encoded, err := json.Marshal(t.clean(value, 0))
		if err == nil {
			return string(encoded)
		}
	}
	return t.CleanText(string(raw))
}

// Payload 记录 JSON 正文的脱敏副本；字符数按 Unicode 字符计算。
func (t *Telemetry) Payload(span trace.Span, key string, raw []byte) string {
	if !t.config.CaptureContent {
		return ""
	}
	text := t.safeJSON(raw)
	chars := []rune(text)
	truncated := len(chars) > t.config.MaxChars
	span.SetAttributes(attribute.Int(key+".chars", len(chars)), attribute.Bool(key+".truncated", truncated))
	if truncated {
		text = string(chars[:t.config.MaxChars]) + "…[TRUNCATED]"
	}
	span.SetAttributes(attribute.String(key, text))
	return text
}

// Preview 将完整模型 JSON 放入一条文本消息，避免界面省略协议字段。
func (t *Telemetry) Preview(span trace.Span, key, role string, raw []byte) {
	if !t.config.CaptureContent {
		return
	}
	preview := []any{map[string]any{"role": role, "parts": []any{map[string]any{"type": "text", "content": t.safeJSON(raw)}}}}
	encoded, err := json.Marshal(preview)
	if err == nil {
		t.Payload(span, key, encoded)
	}
}

// Tools 给受控注册表增加观测，不改变工具的业务返回值。
type Tools struct {
	Registry  *service.Registry
	Telemetry *Telemetry
}

// Definitions 返回工具模式。
func (t *Tools) Definitions() []entity.ToolDefinition { return t.Registry.Definitions() }

// Dispatch 每次分发记录一个工具子 span，包括被拒绝的工具调用。
func (t *Tools) Dispatch(ctx context.Context, call entity.ToolCall) (output entity.ToolResult, err error) {
	attrs := []attribute.KeyValue{attribute.String("gen_ai.operation.name", "execute_tool"),
		attribute.String("app.tool.name", t.Telemetry.CleanText(call.Function.Name)), attribute.String("app.tool.call.id", t.Telemetry.CleanText(call.ID))}
	if r, ok := ctx.Value(runKey{}).(*Run); ok {
		r.tools.Add(1)
		attrs = append(attrs, attribute.Int64("app.llm_call", r.models.Load()))
	}
	ctx, span := t.Telemetry.tracer.Start(ctx, "Tool call", trace.WithAttributes(attrs...))
	started := time.Now()
	defer func() {
		MarkError(span, err)
		if err == nil && !output.OK {
			span.SetStatus(codes.Error, output.Error)
			span.SetAttributes(attribute.String("app.tool.error", output.Error))
		}
		span.SetAttributes(attribute.Float64("app.elapsed_ms", float64(time.Since(started).Microseconds())/1000))
		span.End()
	}()
	raw, _ := json.Marshal(call)
	t.Telemetry.Payload(span, "app.tool.request", raw)
	var arguments any
	if json.Unmarshal([]byte(call.Function.Arguments), &arguments) != nil {
		arguments = call.Function.Arguments
	}
	input, _ := json.Marshal(map[string]any{"function": call.Function.Name, "call_id": call.ID, "arguments": arguments})
	t.Telemetry.Payload(span, "gen_ai.input.messages", input)
	output, err = t.Registry.Dispatch(ctx, call)
	if err == nil {
		raw, _ = json.Marshal(output)
		t.Telemetry.Payload(span, "app.tool.response", raw)
		t.Telemetry.Payload(span, "gen_ai.output.messages", raw)
	}
	return output, err
}
