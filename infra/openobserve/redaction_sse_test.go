package openobserve

import (
	"fmt"
	"strings"
	"testing"
)

func TestSafeEventStreamRedactsAcrossEvents(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
		Secrets:     []string{"test-api-key"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := observeCli.SafeSSE([]byte("data: null\n\n"), "choices.0.delta.content"); got != "data: null\n\n" {
		t.Fatalf("实际 null 事件 %q，预期保留原始协议值", got)
	}
	for _, tc := range []struct {
		format string // SSE 事件正文的协议格式。
		path   string // 调用方明确指定的文本字段路径。
	}{
		{
			format: `{"choices":[{"delta":{"content":%q}}]}`,
			path:   "choices.0.delta.content",
		},
		{
			format: `{"code":"ok","data":{"delta":%q}}`,
			path:   "data.delta",
		},
	} {
		raw := fmt.Sprintf("data: "+tc.format+"\n\ndata: "+tc.format+"\n\ndata: [DONE]\n\n", "普通内容 test-api-", "key")
		safe := observeCli.SafeSSE([]byte(raw), tc.path)
		if strings.Contains(safe, "test-api-") || strings.Contains(safe, `"key"`) {
			t.Fatalf("跨事件敏感值未被遮盖：%s", safe)
		}
		var text string
		for _, frame := range strings.Split(safe, "\n\n") {
			payload, ok := decodeJSON(strings.TrimPrefix(frame, "data: "))
			if !ok {
				continue
			}
			field, key := streamTextField(payload, strings.Split(tc.path, "."))
			if field != nil {
				text += field[key].(string)
			}
		}
		if text != "普通内容 [REDACTED]" {
			t.Fatalf("实际展示内容 %q，预期保留非敏感文本及脱敏标记", text)
		}
		if got := observeCli.SafeSSE([]byte(raw[:len(raw)-25]), tc.path); got != redactedText {
			t.Fatalf("实际残缺 SSE 展示 %q，预期无法解码时遮盖预览", got)
		}
	}
}
