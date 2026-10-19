package deepseek

import (
	"context"
	"errors"
	"strings"
	"testing"

	"saklay/ai/domain/agent"
)

func TestReadTextStreamProtocol(t *testing.T) {
	first := "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"你好\"}}]}\n\n"
	stop := "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"
	for _, tc := range []struct {
		name  string // 协议场景。
		body  string // 模型实际送来的 SSE 文本。
		valid bool   // 是否包含完整且可接受的回答。
	}{
		{
			name:  "完整文本流",
			body:  first + stop + "data: [DONE]\n\n",
			valid: true,
		},
		{
			name:  "CRLF 注释和多行 data",
			body:  ": keep-alive\r\n\r\ndata: {\"choices\":\r\ndata: [{\"delta\":{\"content\":\"你好\"},\"finish_reason\":\"stop\"}]}\r\n\r\ndata: [DONE]\r\n\r\n",
			valid: true,
		},
		{
			name:  "单独的用量事件",
			body:  first + stop + "data: {\"choices\":[],\"usage\":{\"total_tokens\":4}}\n\ndata: [DONE]\n\n",
			valid: true,
		},
		{
			name: "没有 DONE",
			body: first + stop,
		},
		{
			name: "没有结束原因",
			body: first + "data: [DONE]\n\n",
		},
		{
			name: "输出被截断",
			body: first + "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n",
		},
		{
			name: "损坏 JSON",
			body: first + "data: {\n\n",
		},
		{
			name: "结束后又追加正文",
			body: first + stop + first + "data: [DONE]\n\n",
		},
		{
			name: "负用量",
			body: first + stop + "data: {\"usage\":{\"total_tokens\":-1}}\n\ndata: [DONE]\n\n",
		},
		{
			name: "超大事件",
			body: "data: " + strings.Repeat("x", 1<<20) + "\n\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := agent.Response{Message: agent.Message{Role: agent.RoleAssistant}}
			text := ""
			err := readTextStream(context.Background(), strings.NewReader(tc.body), streamReceiver{
				result: &result,
				emit: func(part string) error {
					text += part
					return nil
				},
			})
			if (err == nil) != tc.valid {
				t.Fatalf("实际错误 %v，预期有效 %v", err, tc.valid)
			}
			if tc.valid && (text != "你好" || result.Message.Content != text) {
				t.Fatalf("实际分段正文 %q、累计正文 %q，预期 你好", text, result.Message.Content)
			}
		})
	}
}

func TestReadTextStreamStopsWhenConsumerCancels(t *testing.T) {
	result := agent.Response{}
	err := readTextStream(context.Background(), strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"部分\"}}]}\n\n"), streamReceiver{
		result: &result,
		emit: func(string) error {
			return context.Canceled
		},
	})
	if !errors.Is(err, context.Canceled) || result.Message.Content != "部分" {
		t.Fatalf("实际错误 %v、内容 %q，预期停止并保留部分内容", err, result.Message.Content)
	}
}
