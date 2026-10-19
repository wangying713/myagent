package agent

import (
	"context"
	"errors"
	"time"
)

// StreamClient 是文本流式演示需要的能力；每段内容在读取到时同步交给 Emit。
type StreamClient interface {
	// Stream 收到取消或 Emit 返回错误后立即停止，不启动脱离请求的协程。
	Stream(ctx context.Context, req StreamInput) (Response, error)
}

// StreamInput 同时保存模型请求和本次请求独占的分段接收函数。
type StreamInput struct {
	Request Request            // 模型请求，不携带鉴权信息。
	Emit    func(string) error // 同步处理一段新文本，返回错误则停止读取。
}

// DemoStreamInput 保存要问的问题和浏览器连接对应的分段写出函数。
type DemoStreamInput struct {
	Question string             // 空时使用 context 的生活化解释示例。
	Emit     func(string) error // 当前请求独占的分段接收函数，不能为 nil。
}

// DemoStream 沿用当前请求上下文实时生成，失败时保留已经收到的文字与执行记录。
func (c *Client) DemoStream(ctx context.Context, req DemoStreamInput) (result Result, err error) {
	if req.Question == "" {
		req.Question = "用生活中的例子解释 Go 的 context，分三点说明。"
	}
	if req.Emit == nil {
		return result, ErrInput
	}
	chatCli, ok := c.chatCli.(StreamClient)
	if !ok {
		return result, errors.New("模型客户端不支持流式回答")
	}
	result = NewResult(DemoStream, c.observer.Enabled())
	ctx, run, err := c.prepareRun(ctx, &result, RunDetails{
		Questions: []string{req.Question},
		Prompt:    ChatPrompt,
	})
	if err != nil {
		return result, err
	}
	defer func() {
		run.end(err)
	}()
	started := time.Now()
	result.ModelCalls = 1
	response, err := chatCli.Stream(ctx, StreamInput{
		Request: Request{
			Messages: demoMessages(ChatPrompt, req.Question),
		},
		Emit: func(text string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			result.Answer += text
			return req.Emit(text)
		},
	})
	if err == nil {
		err = response.Validate()
	}
	if total, known := response.Usage.Total(); known {
		result.KnownTokens = total
	} else {
		result.UsageComplete = false
	}
	step := ExecutionStep{
		Kind:         StepKindModel,
		Name:         "实时流式模型请求",
		Request:      c.observer.Safe(response.RequestJSON),
		Response:     c.observer.Safe(response.Raw),
		ElapsedMS:    time.Since(started).Milliseconds(),
		FinishReason: response.FinishReason,
		Usage:        &response.Usage,
	}
	if err != nil {
		step.Error = "流式回答未完成，已保留收到的内容"
	}
	result.Steps = append(result.Steps, step)
	return result, err
}
