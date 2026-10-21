package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"

	"go.opentelemetry.io/otel/attribute"
)

// streamResponseLimit 限制单次模型流最多读取 4 MiB。
const streamResponseLimit = 4 << 20

// Stream 请求真实的模型文本流，逐事件解析；断流、截断和缺少结束标志都不能算成功。
func (c *Client) Stream(ctx context.Context, req agent.StreamInput) (result *agent.Response, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Emit == nil || len(req.Request.Messages) == 0 || len(req.Request.Tools) != 0 {
		return nil, errors.New("文本流需要消息和分段接收函数，且不支持工具调用")
	}
	request := req.Request
	request.Model = c.config.Model
	request.MaxTokens = defaultMaxTokens
	request.Thinking.Type = defaultThinkingType
	request.Stream = true
	request.ToolChoice = ""
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.config.BaseURL, "/")+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	result = &agent.Response{
		RequestJSON: raw,
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	ctx, span := c.observeCli.StartClient(ctx, "/chat/completions", attribute.String("gen_ai.operation.name", "chat"), attribute.String("gen_ai.request.model", c.config.Model), attribute.Bool("app.stream", true))
	started := time.Now()
	defer func() {
		c.observeCli.Payload(span, "app.response", result.Raw)
		openobserve.End(span, started, err)
	}()
	c.observeCli.Payload(span, "app.request", raw)
	response, err := c.httpCli.Do(httpReq.WithContext(ctx))
	if err != nil {
		return result, fmt.Errorf("请求模型文本流: %w", err)
	}
	defer response.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, &ModelError{
			Status: response.StatusCode,
		}
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return result, agent.ErrInvalidResponse
	}
	result.Message.Role = agent.RoleAssistant
	err = readTextStream(ctx, response.Body, streamReceiver{
		result: result,
		emit:   req.Emit,
	})
	return result, err
}

// streamReceiver 保存当前流独占的累计结果和分段处理函数。
type streamReceiver struct {
	result *agent.Response    // 保存收到的正文、用量和结束原因。
	emit   func(string) error // 同步写出新内容，失败立即停止。
}

// readTextStream 按空行而不是网络分片切分 SSE，支持注释、CRLF 和多行 data。
func readTextStream(ctx context.Context, body io.Reader, receiver streamReceiver) error {
	limited := &io.LimitedReader{
		R: body,
		N: streamResponseLimit + 1,
	}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var data []string
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		receiver.result.Raw = append(receiver.result.Raw, []byte(line+"\n")...)
		if len(receiver.result.Raw) > streamResponseLimit || limited.N == 0 {
			return agent.ErrInvalidResponse
		}
		if line == "" {
			if len(data) == 0 {
				continue
			}
			text := strings.Join(data, "\n")
			data = nil
			if text == "[DONE]" {
				return receiver.result.Validate()
			}
			if err := receiver.chunk(text); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取模型流: %w", err)
	}
	return agent.ErrInvalidResponse
}

// chunk 只接收一个候选的文本增量，结束后不再接受正文或工具调用。
func (r streamReceiver) chunk(text string) error {
	var chunk struct {
		Usage   *agent.Usage `json:"usage"` // 末尾可能提供用量，省略时保持未知。
		Choices []struct {
			Index int `json:"index"` // 只接受编号为零的唯一候选。
			Delta struct {
				Role      string          `json:"role"`       // 首段可能提供 assistant 角色。
				Content   string          `json:"content"`    // 本次新收到的正文。
				ToolCalls json.RawMessage `json:"tool_calls"` // 文本流不接受工具调用。
			} `json:"delta"` // 当前增量。
			FinishReason string `json:"finish_reason"` // 非空表示生成结束。
		} `json:"choices"` // 只接受一个文本候选，或单独的用量事件。
	}
	if json.Unmarshal([]byte(text), &chunk) != nil || len(chunk.Choices) > 1 || (len(chunk.Choices) == 0 && chunk.Usage == nil) {
		return agent.ErrInvalidResponse
	}
	if chunk.Usage != nil {
		r.result.Usage = *chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]
	if choice.Index != 0 || (choice.Delta.Role != "" && choice.Delta.Role != agent.RoleAssistant) || len(choice.Delta.ToolCalls) > 0 || r.result.FinishReason != "" {
		return agent.ErrInvalidResponse
	}
	if choice.Delta.Content != "" {
		r.result.Message.Content += choice.Delta.Content
		if err := r.emit(choice.Delta.Content); err != nil {
			return err
		}
	}
	if choice.FinishReason != "" {
		if choice.FinishReason != agent.FinishReasonStop {
			return agent.ErrInvalidResponse
		}
		r.result.FinishReason = choice.FinishReason
	}
	return nil
}
