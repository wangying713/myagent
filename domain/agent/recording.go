package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// recordingModel 累计单次运行的调用、用量和脱敏步骤，不可跨运行共享。
type recordingModel struct {
	inner    ChatClient
	observer Observer
	result   *ExecutionResult
	limits   Limits
}

func (m *recordingModel) Chat(ctx context.Context, input Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if m.result.ModelCalls >= m.limits.ModelCalls || m.result.KnownTokens >= m.limits.Tokens {
		return Response{}, ErrBudgetExceeded
	}
	started := time.Now()
	m.result.ModelCalls++
	response, err := m.inner.Chat(ctx, input)
	if err == nil {
		err = response.Validate()
	}
	if total, known := response.Usage.Total(); known && total <= math.MaxInt-m.result.KnownTokens {
		m.result.KnownTokens += total
	} else {
		m.result.UsageComplete = false
	}
	request := response.RequestJSON
	if len(request) == 0 {
		// Request 仅包含可编码的协议字段；失败时也不能丢失业务错误。
		if encoded, encodeErr := json.Marshal(input); encodeErr == nil {
			request = encoded
		}
	}
	step := ExecutionStep{Kind: "model", Name: fmt.Sprintf("第 %d 次模型请求", m.result.ModelCalls), Request: m.observer.Safe(request), Response: m.observer.Safe(response.Raw), ElapsedMS: time.Since(started).Milliseconds(), FinishReason: response.FinishReason, Usage: &response.Usage}
	if err != nil {
		step.Error = "模型调用未完成"
	}
	m.result.Steps = append(m.result.Steps, step)
	return response, err
}

// recordingTools 为单次运行记录工具结果，并保证已开始的观测在返回时结束。
type recordingTools struct {
	inner    Tools
	observer Observer
	result   *ExecutionResult
}

func (t *recordingTools) Definitions() []ToolDefinition { return t.inner.Definitions() }

func (t *recordingTools) Dispatch(ctx context.Context, call ToolCall) (output ToolResult, err error) {
	if err := ctx.Err(); err != nil {
		return output, err
	}
	ctx, observation := t.observer.StartTool(ctx, call)
	started := time.Now()
	defer func() { observation.End(output, err) }()
	t.result.ToolCalls++
	request, encodeErr := json.Marshal(call)
	if encodeErr != nil {
		return output, fmt.Errorf("记录工具请求: %w", encodeErr)
	}
	output, err = t.inner.Dispatch(ctx, call)
	response, encodeErr := json.Marshal(output)
	if encodeErr != nil {
		return output, fmt.Errorf("记录工具结果: %w", encodeErr)
	}
	step := ExecutionStep{Kind: "tool", Name: t.observer.Clean(call.Function.Name), Request: t.observer.Safe(request), Response: t.observer.Safe(response), ElapsedMS: time.Since(started).Milliseconds()}
	if !output.OK {
		step.Error = t.observer.Clean(output.Error)
	}
	if err != nil {
		step.Error = "工具执行未完成"
	}
	t.result.Steps = append(t.result.Steps, step)
	return output, err
}
