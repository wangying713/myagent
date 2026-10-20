package agent

import (
	"context"
	"fmt"
)

// ChatClient 由模型的使用方定义，只要求当前业务需要的非流式能力。
type ChatClient interface {
	// Chat 执行一次模型请求，须响应 ctx 取消。
	// 失败时可同时返回用于记录的响应数据。
	Chat(ctx context.Context, req Request) (Response, error)
}

// ChatReq 保存对话输入，模型客户端和默认预算由 Client 提供。
type ChatReq struct {
	Operation  string // 本次业务标识，用于区分执行结果。
	Tools      Tools  // 模型可以调用的工具；nil 或没有工具定义时禁止调用。
	ToolChoice string // auto 允许模型选择工具，none 禁止；空值根据是否提供工具决定。
	Question   string // 调用方提供的用户问题，不能为空或只有空白。
	Prompt     string // 调用方提供的系统提示词；为空时不添加系统消息。
	Limits     Limits // 全零时使用客户端默认预算；单次覆盖时三个值都必须大于 0。
}

// ChatRsp 保存本次对话的回答、调用记录和统计，供各业务案例共用。
// HTTP 接口按自己的协议组装响应，不直接序列化此结构体。
type ChatRsp struct {
	// 执行结果。
	Operation string // 本次业务操作标识。
	Completed bool   // 本次业务是否成功完成。
	Answer    string // 模型返回的回答，失败时保留已有内容；对外展示由应用层处理。

	// 运行统计。
	ElapsedMS     int64 // 本次运行耗时，单位为毫秒。
	ModelCalls    int   // 已发起的模型调用次数。
	ToolCalls     int   // 已发起的工具调用次数。
	KnownTokens   int   // 可确认的累计 token 用量。
	UsageComplete bool  // 所有模型调用的用量是否均已确认并累计。

	// 调用记录。
	Calls []CallRecord // 按执行顺序保存的模型与工具调用记录。

	Input RunInput // 本次执行实际使用的问题、提示词、工具及预算。
}

// NewChatRsp 创建本次运行的初始结果。
func NewChatRsp(operation string) ChatRsp {
	return ChatRsp{
		Operation: operation,
		// 尚未调用模型时没有缺失用量；后续无法确认或累计用量时改为 false。
		UsageComplete: true,
		// 使用非 nil 空切片，使尚无调用记录的结果在 HTTP JSON 中表现为 []。
		Calls: []CallRecord{},
	}
}

const (
	CallKindModel = "model" // 模型调用记录。
	CallKindTool  = "tool"  // 工具调用记录。
)

// CallRecord 是一次模型或工具调用的记录。
type CallRecord struct {
	Kind         string // 调用类型，区分模型调用与工具调用。
	Name         string // 模型调用说明或工具名称。
	Request      string // 实际请求正文，对外展示由应用层处理。
	Response     string // 实际响应正文，对外展示由应用层处理。
	ElapsedMS    int64  // 本次调用耗时，单位为毫秒。
	FinishReason string // 模型结束原因，工具调用时为空。
	Usage        *Usage // 模型返回的 token 用量，工具调用时为 nil。
	Error        string // 执行失败或工具拒绝的公开说明。
}

