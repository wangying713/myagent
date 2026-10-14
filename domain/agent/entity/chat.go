// Package entity 定义模型消息、工具协议和运行结果，不负责网络请求。
package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Message 是一条聊天消息。Raw 保存模型原始消息，转发时保留扩展字段和 null。
// 模型返回的消息应视为不可变；需要编辑时，应创建新的 Message。
type Message struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

// MarshalJSON 转发模型原始消息，否则编码程序构造的消息。
func (m Message) MarshalJSON() ([]byte, error) {
	if len(m.Raw) != 0 {
		if !json.Valid(m.Raw) {
			return nil, errors.New("消息原文不是有效 JSON")
		}
		return m.Raw, nil
	}
	type plain Message
	return json.Marshal(plain(m))
}

// ToolCall 是模型提出的函数调用；Arguments 是 JSON 字符串，不是可执行代码。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 保存工具名称和模型生成的参数。
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition 向模型描述一个允许使用的工具。
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 包含工具说明和 JSON 参数模式。
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Thinking 控制模型的思考模式，前三课默认关闭。
type Thinking struct {
	Type string `json:"type"`
}

// ChatRequest 是第一阶段非流式请求。
type ChatRequest struct {
	Model      string           `json:"model"`
	Messages   []Message        `json:"messages"`
	Tools      []ToolDefinition `json:"tools,omitempty"`
	ToolChoice string           `json:"tool_choice,omitempty"`
	MaxTokens  int              `json:"max_tokens"`
	Thinking   *Thinking        `json:"thinking,omitempty"`
	Stream     bool             `json:"stream"`
}

// Usage 保留用量是否缺失，避免把未知用量表示为零。
type Usage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

// Total 返回服务端用量；未提供总量时，尝试从输入和输出相加。
// 缺失、负数或相加溢出的用量视为未知。
func (u Usage) Total() (int, bool) {
	for _, value := range []*int{u.PromptTokens, u.CompletionTokens, u.TotalTokens} {
		if value != nil && *value < 0 {
			return 0, false
		}
	}
	if u.TotalTokens != nil {
		return *u.TotalTokens, true
	}
	if u.PromptTokens != nil && u.CompletionTokens != nil {
		total := *u.PromptTokens + *u.CompletionTokens
		if total >= *u.PromptTokens {
			return total, true
		}
	}
	return 0, false
}

// ChatResponse 是解析后的模型结果；Raw 保存完整响应以供观测。
type ChatResponse struct {
	ID           string
	Model        string
	Message      Message
	FinishReason string
	Usage        Usage
	Raw          json.RawMessage
}

// Validate 检查回答是否完整，拒绝把截断或无效响应当成成功。
func (r ChatResponse) Validate() error {
	if r.Message.Role != "assistant" {
		return errors.New("响应缺少 assistant 消息")
	}
	for _, value := range []*int{r.Usage.PromptTokens, r.Usage.CompletionTokens, r.Usage.TotalTokens} {
		if value != nil && *value < 0 {
			return errors.New("token 用量不能为负数")
		}
	}
	switch r.FinishReason {
	case "stop":
		if strings.TrimSpace(r.Message.Content) == "" || len(r.Message.ToolCalls) != 0 {
			return errors.New("模型未返回完整文本回答")
		}
	case "tool_calls":
		if len(r.Message.ToolCalls) == 0 {
			return errors.New("模型声明工具调用但未提供调用列表")
		}
	default:
		return fmt.Errorf("模型未正常完成，结束原因为 %q", r.FinishReason)
	}
	return nil
}

// ToolResult 是回填给模型的工具结果，失败时只暴露稳定错误码。
type ToolResult struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// RunResult 保存最终回答、已知用量和实际工具执行证据。
type RunResult struct {
	Answer         string
	ModelCalls     int
	ToolCalls      int
	KnownTokens    int
	UsageComplete  bool
	ToolExecutions []ToolExecution
}

// AddUsage 累加一次请求的有效用量；调用方应在首轮前将 UsageComplete 设为 true。
// 缺失或非法的用量只标记不完整，不清除此前已知的统计。
func (r *RunResult) AddUsage(usage Usage) {
	if total, known := usage.Total(); known {
		r.KnownTokens += total
	} else {
		r.UsageComplete = false
	}
}

// ToolExecution 将工具协议请求与实际结果关联起来。
type ToolExecution struct {
	Call   ToolCall
	Result ToolResult
}
