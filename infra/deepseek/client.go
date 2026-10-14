// Package deepseek 使用标准 HTTP 调用模型，不自动重试，也不执行模型生成的代码。
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

	"go.opentelemetry.io/otel/attribute"

	"sakelei/ai/domain/agent/entity"
	"sakelei/ai/infra/openobserve"
)

// Config 提供方地址和密钥由入口注入，不在客户端读取环境变量。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// Client 满足 Agent 的 ChatClient 接口。HTTP 客户端可注入用于离线测试。
type Client struct {
	config    Config
	http      *http.Client
	telemetry *openobserve.Telemetry
}

// ModelError 保存可识别的 HTTP 状态，不在错误字符串中暴露服务端原文。
type ModelError struct {
	StatusCode int
	Reason     string
}

func (e *ModelError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("模型调用失败：HTTP %d（%s）", e.StatusCode, e.Reason)
	}
	return "模型响应无效：" + e.Reason
}

// New 创建客户端；未注入 HTTP 客户端时禁用环境代理并使用明确超时。
func New(config Config, telemetry *openobserve.Telemetry, client *http.Client) (*Client, error) {
	if config.APIKey == "" || config.APIKey == "put-your-key-here" {
		return nil, errors.New("请配置 DEEPSEEK_API_KEY")
	}
	parsed, err := url.Parse(config.BaseURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("模型地址必须是无凭据、无查询参数的 HTTP(S) URL")
	}
	if config.Timeout <= 0 || config.Model == "" || telemetry == nil {
		return nil, errors.New("模型名、正数超时和观测器不能为空")
	}
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client = &http.Client{Transport: transport}
	}
	// 禁止重定向，避免鉴权意外发送到其他端点，也让一次 span 对应一次请求。
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{config: config, telemetry: telemetry, http: &copyClient}, nil
}

// Close 释放空闲连接，调用方应在所有请求结束后调用。
func (c *Client) Close() { c.http.CloseIdleConnections() }

// Chat 记录实际请求和响应，再检查 HTTP 状态及回答完整性。
func (c *Client) Chat(ctx context.Context, request entity.ChatRequest) (result entity.ChatResponse, err error) {
	if request.Stream {
		return result, errors.New("第一阶段不支持流式请求")
	}
	if request.Model == "" {
		request.Model = c.config.Model
	}
	if request.MaxTokens <= 0 {
		request.MaxTokens = 512
	}
	if request.Thinking == nil {
		request.Thinking = &entity.Thinking{Type: "disabled"}
	}
	if len(request.Tools) == 0 {
		request.ToolChoice = ""
	}
	if len(request.Messages) == 0 {
		return result, errors.New("模型请求的 messages 不能为空")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, fmt.Errorf("编码模型请求: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return result, errors.New("创建模型 HTTP 请求失败")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	ctx, span := c.telemetry.StartModel(ctx, request.Model)
	httpRequest = httpRequest.WithContext(ctx)
	started := time.Now()
	defer func() {
		openobserve.MarkError(span, err)
		span.SetAttributes(attribute.Float64("app.elapsed_ms", float64(time.Since(started).Microseconds())/1000))
		span.End()
	}()
	span.SetAttributes(attribute.String("http.request.method", "POST"), attribute.String("url.full", c.telemetry.CleanText(endpoint)),
		attribute.String("server.address", httpRequest.URL.Hostname()), attribute.Int("http.request.body.size", len(raw)))
	c.telemetry.Payload(span, "app.request", raw)
	c.telemetry.Preview(span, "gen_ai.input.messages", "user", raw)
	response, err := c.http.Do(httpRequest)
	if err != nil {
		return result, fmt.Errorf("发送模型请求: %w", err)
	}
	defer response.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	for _, name := range []string{"Content-Type", "X-Request-Id", "Retry-After"} {
		if value := response.Header.Get(name); value != "" {
			span.SetAttributes(attribute.String("http.response.header."+strings.ToLower(name), c.telemetry.CleanText(value)))
		}
	}
	// 正文采集限长与网络内存上限分开：超大响应明确失败，不伪装成完整答案。
	const responseLimit = 4 << 20
	body, readErr := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	span.SetAttributes(attribute.Int("http.response.body.size", len(body)))
	c.telemetry.Payload(span, "app.response", body)
	c.telemetry.Preview(span, "gen_ai.output.messages", "assistant", body)
	if readErr != nil {
		return result, fmt.Errorf("读取模型响应: %w", readErr)
	}
	if len(body) > responseLimit {
		return result, &ModelError{Reason: "响应超过 4 MiB 内存上限"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, &ModelError{StatusCode: response.StatusCode, Reason: "请查看脱敏后的响应正文"}
	}
	var decoded struct {
		ID      string       `json:"id"`
		Model   string       `json:"model"`
		Usage   entity.Usage `json:"usage"`
		Choices []struct {
			Message      json.RawMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &decoded) != nil || len(decoded.Choices) != 1 {
		return result, &ModelError{Reason: "需要一个有效的 choices 回答"}
	}
	choice := decoded.Choices[0]
	var message entity.Message
	if json.Unmarshal(choice.Message, &message) != nil {
		return result, &ModelError{Reason: "assistant 消息结构无效"}
	}
	message.Raw = bytes.Clone(choice.Message)
	result = entity.ChatResponse{ID: decoded.ID, Model: decoded.Model, Message: message,
		FinishReason: choice.FinishReason, Usage: decoded.Usage, Raw: bytes.Clone(body)}
	c.telemetry.RecordUsage(ctx, span, result)
	if validationErr := result.Validate(); validationErr != nil {
		return result, &ModelError{Reason: validationErr.Error()}
	}
	return result, nil
}
