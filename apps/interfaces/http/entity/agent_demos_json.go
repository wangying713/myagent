package entity

// DemosOverviewReq 是 Agent 概览演示的请求。
type DemosOverviewReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosChatReq 是普通聊天演示的请求。
type DemosChatReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosMessagesReq 是消息历史演示的请求。
type DemosMessagesReq struct {
	Questions []string `json:"questions"` // 两轮问题；nil 使用默认示例，显式空数组交由业务拒绝。
}

// DemosToolsReq 是工具调用与循环演示的请求。
type DemosToolsReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosStructuredReq 是结构化输出演示的请求。
type DemosStructuredReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosMultipleToolsReq 是多个工具协作演示的请求。
type DemosMultipleToolsReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosStreamReq 是流式回答演示的请求。
type DemosStreamReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosReliabilityReq 是执行边界演示的请求。
type DemosReliabilityReq struct {
	Question   string `json:"question"`    // 用户问题，省略或空字符串使用本章默认示例。
	ModelCalls *int   `json:"model_calls"` // 允许的模型调用次数；nil 使用本章默认值。
}

// DemosRetrievalReq 是检索与记忆演示的请求。
type DemosRetrievalReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosWorkflowReq 是工作流演示的请求。
type DemosWorkflowReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosObservationReq 是观测演示的请求。
type DemosObservationReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosEvaluationReq 是评估与回归演示的请求。
type DemosEvaluationReq struct{}

// DemosDeliveryReq 是综合练习演示的请求。
type DemosDeliveryReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemoSource 是提供给页面核对的资料片段。
type DemoSource struct {
	ID    string `json:"id"`    // 本地资料编号。
	Title string `json:"title"` // 资料标题。
	Text  string `json:"text"`  // 实际使用的资料正文。
}

// DemoStage 是固定工作流的已完成步骤。
type DemoStage struct {
	Name    string `json:"name"`    // 步骤名称。
	Content string `json:"content"` // 本步骤实际输出。
}

// DemoCase 保存一道评估题的输入、证据及检查结果。
type DemoCase struct {
	Question  string                `json:"question"`  // 固定题目。
	Expected  string                `json:"expected"`  // 预期数字。
	Passed    bool                  `json:"passed"`    // 工具和回答是否都通过检查。
	Execution *AgentExecutionResult `json:"execution"` // 本题实际执行记录。
	Error     string                `json:"error"`     // 失败说明，成功时为空。
}

// DemosOverviewRsp 定义Agent 到底是什么的 HTTP JSON 表现。
type DemosOverviewRsp struct {
	Direct    *AgentExecutionResult `json:"direct"`    // 直接回答的过程。
	WithTools *AgentExecutionResult `json:"withTools"` // 允许调用工具的过程。
}

// DemosChatRsp 定义普通聊天的 HTTP JSON 表现。
type DemosChatRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}

// DemosMessagesRsp 定义消息历史的 HTTP JSON 表现。
type DemosMessagesRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}

// DemosToolsRsp 定义工具调用与循环的 HTTP JSON 表现。
type DemosToolsRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}

// DemosStructuredRsp 定义结构化输出的 HTTP JSON 表现。
type DemosStructuredRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
	Product   string                `json:"product"`   // 校验通过的商品名称。
	Quantity  int                   `json:"quantity"`  // 校验通过的商品数量。
}

// DemosMultipleToolsRsp 定义多个工具协作的 HTTP JSON 表现。
type DemosMultipleToolsRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}

// DemosStreamRsp 定义流式回答的 HTTP JSON 表现。
type DemosStreamRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
	Delta     string                `json:"delta"`     // 本次新生成的文字。
	Done      bool                  `json:"done"`      // 是否为最终事件，成功与否另看 code。
}

// DemosReliabilityRsp 定义失败与执行边界的 HTTP JSON 表现。
type DemosReliabilityRsp struct {
	Execution  *AgentExecutionResult `json:"execution"`  // 实际执行结果。
	StopReason string                `json:"stopReason"` // 执行停止原因。
}

// DemosRetrievalRsp 定义检索与记忆的 HTTP JSON 表现。
type DemosRetrievalRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
	Sources   []DemoSource          `json:"sources"`   // 实际命中的资料片段。
}

// DemosWorkflowRsp 定义工作流的 HTTP JSON 表现。
type DemosWorkflowRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
	Stages    []DemoStage           `json:"stages"`    // 已完成的工作流步骤。
}

// DemosObservationRsp 定义日志与观测的 HTTP JSON 表现。
type DemosObservationRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}

// DemosEvaluationRsp 定义评估与回归的 HTTP JSON 表现。
type DemosEvaluationRsp struct {
	Cases  []DemoCase `json:"cases"`  // 已运行的固定题目。
	Passed int        `json:"passed"` // 通过检查的题数。
	Total  int        `json:"total"`  // 固定题集总题数。
}

// DemosDeliveryRsp 定义综合练习的 HTTP JSON 表现。
type DemosDeliveryRsp struct {
	Execution *AgentExecutionResult `json:"execution"` // 实际执行结果。
}
