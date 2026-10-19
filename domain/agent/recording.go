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
	modelCli ChatClient // 实际发起模型请求的客户端。
	observer Observer   // 用于生成脱敏、限长展示副本的观测接口。
	result   *Result    // 本次运行共享的累计结果。
	limits   Limits     // 本次运行的模型次数与用量预算。
}

func (m *recordingModel) Chat(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if m.result.ModelCalls >= m.limits.ModelCalls || m.result.KnownTokens >= m.limits.Tokens {
		return Response{}, ErrBudgetExceeded
	}
	started := time.Now()
	m.result.ModelCalls++
	response, err := m.modelCli.Chat(ctx, req)
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
		if encoded, encodeErr := json.Marshal(req); encodeErr == nil {
			request = encoded
		}
	}
	step := ExecutionStep{
		Kind:         StepKindModel,
		Name:         fmt.Sprintf("第 %d 次模型请求", m.result.ModelCalls),
		Request:      m.observer.Safe(request),
		Response:     m.observer.Safe(response.Raw),
		ElapsedMS:    time.Since(started).Milliseconds(),
		FinishReason: response.FinishReason,
		Usage:        &response.Usage,
	}
	if err != nil {
		step.Error = "模型调用未完成"
	}
	m.result.Steps = append(m.result.Steps, step)
	return response, err
}

// recordingTools 为单次运行记录工具结果，并保证已开始的观测在返回时结束。
type recordingTools struct {
	tools    Tools    // 实际执行已注册工具的工具集。
	observer Observer // 负责工具调用观测与展示脱敏的接口。
	result   *Result  // 本次运行共享的调用计数与步骤记录。
}

func (t *recordingTools) Definitions() []ToolDefinition {
	return t.tools.Definitions()
}

// Dispatch 调用实际工具，并记下这次调用的输入、结果、耗时和调用次数，方便查看执行过程。
// 具体计算交给 tools，例如乘法会交给 MultiplyTools 执行。
func (t *recordingTools) Dispatch(ctx context.Context, req ToolCall) (output ToolResult, err error) {
	// 如果这次任务已经取消，就不再调用工具。
	if err := ctx.Err(); err != nil {
		return output, err
	}
	// 开始记录这次工具调用，并记下开始时间。
	ctx, observation := t.observer.StartTool(ctx, req)
	started := time.Now()
	// 等函数返回时，再用最终结果和错误结束记录；中途出错也会执行。
	defer func() {
		observation.End(output, err)
	}()
	// 调用次数加一，把工具请求转成 JSON，留作执行记录。
	// ToolCall 只包含字符串和简单结构体，可以直接编码。
	t.result.ToolCalls++
	request, _ := json.Marshal(req)
	// 在这里调用真正的工具。即使工具返回错误，也继续尝试记录它的结果。
	output, err = t.tools.Dispatch(ctx, req)
	response, encodeErr := json.Marshal(output)
	if encodeErr != nil {
		return output, fmt.Errorf("记录工具结果: %w", encodeErr)
	}
	// 整理这一步的记录：隐藏敏感信息、限制正文长度，再加上实际耗时。
	step := ExecutionStep{
		Kind:      StepKindTool,
		Name:      t.observer.Clean(req.Function.Name),
		Request:   t.observer.Safe(request),
		Response:  t.observer.Safe(response),
		ElapsedMS: time.Since(started).Milliseconds(),
	}
	// 工具拒绝时记下原因；取消等执行错误则显示“工具执行未完成”。
	if err != nil {
		step.Error = "工具执行未完成"
	} else if !output.OK {
		step.Error = t.observer.Clean(output.Error)
	}
	t.result.Steps = append(t.result.Steps, step)
	return output, err
}
