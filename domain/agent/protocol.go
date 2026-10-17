// Package agent 定义聊天协议、受控工具和有限 Agent 循环，不读取配置或执行网络请求。
package agent

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

var (
	ErrInvalidResponse = errors.New("模型响应不完整或协议无效") // 模型消息、结束原因或工具调用协议不符合要求。
	ErrBudgetExceeded  = errors.New("达到运行预算，任务未完成") // 继续执行将超出模型次数、工具次数或已知用量预算。
)

const (
	RoleSystem    = "system"    // 系统提示消息。
	RoleUser      = "user"      // 用户输入消息。
	RoleAssistant = "assistant" // 模型返回的消息。
	RoleTool      = "tool"      // 回填给模型的工具执行结果。
)

const (
	FinishReasonStop      = "stop"       // 模型已完成回答。
	FinishReasonToolCalls = "tool_calls" // 模型请求执行工具。
)

const (
	ToolTypeFunction = "function" // 当前支持的函数工具协议类型。
	ToolChoiceAuto   = "auto"     // 允许模型在已注册工具中自行选择是否调用。
)

// Message 保留模型原始 assistant 消息，回传历史时不丢失扩展字段或 null。
type Message struct {
	Role       string     `json:"role"`                   // 消息角色，使用 Role 系列常量。
	Content    string     `json:"content"`                // 消息正文文本。
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 模型请求执行的工具调用列表。
	ToolCallID string     `json:"tool_call_id,omitempty"` // 工具结果关联的调用 ID。
	// Raw 保存完整的 assistant JSON；非空时优先用于编码。
	Raw json.RawMessage `json:"-"`
}

// MarshalJSON 优先回传 Raw，以保留扩展字段和 null；Raw 无效时返回 ErrInvalidResponse。
func (m Message) MarshalJSON() ([]byte, error) {
	if len(m.Raw) != 0 {
		if !json.Valid(m.Raw) {
			return nil, ErrInvalidResponse
		}
		return m.Raw, nil
	}
	type message Message
	return json.Marshal(message(m))
}

// ToolCall 保存模型提出的工具调用，ID 用于关联后续工具结果。
type ToolCall struct {
	ID       string       `json:"id"`       // 本次工具调用的唯一标识，用于关联工具结果。
	Type     string       `json:"type"`     // 工具调用类型，当前为 function。
	Function FunctionCall `json:"function"` // 待执行的函数名称与参数。
}

// FunctionCall 保存工具名称和 JSON 字符串形式的参数，参数由注册工具校验。
type FunctionCall struct {
	Name      string `json:"name"`      // 模型请求调用的工具函数名称。
	Arguments string `json:"arguments"` // JSON 字符串形式的参数，由注册工具校验。
}

// ToolDefinition 描述提供给模型选择的已注册工具及其函数协议。
type ToolDefinition struct {
	Type     string             `json:"type"`     // 工具定义类型，当前为 function。
	Function FunctionDefinition `json:"function"` // 供模型选择的函数定义。
}

// FunctionDefinition 描述工具能力，Parameters 使用 JSON Schema 约束参数。
type FunctionDefinition struct {
	Name        string          `json:"name"`        // 已注册的工具函数名称。
	Description string          `json:"description"` // 供模型选择工具时阅读的能力说明。
	Parameters  json.RawMessage `json:"parameters"`  // 约束工具参数的 JSON Schema。
}

// Request 保存模型请求协议；实际模型名和生成参数由注入的客户端适配。
type Request struct {
	Model      string           `json:"model"`                 // 目标模型名称，由模型客户端适配。
	Messages   []Message        `json:"messages"`              // 按对话顺序排列的消息历史。
	Tools      []ToolDefinition `json:"tools,omitempty"`       // 当前允许模型选择的工具定义。
	ToolChoice string           `json:"tool_choice,omitempty"` // 工具选择策略，auto 允许模型自行决定是否调用。
	Thinking   struct {
		Type string `json:"type"` // 思考模式类型，由模型客户端适配。
	} `json:"thinking"` // 模型思考模式配置。
	MaxTokens int  `json:"max_tokens"` // 最大输出 token 数，由模型客户端适配。
	Stream    bool `json:"stream"`     // 是否请求流式响应，当前客户端固定为 false。
}

// Usage 用指针区分零用量和未提供的用量。
type Usage struct {
	PromptTokens     *int `json:"prompt_tokens"`     // 输入 token 数，nil 表示未提供。
	CompletionTokens *int `json:"completion_tokens"` // 输出 token 数，nil 表示未提供。
	TotalTokens      *int `json:"total_tokens"`      // 总 token 数，nil 表示未提供。
}

// Total 优先返回已知总用量，没有总数时尝试合计输入和输出用量。
// 含负值、缺少必要用量或合计溢出时返回 false，不能将其视为已知零用量。
func (u Usage) Total() (int, bool) {
	for _, value := range []*int{
		u.PromptTokens,
		u.CompletionTokens,
		u.TotalTokens,
	} {
		if value != nil && *value < 0 {
			return 0, false
		}
	}
	if u.TotalTokens != nil {
		return *u.TotalTokens, true
	}
	if u.PromptTokens != nil && u.CompletionTokens != nil && *u.PromptTokens <= math.MaxInt-*u.CompletionTokens {
		return *u.PromptTokens + *u.CompletionTokens, true
	}
	return 0, false
}

// Response 的原文仅供执行记录和观测使用，业务判断基于明确字段。
type Response struct {
	Message      Message // 模型返回的 assistant 消息。
	FinishReason string  // 模型结束原因，使用 FinishReason 系列常量。
	Usage        Usage   // 模型返回的 token 用量。
	RequestJSON  []byte  // 实际发送的请求 JSON，仅供记录与观测使用。
	Raw          []byte  // 收到的响应原文，仅供记录与观测使用。
}

// Validate 校验 assistant 身份、结束原因、消息内容及非负用量。
// 工具调用 ID 和参数由执行循环及注册工具继续校验。
func (r Response) Validate() error {
	if r.Message.Role != RoleAssistant {
		return ErrInvalidResponse
	}
	for _, value := range []*int{
		r.Usage.PromptTokens,
		r.Usage.CompletionTokens,
		r.Usage.TotalTokens,
	} {
		if value != nil && *value < 0 {
			return ErrInvalidResponse
		}
	}
	switch r.FinishReason {
	case FinishReasonStop:
		if strings.TrimSpace(r.Message.Content) == "" || len(r.Message.ToolCalls) != 0 {
			return ErrInvalidResponse
		}
	case FinishReasonToolCalls:
		if len(r.Message.ToolCalls) == 0 {
			return ErrInvalidResponse
		}
	default:
		return ErrInvalidResponse
	}
	return nil
}

// ToolResult 保存工具成功值或拒绝原因，Result 须能够编码为 JSON。
type ToolResult struct {
	OK     bool   `json:"ok"`               // 工具是否执行成功。
	Result any    `json:"result,omitempty"` // 成功结果，须能够编码为 JSON。
	Error  string `json:"error,omitempty"`  // 工具拒绝或失败的公开说明。
}

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
