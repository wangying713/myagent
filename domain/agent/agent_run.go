package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// ErrBudgetExceeded 表示继续执行将超出模型次数、工具次数或已知用量预算。
var ErrBudgetExceeded = errors.New("达到运行预算，任务未完成")

// Limits 限制单次运行的模型次数、工具次数和累计已知 token 用量，各值须为正数。
// token 预算在调用之间检查，单次模型调用可能超过剩余预算。
type Limits struct {
	ModelCalls int // 单次运行允许的最大模型调用次数。
	ToolCalls  int // 单次运行允许的最大工具调用次数。
	Tokens     int // 单次运行的累计已知 token 软预算。
}

// DefaultLimits 返回三个学习流程使用的默认运行预算。
func DefaultLimits() Limits {
	return Limits{
		ModelCalls: 5,
		ToolCalls:  8,
		Tokens:     8000,
	}
}

// RunInput 保存本次执行实际使用的输入。
type RunInput struct {
	Questions  []string         // 按执行顺序排列的问题；运行前已补齐默认值并通过校验。
	Prompt     string           // 本次执行的系统提示词。
	Tools      []ToolDefinition // 本次执行可用的工具定义；nil 表示没有工具。
	ToolChoice string           // 模型选择工具的策略；无此设置时为空。
	Limits     Limits           // 本次执行的模型、工具与 token 预算。
}

// agentRun 保存这一次 Agent 运行的数据，负责调用模型和工具、记录结果及结束时收尾。
// 每次运行单独创建，调用结束后不再复用。
type agentRun struct {
	result  *ChatRsp   // 保存这次运行的回答、步骤和统计。
	chatCli ChatClient // 实际发起模型请求的客户端。
	limits  Limits     // 本次运行允许的调用次数与 token 上限。
	started time.Time  // 记下开始时间，用来计算总耗时。
}

// prepareRun 检查问题，创建运行记录，并保存本次使用的客户端和预算。
// 模型请求由调用方随后发起，执行结束后由 end 收尾。
func (c *Client) prepareRun(result *ChatRsp, req RunInput) (*agentRun, error) {
	if len(req.Questions) == 0 {
		return nil, ErrInput
	}
	for _, question := range req.Questions {
		if err := ValidateQuestion(question); err != nil {
			return nil, err
		}
	}
	req.Limits = c.limits
	result.Input = req
	return &agentRun{
		result:  result,
		chatCli: c.chatCli,
		limits:  c.limits,
		started: time.Now(),
	}, nil
}

// callModel 先检查预算，再请求模型，并记下用量、结果和耗时；失败时也保留记录。
func (r *agentRun) callModel(ctx context.Context, req Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.result.ModelCalls >= r.limits.ModelCalls || r.result.KnownTokens >= r.limits.Tokens {
		return nil, ErrBudgetExceeded
	}
	started := time.Now()
	r.result.ModelCalls++
	response, err := r.chatCli.Chat(ctx, req)
	if err == nil {
		err = response.Validate()
	}
	var data Response
	if response != nil {
		data = *response
	}
	if total, known := data.Usage.Total(); known && total <= math.MaxInt-r.result.KnownTokens {
		r.result.KnownTokens += total
	} else {
		r.result.UsageComplete = false
	}
	request := data.RequestJSON
	if len(request) == 0 {
		// Request 仅包含可编码的协议字段；失败时也不能丢失业务错误。
		if encoded, encodeErr := json.Marshal(req); encodeErr == nil {
			request = encoded
		}
	}
	record := CallRecord{
		Kind:         CallKindModel,
		Name:         fmt.Sprintf("第 %d 次模型请求", r.result.ModelCalls),
		Request:      string(request),
		Response:     string(data.Raw),
		ElapsedMS:    time.Since(started).Milliseconds(),
		FinishReason: data.FinishReason,
		Usage:        &data.Usage,
	}
	if err != nil {
		record.Error = "模型调用未完成"
	}
	r.result.Calls = append(r.result.Calls, record)
	return response, err
}

// callTool 执行工具，记录调用次数、结果和耗时，再把结果整理成模型能继续读取的消息。
func (r *agentRun) callTool(ctx context.Context, tools Tools, req ToolCallReq) (message *Message, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	started := time.Now()
	var output *ToolCallRsp
	r.result.ToolCalls++
	// ToolCallReq 只包含字符串和简单结构体，可以直接编码。
	request, _ := json.Marshal(req)
	output, err = tools.Dispatch(ctx, req)
	if output == nil && err == nil {
		err = fmt.Errorf("工具未返回执行结果: %w", ErrInvalidResponse)
	}
	// 完整结果用于后续模型消息和执行记录，展示副本由应用层处理。
	response, encodeErr := json.Marshal(output)
	if encodeErr != nil && err == nil {
		err = fmt.Errorf("记录工具结果: %w", encodeErr)
	}
	record := CallRecord{
		Kind:      CallKindTool,
		Name:      req.Function.Name,
		Request:   string(request),
		Response:  string(response),
		ElapsedMS: time.Since(started).Milliseconds(),
	}
	if err != nil {
		record.Error = "工具执行未完成"
	} else if !output.OK {
		record.Error = output.Error
	}
	r.result.Calls = append(r.result.Calls, record)
	if err != nil {
		return nil, err
	}
	return &Message{
		Role:       RoleTool,
		ToolCallID: req.ID,
		Content:    string(response),
	}, nil
}

// end 根据最终错误更新本次业务运行的完成状态和耗时。
// 调用方通过 defer 闭包传入最终 err，使成功返回和提前失败都会执行收尾。
func (r *agentRun) end(err error) {
	// 以整个业务流程的耗时和最终错误确定运行状态；失败时保留已有回答、步骤和用量。
	r.result.ElapsedMS = time.Since(r.started).Milliseconds()
	r.result.Completed = err == nil
}
