package agent

import (
	"context"
	"fmt"
	"strings"
)

const (
	AgentQuestion = "每盒有 17 支笔，买 23 盒，一共多少支？" // 工具调用示例的默认乘法问题。
	// AgentPrompt 约束模型通过工具计算乘法，并在工具失败时如实回答。
	AgentPrompt           = "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
	DemoStructured        = "structured"     // 提取并检查商品与数量。
	DemoMultipleTools     = "multiple-tools" // 先查询商品，再计算总价。
	DemoReliability       = "reliability"    // 演示调用次数限制。
	DemoRetrieval         = "retrieval"      // 检索内置资料后回答。
	DemoWorkflow          = "workflow"       // 按固定的三步流程整理需求。
	DemoDelivery          = "delivery"       // 综合商品资料、价格与计算。
	DemoStream            = "stream"         // 实时生成文本。
	DemoChatQuestion      = "用一句话解释什么是递归。"   // 普通聊天案例在未填写问题时使用的示例。
	DemoChatPrompt        = "你是一个简洁的中文助手。"   // 普通聊天、对比及流式案例使用的提示词。
	DemoMaxQuantity       = 100              // 商品数量上限。
	DemoProductName       = "笔记本"            // 内置样例商品。
	DemoProductPrice      = 23               // 样例商品单价，单位为元。
	ToolNameLookupProduct = "lookup_product" // 本地商品资料查询工具。
)

// DemoOrder 是模型提取后通过 Go 校验的商品信息；不会真的下单。
type DemoOrder struct {
	Product  string `json:"product"`  // 商品名称，缺少信息时为空。
	Quantity int    `json:"quantity"` // 商品数量，缺少时为零；有效范围为 1～100。
}

// DemoStructuredResult 保留模型原回答及校验后的字段。
type DemoStructuredResult struct {
	Execution *ChatRsp  // 模型调用和校验过程；结果存在时非 nil。
	Order     DemoOrder // 只有校验通过后才填写。
}

// DemoStructured 要求模型输出 JSON，再由 Go 拒绝缺字段、未知字段和不合理的数量。
func (c *Client) DemoStructured(ctx context.Context, question string) (result *DemoStructuredResult, err error) {
	if question == "" {
		question = "帮我买三本笔记本。"
	}
	result = &DemoStructuredResult{}
	result.Execution = NewChatRsp(DemoStructured)
	prompt := `只从用户文字提取商品和数量，输出 JSON，例如 {"product":"笔记本","quantity":3}。只允许 product、quantity 两个字段。缺少商品用空字符串，缺少数量用 0，不能猜测，不执行购买。`
	run, err := c.prepareRun(result.Execution, RunInput{
		Questions: []string{question},
		Prompt:    prompt,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		run.end(err)
	}()
	response, err := run.callModel(ctx, Request{
		Messages: chatMessages(prompt, question),
		ResponseFormat: &ResponseFormat{
			Type: "json_object",
		},
	})
	if err != nil {
		return result, err
	}
	result.Execution.Answer = response.Message.Content
	if response.FinishReason != FinishReasonStop {
		return result, ErrInvalidResponse
	}
	var order DemoOrder
	if decodeDemoObject(response.Message.Content, &order) != nil || strings.TrimSpace(order.Product) == "" || order.Quantity < 1 || order.Quantity > DemoMaxQuantity {
		return result, fmt.Errorf("商品或数量不符合约定: %w", ErrInvalidResponse)
	}
	result.Order = order
	return result, nil
}
