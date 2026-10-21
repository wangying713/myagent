package openobserve

import (
	"encoding/json"
	"strings"
	"testing"

	"saklay/ai/domain/agent"
)

func TestRunCallPreviewPreservesJSONAndPartialText(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	_, run := observeCli.StartRun(t.Context(), agent.OperationRun, nil)
	result := &agent.ChatRsp{
		Input: agent.RunInput{
			Tools: []agent.ToolDefinition{
				{
					Type: agent.ToolTypeFunction,
					Function: agent.FunctionDefinition{
						Name:       "test-tool",
						Parameters: json.RawMessage(`{"minimum":9007199254740993}`),
					},
				},
			},
		},
		Calls: []agent.CallRecord{
			{
				Kind:      agent.CallKindModel,
				Name:      "test-model",
				Request:   `{"count":9007199254740993,"password":"secret"}`,
				Response:  "响应片段 configured-secret",
				ElapsedMS: 12,
			},
		},
	}
	run.End(result, agent.ErrInvalidResponse)
	attrs, _ := runAttributes(t, observeCli, exporter)
	input := attrs["app.request"].AsString()
	if !strings.Contains(input, `"parameters":{"minimum":9007199254740993}`) {
		t.Errorf("实际输入 %s，预期工具参数保持对象形式及大整数精度", input)
	}
	output := attrs["app.response"].AsString()
	for _, want := range []string{
		`"request":{"count":9007199254740993,"password":"[REDACTED]"}`,
		`"response":"响应片段 [REDACTED]"`,
		`"elapsed_ms":12`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("实际输出 %s，预期包含 %s", output, want)
		}
	}
	if result.Calls[0].Response != "响应片段 configured-secret" {
		t.Error("观测预览修改了原始调用记录")
	}
}

func TestRunPreviewWithoutCallsUsesEmptyArray(t *testing.T) {
	observeCli, exporter := newTestClient(t, true, 4096)
	_, run := observeCli.StartRun(t.Context(), agent.OperationChat, nil)
	run.End(&agent.ChatRsp{}, nil)
	attrs, _ := runAttributes(t, observeCli, exporter)
	if !strings.Contains(attrs["app.response"].AsString(), `"steps":[]`) {
		t.Errorf("实际输出 %s，预期无调用时 steps 为空数组", attrs["app.response"].AsString())
	}
}
