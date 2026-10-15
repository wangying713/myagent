// Package deepseek 通过标准库 HTTP 调用非流式 Chat Completions。
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
)

type Config struct {
	APIKey, BaseURL, Model string
	Timeout                time.Duration
}

type Client struct {
	config    Config
	http      *http.Client
	telemetry *openobserve.Telemetry
}

type ModelError struct{ Status int }

func (e *ModelError) Error() string { return fmt.Sprintf("模型服务返回 HTTP %d", e.Status) }

func New(cfg Config, telemetry *openobserve.Telemetry, client *http.Client) (*Client, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("模型地址应为不含凭据和查询参数的 HTTP(S) URL")
	}
	if cfg.APIKey == "" || cfg.APIKey == "put-your-key-here" {
		return nil, errors.New("请填写 DEEPSEEK_API_KEY")
	}
	if cfg.Model == "" || cfg.Timeout <= 0 || telemetry == nil {
		return nil, errors.New("模型名、超时与观测依赖无效")
	}
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client = &http.Client{Transport: transport}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{config: cfg, http: &copyClient, telemetry: telemetry}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) Chat(ctx context.Context, input agent.Request) (result agent.Response, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if input.Stream {
		return result, errors.New("这三个 demo 不支持流式请求")
	}
	input.Model = c.config.Model
	input.Thinking.Type = "disabled"
	input.MaxTokens = 512
	input.Stream = false
	if len(input.Tools) == 0 {
		input.ToolChoice = ""
	}
	if len(input.Messages) == 0 {
		return result, errors.New("消息不能为空")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return result, fmt.Errorf("编码模型请求: %w", err)
	}
	result.RequestJSON = raw
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return result, errors.New("无法创建模型请求")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	ctx, span := c.telemetry.StartClient(ctx, "chat "+c.config.Model, attribute.String("gen_ai.operation.name", "chat"), attribute.String("gen_ai.request.model", c.config.Model), attribute.String("gen_ai.provider.name", "deepseek"))
	started := time.Now()
	defer func() { openobserve.End(span, started, err) }()
	c.telemetry.Payload(span, "app.request", raw)
	span.SetAttributes(attribute.String("http.request.method", "POST"), attribute.String("server.address", req.URL.Hostname()))
	resp, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return result, fmt.Errorf("请求模型服务: %w", err)
	}
	defer resp.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	const responseLimit = 4 << 20
	result.Raw, err = io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	c.telemetry.Payload(span, "app.response", result.Raw)
	if err != nil {
		return result, fmt.Errorf("读取模型响应: %w", err)
	}
	if len(result.Raw) > responseLimit {
		return result, fmt.Errorf("模型响应超过 4 MiB: %w", agent.ErrInvalidResponse)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, &ModelError{Status: resp.StatusCode}
	}
	var decoded struct {
		Usage   agent.Usage `json:"usage"`
		Choices []struct {
			Message      json.RawMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
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
