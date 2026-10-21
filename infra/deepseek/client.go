// Package deepseek 通过标准库 HTTP 调用普通及文本流式 Chat Completions。
package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Config 保存模型鉴权、服务地址、模型名及单次调用超时，由启动入口显式传入。
type Config struct {
	APIKey  string        // 模型服务鉴权密钥。
	BaseURL string        // 模型 API 基础地址。
	Model   string        // 请求使用的模型名称。
	Timeout time.Duration // 单次模型调用的超时时长。
}

const (
	providerName        = "deepseek"
	defaultMaxTokens    = 512
	defaultThinkingType = "disabled"
)

// Client 执行模型请求并记录脱敏观测；构造后可并发调用。
type Client struct {
	config     Config              // 构造时保存的模型服务配置。
	httpCli    *http.Client        // 发送模型请求的 HTTP 客户端。
	observeCli *openobserve.Client // 记录调用链路及脱敏正文的观测组件。
}

// ModelError 保存模型服务的非 2xx 状态码，调用方可用 errors.As 识别。
type ModelError struct {
	Status int // 模型服务返回的非 2xx HTTP 状态码。
}

// Error 返回状态码说明，不包含模型服务的响应正文。
func (e *ModelError) Error() string {
	return fmt.Sprintf("模型服务返回 HTTP %d", e.Status)
}

// New 校验配置并创建客户端，httpCli 为 nil 时使用独立的 HTTP Transport。
// 传入 httpCli 时复用其 Transport；调用方应在业务结束后调用 Client.Close。
func New(cfg Config, observeCli *openobserve.Client, httpCli *http.Client) (*Client, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("模型地址应为不含凭据和查询参数的 HTTP(S) URL")
	}
	if cfg.APIKey == "" || cfg.APIKey == "put-your-key-here" {
		return nil, errors.New("请填写 DEEPSEEK_API_KEY")
	}
	if cfg.Model == "" || cfg.Timeout <= 0 || observeCli == nil {
		return nil, errors.New("模型名、超时与观测依赖无效")
	}
	if httpCli == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		httpCli = &http.Client{
			Transport: transport,
		}
	}
	copiedHTTPCli := *httpCli
	copiedHTTPCli.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{
		config:     cfg,
		httpCli:    &copiedHTTPCli,
		observeCli: observeCli,
	}, nil
}

// Close 关闭 Transport 的空闲连接，不取消正在执行的请求。
func (c *Client) Close() {
	c.httpCli.CloseIdleConnections()
}

// ModelInfo 返回运行观测使用的白名单配置，不暴露鉴权和 HTTP 客户端。
func (c *Client) ModelInfo() openobserve.ModelInfo {
	return openobserve.ModelInfo{
		Provider:     providerName,
		Name:         c.config.Model,
		Endpoint:     strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions",
		Timeout:      c.config.Timeout,
		MaxTokens:    defaultMaxTokens,
		ThinkingType: defaultThinkingType,
		Stream:       false,
	}
}

// Chat 执行一次模型请求，按 ctx 和配置超时中较早的截止时间取消，不自动重试。
// 请求使用配置中的模型并关闭流式和思考模式；失败时仍返回已取得的请求或响应原文。
func (c *Client) Chat(ctx context.Context, req agent.Request) (result *agent.Response, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Stream {
		return nil, errors.New("流式请求请使用 Stream 方法")
	}
	req.Model = c.config.Model
	req.Thinking.Type = defaultThinkingType
	req.MaxTokens = defaultMaxTokens
	req.Stream = false
	if len(req.Tools) == 0 {
		req.ToolChoice = ""
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("消息不能为空")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("编码模型请求: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("无法创建模型请求")
	}
	result = &agent.Response{
		RequestJSON: raw,
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	ctx, span := c.observeCli.Tracer.Start(ctx, httpReq.URL.EscapedPath(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.request.model", c.config.Model),
			attribute.String("gen_ai.provider.name", providerName),
		),
	)
	started := time.Now()
	defer func() {
		openobserve.End(span, started, err)
	}()
	c.observeCli.Payload(span, "app.request", raw)
	span.SetAttributes(attribute.String("http.request.method", "POST"), attribute.String("server.address", httpReq.URL.Hostname()), attribute.String("url.full", httpReq.URL.String()))
	resp, err := c.httpCli.Do(httpReq.WithContext(ctx))
	if err != nil {
		return result, fmt.Errorf("请求模型服务: %w", err)
	}
	defer resp.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	const responseLimit = 4 << 20
	result.Raw, err = io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	c.observeCli.Payload(span, "app.response", result.Raw)
	if err != nil {
		return result, fmt.Errorf("读取模型响应: %w", err)
	}
	if len(result.Raw) > responseLimit {
		return result, fmt.Errorf("模型响应超过 4 MiB: %w", agent.ErrInvalidResponse)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, &ModelError{
			Status: resp.StatusCode,
		}
	}
	var decoded struct {
		Usage   agent.Usage `json:"usage"` // 模型响应中的 token 用量。
		Choices []struct {
			Message      json.RawMessage `json:"message"`       // 保留扩展字段与 null 的原始 assistant JSON。
			FinishReason string          `json:"finish_reason"` // 当前候选结果的模型结束原因。
		} `json:"choices"` // 模型返回的候选结果，当前要求恰好一项。
	}
	if json.Unmarshal(result.Raw, &decoded) != nil || len(decoded.Choices) != 1 {
		return result, agent.ErrInvalidResponse
	}
	result.Usage = decoded.Usage
	result.FinishReason = decoded.Choices[0].FinishReason
	if json.Unmarshal(decoded.Choices[0].Message, &result.Message) != nil {
		return result, agent.ErrInvalidResponse
	}
	result.Message.Raw = bytes.Clone(decoded.Choices[0].Message)
	if total, known := result.Usage.Total(); known {
		span.SetAttributes(attribute.Int("app.usage.total_tokens", total))
	}
	span.SetAttributes(attribute.StringSlice("gen_ai.response.finish_reasons", []string{result.FinishReason}))
	return result, result.Validate()
}
