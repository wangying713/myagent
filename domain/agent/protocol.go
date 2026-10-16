// Package agent 定义聊天协议、受控工具和有限 Agent 循环，不读取配置或执行网络请求。
package agent

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

var (
	// ErrInvalidResponse 表示模型消息、结束原因或工具调用协议不符合要求。
	ErrInvalidResponse = errors.New("模型响应不完整或协议无效")
	// ErrBudgetExceeded 表示继续执行将超出模型次数、工具次数或已知用量预算。
	ErrBudgetExceeded = errors.New("达到运行预算，任务未完成")
)

// Message 保留模型原始 assistant 消息，回传历史时不丢失扩展字段或 null。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 保存工具名称和 JSON 字符串形式的参数，参数由注册工具校验。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition 描述提供给模型选择的已注册工具及其函数协议。
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 描述工具能力，Parameters 使用 JSON Schema 约束参数。
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Request 保存模型请求协议；实际模型名和生成参数由注入的客户端适配。
type Request struct {
	Model      string           `json:"model"`
	Messages   []Message        `json:"messages"`
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice string           `json:"tool_choice,omitempty"`
	Thinking   struct {
		Type string `json:"type"`
	} `json:"thinking"`
	MaxTokens int  `json:"max_tokens"`
	Stream    bool `json:"stream"`
}

// Usage 用指针区分零用量和未提供的用量。
type Usage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

// Total 优先返回已知总用量，没有总数时尝试合计输入和输出用量。
// 含负值、缺少必要用量或合计溢出时返回 false，不能将其视为已知零用量。
func (u Usage) Total() (int, bool) {
	for _, value := range []*int{u.PromptTokens, u.CompletionTokens, u.TotalTokens} {
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
	Message      Message
	FinishReason string
	Usage        Usage
	RequestJSON  []byte
	Raw          []byte
}

// Validate 校验 assistant 身份、结束原因、消息内容及非负用量。
// 工具调用 ID 和参数由执行循环及注册工具继续校验。
func (r Response) Validate() error {
	if r.Message.Role != "assistant" {
		return ErrInvalidResponse
	}
	for _, value := range []*int{r.Usage.PromptTokens, r.Usage.CompletionTokens, r.Usage.TotalTokens} {
		if value != nil && *value < 0 {
			return ErrInvalidResponse
		}
	}
	switch r.FinishReason {
	case "stop":
		if strings.TrimSpace(r.Message.Content) == "" || len(r.Message.ToolCalls) != 0 {
			return ErrInvalidResponse
		}
	case "tool_calls":
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
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Limits 限制单次运行的模型次数、工具次数和累计已知 token 用量，各值须为正数。
// token 预算在调用之间检查，单次模型调用可能超过剩余预算。
type Limits struct {
	ModelCalls int
	ToolCalls  int
	Tokens     int
}

// DefaultLimits 返回三个学习流程使用的默认运行预算。
func DefaultLimits() Limits { return Limits{ModelCalls: 5, ToolCalls: 8, Tokens: 8000} }
