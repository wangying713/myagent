package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
)

// MultiplyTools 是第三课唯一可用的工具集；不会执行模型生成的函数或文本。
type MultiplyTools struct{}

func (MultiplyTools) Definitions() []ToolDefinition {
	return []ToolDefinition{{Type: "function", Function: FunctionDefinition{
		Name: "multiply", Description: "计算两个数的乘积。涉及乘法时使用。",
		Parameters: json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"],"additionalProperties":false}`),
	}}}
}

func (MultiplyTools) Dispatch(ctx context.Context, call ToolCall) (ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return ToolResult{}, err
	}
	if call.Type != "function" || call.Function.Name != "multiply" {
		return ToolResult{Error: "unknown_tool"}, nil
	}
	if !json.Valid([]byte(call.Function.Arguments)) {
		return ToolResult{Error: "invalid_json"}, nil
	}
	a, b, ok := multiplyArguments([]byte(call.Function.Arguments))
	if !ok {
		return ToolResult{Error: "invalid_arguments"}, nil
	}
	value := a * b
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ToolResult{Error: "tool_execution_failed"}, nil
	}
	return ToolResult{OK: true, Result: value}, nil
}

// 使用 token 检查键名和重复字段，避免结构体解码接受大小写别名或重复参数。
func multiplyArguments(raw []byte) (float64, float64, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return 0, 0, false
	}
	values := make(map[string]float64, 2)
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return 0, 0, false
		}
		name, ok := key.(string)
		if !ok || (name != "a" && name != "b") {
			return 0, 0, false
		}
		if _, exists := values[name]; exists {
			return 0, 0, false
		}
		var value any
		if d.Decode(&value) != nil {
			return 0, 0, false
		}
		number, ok := value.(json.Number)
		if !ok {
			return 0, 0, false
		}
		parsed, err := number.Float64()
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return 0, 0, false
		}
		values[name] = parsed
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(values) != 2 {
		return 0, 0, false
	}
	return values["a"], values["b"], true
}
