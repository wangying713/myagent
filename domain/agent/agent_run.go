package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// agentRun 保存这一次 Agent 运行的数据，负责调用模型和工具、记录结果及结束时收尾。
// 每次运行单独创建，调用结束后不再复用。
type agentRun struct {
	observer Observer       // 负责执行记录的脱敏和最终回答的清理。
	result   *ChatRsp       // 保存这次运行的回答、步骤和统计。
	chatCli  ChatClient     // 实际发起模型请求的客户端。
	limits   Limits         // 本次运行允许的调用次数与 token 上限。
	run      RunObservation // 这次运行的链路记录，结束时一并收尾。
	started  time.Time      // 记下开始时间，用来计算总耗时。
}

// prepareRun 检查问题，创建运行记录，并保存本次使用的客户端和预算。
// 模型请求由调用方随后发起，执行结束后由 end 收尾。
func (c *Client) prepareRun(ctx context.Context, result *ChatRsp, req RunDetails) (context.Context, *agentRun, error) {
	// 统一在这里检查，调用方补好默认问题即可，避免新增入口时漏掉校验。
	if len(req.Questions) == 0 {
		return ctx, nil, ErrInput
	}
	for _, question := range req.Questions {
		if err := validateQuestion(question); err != nil {
			return ctx, nil, err
		}
	}
	// 把允许调用模型、工具的次数和 token 上限也记下来。
	req.Limits = c.limits
	// 如果模型能提供自己的名称、地址等信息，就一起记下来。
	if chatCli, ok := c.chatCli.(ModelInfoProvider); ok {
		info := chatCli.ModelInfo()
		req.Model = &info
	}
	// 开始记录这次运行，并把查询记录用的 trace ID 放进结果。
	// 后续沿用返回的 ctx，模型和工具的记录就能串到同一次运行下。
	ctx, run := c.observer.StartRun(ctx, result.Operation, req)
	result.TraceID = run.TraceID()
	// 给这次运行准备一份单独的数据，并记下开始时间。
	return ctx, &agentRun{
		observer: c.observer,
		result:   result,
		chatCli:  c.chatCli,
		limits:   c.limits,
		run:      run,
		started:  time.Now(),
	}, nil
}

// callModel 先检查预算，再请求模型，并记下用量、结果和耗时；失败时也保留记录。
func (r *agentRun) callModel(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if r.result.ModelCalls >= r.limits.ModelCalls || r.result.KnownTokens >= r.limits.Tokens {
		return Response{}, ErrBudgetExceeded
	}
	started := time.Now()
	r.result.ModelCalls++
	response, err := r.chatCli.Chat(ctx, req)
	if err == nil {
		err = response.Validate()
	}
	if total, known := response.Usage.Total(); known && total <= math.MaxInt-r.result.KnownTokens {
		r.result.KnownTokens += total
	} else {
		r.result.UsageComplete = false
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
		Name:         fmt.Sprintf("第 %d 次模型请求", r.result.ModelCalls),
		Request:      r.observer.Safe(request),
		Response:     r.observer.Safe(response.Raw),
		ElapsedMS:    time.Since(started).Milliseconds(),
		FinishReason: response.FinishReason,
		Usage:        &response.Usage,
	}
	if err != nil {
		step.Error = "模型调用未完成"
	}
	r.result.Steps = append(r.result.Steps, step)
	return response, err
}

// callTool 执行工具，记录调用次数、结果和耗时，再把结果整理成模型能继续读取的消息。
func (r *agentRun) callTool(ctx context.Context, tools Tools, req ToolCall) (message Message, err error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	ctx, observation := r.observer.StartTool(ctx, req)
	started := time.Now()
	var output ToolResult
	// 无论执行成功还是中途出错，都用最终结果和错误结束记录。
	defer func() {
		observation.End(output, err)
	}()
	r.result.ToolCalls++
	// ToolCall 只包含字符串和简单结构体，可以直接编码。
	request, _ := json.Marshal(req)
	output, err = tools.Dispatch(ctx, req)
	// 编码后的完整结果发给模型，展示记录另做脱敏，避免改变后续对话内容。
	response, encodeErr := json.Marshal(output)
	if encodeErr != nil {
		return Message{}, fmt.Errorf("记录工具结果: %w", encodeErr)
	}
	step := ExecutionStep{
		Kind:      StepKindTool,
		Name:      r.observer.Clean(req.Function.Name),
		Request:   r.observer.Safe(request),
		Response:  r.observer.Safe(response),
		ElapsedMS: time.Since(started).Milliseconds(),
	}
	if err != nil {
		step.Error = "工具执行未完成"
	} else if !output.OK {
		step.Error = r.observer.Clean(output.Error)
	}
	r.result.Steps = append(r.result.Steps, step)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Role:       RoleTool,
		ToolCallID: req.ID,
		Content:    string(response),
	}, nil
}

// end 根据最终错误收尾本次业务运行，更新结果后结束运行观测。
// 调用方通过 defer 闭包传入最终 err，使成功返回和提前失败都会执行收尾。
func (r *agentRun) end(err error) {
	// 以整个业务流程的耗时和最终错误确定运行状态；失败时保留已有回答、步骤和用量。
	r.result.ElapsedMS = time.Since(r.started).Milliseconds()
	r.result.Completed = err == nil
	// 返回结果与观测展示共用脱敏后的回答，先完成脱敏再交给观测接口。
	r.result.Answer = r.observer.Clean(r.result.Answer)
	r.run.End(*r.result, err)
}
