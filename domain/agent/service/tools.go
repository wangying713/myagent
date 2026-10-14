// Package service 提供受控工具与有限 Agent 循环。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"sakelei/ai/domain/agent/entity"
)

// ErrInvalidArguments 表示参数不符合工具约定，业务函数不应继续执行。
var ErrInvalidArguments = errors.New("工具参数无效")

// Tool 包含模型可见的说明和程序控制的执行函数。
type Tool struct {
	Definition entity.ToolDefinition
	Execute    func(context.Context, json.RawMessage) (any, error)
}

// Registry 只分发调用方显式注册的工具。创建后不修改，可并发读取。
type Registry struct {
	tools       map[string]Tool
	definitions []entity.ToolDefinition
}

// NewRegistry 拒绝重名、无执行函数或无效参数描述的工具。
func NewRegistry(tools []Tool) (*Registry, error) {
	r := &Registry{tools: make(map[string]Tool)}
	for _, tool := range tools {
		name := tool.Definition.Function.Name
		if name == "" || tool.Definition.Type != "function" || tool.Execute == nil || !json.Valid(tool.Definition.Function.Parameters) {
			return nil, errors.New("工具定义不完整")
		}
		if _, exists := r.tools[name]; exists {
			return nil, fmt.Errorf("工具 %q 重复注册", name)
		}
		// 复制参数模式，避免调用者修改原始字节影响后续请求。
		tool.Definition.Function.Parameters = bytes.Clone(tool.Definition.Function.Parameters)
		r.tools[name] = tool
		r.definitions = append(r.definitions, tool.Definition)
	}
	return r, nil
}

// Definitions 返回独立的工具描述副本。
func (r *Registry) Definitions() []entity.ToolDefinition {
	definitions := append([]entity.ToolDefinition(nil), r.definitions...)
	for i := range definitions {
		definitions[i].Function.Parameters = bytes.Clone(definitions[i].Function.Parameters)
	}
	return definitions
}

// Dispatch 返回可回填的工具失败；调用方取消则返回 error 终止运行。
func (r *Registry) Dispatch(ctx context.Context, call entity.ToolCall) (entity.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return entity.ToolResult{}, err
	}
	if call.Type != "function" {
		return entity.ToolResult{Error: "invalid_tool_call"}, nil
	}
	tool, exists := r.tools[call.Function.Name]
	if !exists {
		return entity.ToolResult{Error: "unknown_tool"}, nil
	}
	arguments := json.RawMessage(call.Function.Arguments)
	if !json.Valid(arguments) {
		return entity.ToolResult{Error: "invalid_json"}, nil
	}
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return entity.ToolResult{Error: "invalid_arguments"}, nil
	}
	value, err := tool.Execute(ctx, arguments)
	if err != nil {
		if ctx.Err() != nil {
			return entity.ToolResult{}, ctx.Err()
		}
		code := "tool_execution_failed"
		if errors.Is(err, ErrInvalidArguments) {
			code = "invalid_arguments"
		}
		return entity.ToolResult{Error: code}, nil
	}
	if _, err := json.Marshal(value); err != nil {
		return entity.ToolResult{Error: "tool_execution_failed"}, nil
	}
	return entity.ToolResult{OK: true, Result: value}, nil
}

// MultiplyTool 创建本地乘法工具，拒绝缺失、多余、错误类型和非有限数值。
func MultiplyTool() Tool {
	return Tool{
		Definition: entity.ToolDefinition{
			Type: "function",
			Function: entity.FunctionDefinition{
				Name: "multiply", Description: "计算两个数的乘积。涉及乘法时使用。",
				Parameters: json.RawMessage(`{"type":"object","properties":{"a":{"type":"number","description":"第一个数"},"b":{"type":"number","description":"第二个数"}},"required":["a","b"],"additionalProperties":false}`),
			},
		},
		Execute: func(ctx context.Context, raw json.RawMessage) (any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var args struct {
				A *float64 `json:"a"`
				B *float64 `json:"b"`
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) != nil || len(fields) != 2 || fields["a"] == nil || fields["b"] == nil {
				return nil, ErrInvalidArguments
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&args); err != nil || args.A == nil || args.B == nil {
				return nil, ErrInvalidArguments
			}
			if decoder.Decode(new(any)) != io.EOF {
				return nil, ErrInvalidArguments
			}
			value := *args.A * *args.B
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, ErrInvalidArguments
			}
			return value, nil
		},
	}
}
