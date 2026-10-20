package entity

// DemosOverviewReq 是 Agent 概览请求。
type DemosOverviewReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosOverviewRsp 是 Agent 概览响应。
type DemosOverviewRsp struct {
	Direct    *DemoExecution `json:"direct"`    // 直接回答的过程。
	WithTools *DemoExecution `json:"withTools"` // 允许调用工具的过程。
}

// DemosChatReq 是普通聊天请求。
type DemosChatReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosChatRsp 是普通聊天响应。
type DemosChatRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemosMessagesReq 是消息历史请求。
type DemosMessagesReq struct {
	Questions []string `json:"questions"` // 两轮问题；nil 使用默认示例，显式空数组交由业务拒绝。
}

// DemosMessagesRsp 是消息历史响应。
type DemosMessagesRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemosToolsReq 是工具调用与循环请求。
type DemosToolsReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosToolsRsp 是工具调用与循环响应。
type DemosToolsRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemosStructuredReq 是结构化输出请求。
type DemosStructuredReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosStructuredRsp 是结构化输出响应。
type DemosStructuredRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
	Product   string         `json:"product"`   // 校验通过的商品名称。
	Quantity  int            `json:"quantity"`  // 校验通过的商品数量。
}

// DemosMultipleToolsReq 是多工具协作请求。
type DemosMultipleToolsReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosMultipleToolsRsp 是多工具协作响应。
type DemosMultipleToolsRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemosStreamReq 是流式回答请求。
type DemosStreamReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosStreamRsp 是流式回答响应。
type DemosStreamRsp struct {
	Execution *DemoExecution `json:"execution"` // 增量事件为 nil，最终事件携带完整或部分执行记录。
	Delta     string         `json:"delta"`     // 本次新生成的文字。
	Done      bool           `json:"done"`      // 是否为最终事件，成功与否另看 code。
}

// DemosReliabilityReq 是执行边界请求。
type DemosReliabilityReq struct {
	Question   string `json:"question"`    // 用户问题，省略或空字符串使用本章默认示例。
	ModelCalls *int   `json:"model_calls"` // 允许的模型调用次数；nil 使用本章默认值。
}

// DemosReliabilityRsp 是执行边界响应。
type DemosReliabilityRsp struct {
	Execution  *DemoExecution `json:"execution"`  // 本次执行记录。
	StopReason string         `json:"stopReason"` // 执行停止原因。
}

// DemosRetrievalReq 是检索与记忆请求。
type DemosRetrievalReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosRetrievalRsp 是检索与记忆响应。
type DemosRetrievalRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
	Sources   []DemoSource   `json:"sources"`   // 实际命中的资料片段，无命中时输出空数组。
}

// DemosWorkflowReq 是工作流请求。
type DemosWorkflowReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosWorkflowRsp 是工作流响应。
type DemosWorkflowRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
	Stages    []DemoStage    `json:"stages"`    // 已完成的工作流步骤，尚无步骤时输出空数组。
}

// DemosObservationReq 是日志与观测请求。
type DemosObservationReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosObservationRsp 是日志与观测响应。
type DemosObservationRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemosEvaluationReq 是评估与回归请求。
type DemosEvaluationReq struct{}

// DemosEvaluationRsp 是评估与回归响应。
type DemosEvaluationRsp struct {
	Cases  []DemoCase `json:"cases"`  // 已运行的固定题目，未执行时输出空数组。
	Passed int        `json:"passed"` // 通过检查的题数。
	Total  int        `json:"total"`  // 固定题集总题数。
}

// DemosDeliveryReq 是综合练习请求。
type DemosDeliveryReq struct {
	Question string `json:"question"` // 用户问题，省略或空字符串使用本章默认示例。
}

// DemosDeliveryRsp 是综合练习响应。
type DemosDeliveryRsp struct {
	Execution *DemoExecution `json:"execution"` // 本次执行记录。
}

// DemoExecution 是演示执行记录。
type DemoExecution struct {
	Lesson           string `json:"lesson"`            // 当前演示的业务操作标识。
	Completed        bool   `json:"completed"`         // 本次业务是否成功完成。
	Answer           string `json:"answer"`            // 脱敏后的回答，失败时保留已有内容。
	TelemetryEnabled bool   `json:"telemetry_enabled"` // 是否启用链路观测。
	ElapsedMS        int64  `json:"elapsed_ms"`        // 本次运行耗时，单位为毫秒。
	ModelCalls       int    `json:"model_calls"`       // 已发起的模型调用次数。
	ToolCalls        int    `json:"tool_calls"`        // 已发起的工具调用次数。
	KnownTokens      int    `json:"known_tokens"`      // 可确认的累计 token 用量。
	UsageComplete    bool   `json:"usage_complete"`    // 所有模型调用的用量是否均已确认并累计。
	// 按执行顺序保存模型与工具调用记录；nil 输出 null，非 nil 空切片输出空数组。
	Steps []AgentExecutionStep `json:"steps"`
}

// DemoSource 是检索资料片段。
type DemoSource struct {
	ID    string `json:"id"`    // 本地资料编号。
	Title string `json:"title"` // 资料标题。
	Text  string `json:"text"`  // 实际使用的资料正文。
}

// DemoStage 是已完成的工作流步骤。
type DemoStage struct {
	Name    string `json:"name"`    // 步骤名称。
	Content string `json:"content"` // 本步骤实际输出。
}

// DemoCase 是评估用例。
type DemoCase struct {
	Question  string         `json:"question"`  // 固定题目。
	Expected  string         `json:"expected"`  // 预期数字。
	Passed    bool           `json:"passed"`    // 工具和回答是否都通过检查。
	Execution *DemoExecution `json:"execution"` // 本题实际执行记录。
	Error     string         `json:"error"`     // 失败说明，成功时为空。
}
