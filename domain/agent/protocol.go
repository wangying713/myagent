// Package agent 定义聊天协议、受控工具和有限 Agent 循环，不读取配置或执行网络请求。
package agent

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
)

var (
	ErrInvalidResponse = errors.New("模型响应不完整或协议无效")
	ErrBudgetExceeded  = errors.New("达到运行预算，任务未完成")
)

// Message 保留模型原始 assistant 消息，回传历史时不丢失扩展字段或 null。
type Message struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

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

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

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

type ToolResult struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Limits struct {
	ModelCalls int
	ToolCalls  int
	Tokens     int
}

func DefaultLimits() Limits { return Limits{ModelCalls: 5, ToolCalls: 8, Tokens: 8000} }
