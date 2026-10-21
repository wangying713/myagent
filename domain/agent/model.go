// Package agent 定义聊天协议、受控工具和有限 Agent 循环，不读取配置或执行网络请求。
package agent

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

// ErrInvalidResponse 表示模型消息、结束原因或工具调用协议不符合要求。
var ErrInvalidResponse = errors.New("模型响应不完整或协议无效")

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

// Message 保留模型原始 assistant 消息，回传历史时不丢失扩展字段或 null。
type Message struct {
	Role       string        `json:"role"`                   // 消息角色，使用 Role 系列常量。
	Content    string        `json:"content"`                // 消息正文文本。
	ToolCalls  []ToolCallReq `json:"tool_calls,omitempty"`   // 模型请求执行的工具调用列表。
	ToolCallID string        `json:"tool_call_id,omitempty"` // 工具结果关联的调用 ID。
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

// Request 保存模型请求协议；实际模型名和生成参数由注入的客户端适配。
type Request struct {
	Model          string           `json:"model"`                     // 目标模型名称，由模型客户端适配。
	Messages       []Message        `json:"messages"`                  // 按对话顺序排列的消息历史。
	Tools          []ToolDefinition `json:"tools,omitempty"`           // 当前允许模型选择的工具定义。
	ToolChoice     string           `json:"tool_choice,omitempty"`     // 工具选择策略，auto 允许自行选择，none 禁止调用。
	ResponseFormat *ResponseFormat  `json:"response_format,omitempty"` // 非 nil 时要求模型输出 JSON 对象。
	Thinking       struct {
		Type string `json:"type"` // 思考模式类型，由模型客户端适配。
	} `json:"thinking"` // 模型思考模式配置。
	MaxTokens int  `json:"max_tokens"` // 最大输出 token 数，由模型客户端适配。
	Stream    bool `json:"stream"`     // 是否请求流式响应；流式使用独立的 Stream 方法。
}

// ResponseFormat 指定模型回答的编码形式，JSON 内容仍须通过业务校验。
type ResponseFormat struct {
	Type string `json:"type"` // json_object 要求回答为 JSON 对象。
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

// Validate 校验 assistant 身份、结束原因、消息内容及非负用量；nil 响应无效。
// 工具调用 ID 和参数由执行循环及注册工具继续校验。
func (r *Response) Validate() error {
	if r == nil || r.Message.Role != RoleAssistant {
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