// Chat 统一处理普通问答和工具对话，并记录执行过程与用量。
// 不提供工具或指定 none 时只接受直接回答；允许工具时按预算继续对话。
func (c *Client) Chat(ctx context.Context, req ChatReq) (rsp ChatRsp, err error) {
	if req.Operation == "" {
		req.Operation = OperationChat
	}
	rsp = NewChatRsp(req.Operation)
	definitions, err := c.prepareChat(&req)
	if err != nil {
		return rsp, err
	}
	limits := req.Limits
	// 只给当前调用设置预算，客户端本身保持不变，可以继续供其他请求使用。
	local := *c
	local.limits = limits
	run, err := local.prepareRun(&rsp, RunInput{
		Questions:  []string{req.Question},
		Prompt:     req.Prompt,
		Tools:      definitions,
		ToolChoice: req.ToolChoice,
	})
	if err != nil {
		return rsp, err
	}
	defer func() {
		run.end(err)
	}()
	messages := chatMessages(req.Prompt, req.Question)
	// 单独扣减预算，避免累计用量过大、无法相加时漏掉下一次调用的限制。
	remainingTokens := limits.Tokens
	seenToolCallIDs := make(map[string]bool)
	for round := 0; round < limits.ModelCalls; round++ {
		if err := ctx.Err(); err != nil {
			return rsp, err
		}
		if remainingTokens == 0 {
			return rsp, ErrBudgetExceeded
		}
		modelRsp, err := run.callModel(ctx, Request{
			Messages:   messages,
			Tools:      definitions,
			ToolChoice: req.ToolChoice,
		})
		if err != nil {
			return rsp, fmt.Errorf("第 %d 轮模型调用: %w", round+1, err)
		}
		if used, known := modelRsp.Usage.Total(); known {
			// 这次可以用超，但剩余额度归零后就不能再调用模型。
			remainingTokens -= min(used, remainingTokens)
		}
		messages = append(messages, modelRsp.Message)
		if modelRsp.FinishReason == FinishReasonStop {
			rsp.Answer = modelRsp.Message.Content
			return rsp, nil
		}
		// 工具被禁用时，即使模型错误地提出调用，也不能执行或继续对话。
		if req.ToolChoice == ToolChoiceNone {
			return rsp, ErrInvalidResponse
		}
		toolCalls := modelRsp.Message.ToolCalls
		if len(toolCalls) > limits.ToolCalls-rsp.ToolCalls {
			return rsp, ErrBudgetExceeded
		}
		if err := validateToolCalls(toolCalls, seenToolCallIDs); err != nil {
			return rsp, err
		}
		for _, toolCall := range toolCalls {
			message, err := run.callTool(ctx, req.Tools, toolCall)
			if err != nil {
				return rsp, err
			}
			messages = append(messages, message)
		}
	}
	return rsp, ErrBudgetExceeded
}

// prepareChat 确定本次能用哪些工具、是否允许调用，以及使用多少预算。
// req 是 Chat 收到的请求副本，补齐默认值不会影响调用方或其他请求。
func (c *Client) prepareChat(req *ChatReq) ([]ToolDefinition, error) {
	if req.ToolChoice != "" && req.ToolChoice != ToolChoiceAuto && req.ToolChoice != ToolChoiceNone {
		return nil, ErrInput
	}
	var definitions []ToolDefinition
	if req.Tools != nil {
		definitions = req.Tools.Definitions()
	}
	if len(definitions) == 0 {
		req.ToolChoice = ToolChoiceNone
	} else if req.ToolChoice == "" {
		req.ToolChoice = ToolChoiceAuto
	}
	if req.Limits == (Limits{}) {
		req.Limits = c.limits
	}
	if req.Limits.ModelCalls <= 0 || req.Limits.ToolCalls <= 0 || req.Limits.Tokens <= 0 {
		return nil, fmt.Errorf("运行预算必须为正数")
	}
	return definitions, nil
}

// chatMessages 组装本次问答的消息，未提供提示词时只放用户问题。
func chatMessages(prompt, question string) []Message {
	messages := []Message{}
	if prompt != "" {
		messages = append(messages, Message{
			Role:    RoleSystem,
			Content: prompt,
		})
	}
	return append(messages, Message{
		Role:    RoleUser,
		Content: question,
	})
}

// validateToolCalls 先检查这一轮所有工具的 ID，再开始执行，避免执行到一半才发现 ID 有问题。
func validateToolCalls(toolCalls []ToolCall, seenToolCallIDs map[string]bool) error {
	for _, toolCall := range toolCalls {
		if toolCall.ID == "" || seenToolCallIDs[toolCall.ID] {
			return ErrInvalidResponse
		}
		seenToolCallIDs[toolCall.ID] = true
	}
	return nil
}
