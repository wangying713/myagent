package agentapi

import (
	"time"

	"saklay/ai/apps/interfaces/http/proto"
	"saklay/ai/infra/httpx"
)

// NewAgentAdapter 为三个 API 注入相同的 JSON 格式与业务错误映射。
func NewAgentAdapter(timeout time.Duration) (*httpx.Adapter[*httpproto.ExecutionResult], error) {
	return httpx.NewAdapter(timeout, func(response *httpproto.ExecutionResult) any {
		return resultJSON(response)
	}, classify)
}

// PrepareMessages 在 HTTP 解码后保留省略与显式空数组的区别。
// repeated 字段本身没有存在性，不能把显式空数组变为默认问题。
func PrepareMessages(req *httpproto.MessagesRequest, present map[string]bool) {
	if present["questions"] && len(req.Questions) == 0 {
		req.Questions = []string{}
	}
}
