package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
)

const (
	ToolTypeFunction = "function" // 当前支持的函数工具协议类型。
	ToolChoiceAuto   = "auto"     // 允许模型在已注册工具中自行选择是否调用。
	ToolChoiceNone   = "none"     // 禁止模型调用工具，只允许直接回答。
)

// ToolDefinition 是调用前发给模型的工具说明，供模型判断是否使用该工具。
type ToolDefinition struct {
	Type     string                 `json:"type"`     // 工具定义类型，当前为 function。
	Function ToolFunctionDefinition `json:"function"` // 供模型选择的函数定义。
}

// ToolFunctionDefinition 说明工具函数的名称、用途及参数格式，尚不包含实际调用参数。
type ToolFunctionDefinition struct {
	Name        string          `json:"name"`        // 已注册的工具函数名称。
	Description string          `json:"description"` // 供模型选择工具时阅读的能力说明。
	Parameters  json.RawMessage `json:"parameters"`  // 约束工具参数的 JSON Schema。
}

// ToolCallReq 是模型返回的单次工具调用请求，ID 用于关联后续执行结果。
type ToolCallReq struct {
	ID       string           `json:"id"`       // 本次工具调用的唯一标识，用于关联工具结果。
	Type     string           `json:"type"`     // 工具调用类型，当前为 function。
	Function ToolFunctionCall `json:"function"` // 待执行的函数名称与参数。
}

// ToolFunctionCall 保存本次请求的函数名称和实际参数，Arguments 是待工具校验的 JSON 字符串。
type ToolFunctionCall struct {
	Name      string `json:"name"`      // 模型请求调用的工具函数名称。
	Arguments string `json:"arguments"` // JSON 字符串形式的参数，由注册工具校验。
}

// ToolCallRsp 是工具执行后的结果，保存成功值或拒绝原因，Result 须能够编码为 JSON。
type ToolCallRsp struct {
	OK     bool   `json:"ok"`               // 工具是否执行成功。
	Result any    `json:"result,omitempty"` // 成功结果，须能够编码为 JSON。
	Error  string `json:"error,omitempty"`  // 工具拒绝或失败的公开说明。
}

// Tools 提供已注册的工具定义和分发能力，协议拒绝通过 ToolCallRsp 返回。
type Tools interface {
	// Definitions 返回供模型选择的工具说明及参数格式约束。
	Definitions() []ToolDefinition
	// Dispatch 返回工具结果；前置取消返回 nil 和错误，业务拒绝使用非 nil 的 OK=false 结果。
	Dispatch(ctx context.Context, req ToolCallReq) (*ToolCallRsp, error)
}

// ToolNameMultiply 是乘法工具在声明和调用校验中共用的协议名称。
const ToolNameMultiply = "multiply"

// MultiplyTools 只提供已注册的乘法工具，不执行模型生成的函数或文本。
type MultiplyTools struct{}

// multiplyArgs 同时定义乘法工具的参数类型和执行时使用的数据。
type multiplyArgs struct {
	A float64 `json:"a"` // 参与乘法的第一个有限数值。
	B float64 `json:"b"` // 参与乘法的第二个有限数值。
}

// multiplyToolDescription 在包初始化时从同一次构建嵌入的源码读取，此后只读。
var multiplyToolDescription = mustToolDescription[MultiplyTools](multiplyToolSource, ToolNameMultiply)

// Definitions 返回乘法工具及其 JSON 参数约束，每次调用独立构建定义。
func (MultiplyTools) Definitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: ToolTypeFunction,
			Function: ToolFunctionDefinition{
				Name:        ToolNameMultiply,
				Description: multiplyToolDescription,
				Parameters:  mustToolParameters[multiplyArgs](),
			},
		},
	}
}

// Dispatch 只执行已注册的乘法工具，未知工具或无效参数以 OK=false 的结果返回。
// ctx 已取消时返回上下文错误，不执行计算。
func (t MultiplyTools) Dispatch(ctx context.Context, req ToolCallReq) (*ToolCallRsp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Type != ToolTypeFunction || req.Function.Name != ToolNameMultiply {
		return &ToolCallRsp{
			Error: "unknown_tool",
		}, nil
	}
	if !json.Valid([]byte(req.Function.Arguments)) {
		return &ToolCallRsp{
			Error: "invalid_json",
		}, nil
	}
	args, ok := multiplyArguments([]byte(req.Function.Arguments))
	if !ok {
		return &ToolCallRsp{
			Error: "invalid_arguments",
		}, nil
	}
	return t.multiply(ctx, args)
}

// multiply 计算两个数的乘积。涉及乘法时使用。
func (MultiplyTools) multiply(ctx context.Context, req multiplyArgs) (*ToolCallRsp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value := req.A * req.B
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return &ToolCallRsp{
			Error: "tool_execution_failed",
		}, nil
	}
	return &ToolCallRsp{
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
