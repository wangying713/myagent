package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
)

// ToolNameMultiply 是乘法工具在声明和调用校验中共用的协议名称。
const ToolNameMultiply = "multiply"

// MultiplyTools 是当前唯一可用的工具集；不会执行模型生成的函数或文本。
type MultiplyTools struct{}

// multiplyArgs 同时定义乘法工具的参数类型和执行时使用的数据。
type multiplyArgs struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// multiplyToolDescription 在包初始化时从同一次构建嵌入的源码读取，此后只读。
var multiplyToolDescription = mustToolDescription[MultiplyTools](multiplyToolSource, ToolNameMultiply)

// Definitions 返回乘法工具及其 JSON 参数约束，每次调用独立构建定义。
func (MultiplyTools) Definitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: ToolTypeFunction,
			Function: FunctionDefinition{
				Name:        ToolNameMultiply,
				Description: multiplyToolDescription,
				Parameters:  mustToolParameters[multiplyArgs](),
			},
		},
	}
}

// Dispatch 只执行已注册的乘法工具，未知工具或无效参数以 OK=false 的结果返回。
// ctx 已取消时返回上下文错误，不执行计算。
func (t MultiplyTools) Dispatch(ctx context.Context, call ToolCall) (ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return ToolResult{}, err
	}
	if call.Type != ToolTypeFunction || call.Function.Name != ToolNameMultiply {
		return ToolResult{Error: "unknown_tool"}, nil
	}
	if !json.Valid([]byte(call.Function.Arguments)) {
		return ToolResult{Error: "invalid_json"}, nil
	}
	args, ok := multiplyArguments([]byte(call.Function.Arguments))
	if !ok {
		return ToolResult{Error: "invalid_arguments"}, nil
	}
	return t.multiply(ctx, args)
}

// multiply 计算两个数的乘积。涉及乘法时使用。
func (MultiplyTools) multiply(ctx context.Context, args multiplyArgs) (ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return ToolResult{}, err
	}
	value := args.A * args.B
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ToolResult{Error: "tool_execution_failed"}, nil
	}
	return ToolResult{
		OK:     true,
		Result: value,
	}, nil
}

// multiplyArguments 使用 JSON token 精确检查 a、b 和重复字段，只接受有限数值。
func multiplyArguments(raw []byte) (multiplyArgs, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return multiplyArgs{}, false
	}
	values := make(map[string]float64, 2)
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return multiplyArgs{}, false
		}
		name, ok := key.(string)
		if !ok || (name != "a" && name != "b") {
			return multiplyArgs{}, false
		}
		if _, exists := values[name]; exists {
			return multiplyArgs{}, false
		}
		var value any
		if d.Decode(&value) != nil {
			return multiplyArgs{}, false
		}
		number, ok := value.(json.Number)
		if !ok {
			return multiplyArgs{}, false
		}
		parsed, err := number.Float64()
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return multiplyArgs{}, false
		}
		values[name] = parsed
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(values) != 2 {
		return multiplyArgs{}, false
	}
	return multiplyArgs{
		A: values["a"],
		B: values["b"],
	}, true
}
