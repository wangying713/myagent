package agent

import (
	"context"
	"time"
)

// agentRun 保存这一次 Agent 运行的模型、结果和开始时间，负责结束时收尾。
// 每次运行单独创建，调用结束后不再复用。
type agentRun struct {
	observer Observer        // 隐藏最终回答中的敏感内容。
	result   *Result         // 保存这次运行的回答、步骤和统计。
	modelCli *recordingModel // 每次请求模型时，记录调用次数、用量和步骤。
	run      RunObservation  // 这次运行的链路记录，结束时一并收尾。
	started  time.Time       // 记下开始时间，用来计算总耗时。
}

// prepareRun 检查问题，创建运行记录，并准备能记录调用次数和用量的模型客户端。
// 模型请求由调用方随后发起，执行结束后由 end 收尾。
func (c *Client) prepareRun(ctx context.Context, result *Result, req RunDetails) (context.Context, *agentRun, error) {
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
	if modelCli, ok := c.modelCli.(ModelInfoProvider); ok {
		info := modelCli.ModelInfo()
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
		run:      run,
		started:  time.Now(),
		// 给模型加上记账功能，每次调用都把用量和执行结果写进同一份 result。
		modelCli: &recordingModel{
			modelCli: c.modelCli,
			observer: c.observer,
			result:   result,
			limits:   c.limits,
		},
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
