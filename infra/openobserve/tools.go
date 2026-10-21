package openobserve

import (
	"context"

	"saklay/ai/domain/agent"
)

// Tools 在真实工具执行边界记录观测，不改变定义、参数或返回结果。
type Tools struct {
	Tools      agent.Tools // 实际执行的工具集合，由应用层注入。
	ObserveCli *Client     // 记录工具调用的观测组件，由应用层管理生命周期。
}

// Definitions 返回原工具集合的定义。
func (t Tools) Definitions() []agent.ToolDefinition {
	return t.Tools.Definitions()
}

// Dispatch 沿用请求上下文执行工具，成功、失败和取消均结束对应的观测。
func (t Tools) Dispatch(ctx context.Context, req agent.ToolCall) (result agent.ToolResult, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	ctx, observation := t.ObserveCli.StartTool(ctx, req)
	defer func() {
		observation.End(result, err)
	}()
	return t.Tools.Dispatch(ctx, req)
}
