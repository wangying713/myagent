package openobserve

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSafePayloadRedactionAndTruncation(t *testing.T) {
	observeCli, err := New(context.Background(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
		Secrets:     []string{"real-key"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := `{"api_key":"real-key","arguments":"{\"password\":\"hidden-value\",\"a\":1}","text":"Bearer token-value"}`
	raw := []byte(input)
	original := bytes.Clone(raw)
	safe := observeCli.Safe(raw)
	for _, secret := range []string{
		"real-key",
		"hidden-value",
		"token-value",
	} {
		if strings.Contains(safe, secret) {
			t.Errorf("脱敏后保留敏感值：%s", safe)
		}
	}
	if !strings.Contains(safe, "[REDACTED]") {
		t.Error("缺少脱敏标记")
	}
	safe = observeCli.Safe([]byte(strings.Repeat("中文", 200)))
	if !strings.HasSuffix(safe, "…[TRUNCATED]") {
		t.Error("缺少截断标记")
	}
	if !bytes.Equal(raw, original) {
		t.Error("展示脱敏不应修改业务输入")
	}
}

func TestSafeNestedJSONPreservesNumbers(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"9007199254740993",
		"9223372036854775807",
		"0.123456789012345678901",
		"1e400",
	} {
		t.Run(value, func(t *testing.T) {
			arguments := `{"a":` + value + `,"secret":"hidden"}`
			raw, err := json.Marshal(map[string]string{"arguments": arguments})
			if err != nil {
				t.Fatal(err)
			}
			original := bytes.Clone(raw)
			var payload map[string]string
			if err := json.Unmarshal([]byte(observeCli.Safe(raw)), &payload); err != nil {
				t.Fatal(err)
			}
			var nested map[string]json.RawMessage
			if err := json.Unmarshal([]byte(payload["arguments"]), &nested); err != nil {
				t.Fatal(err)
			}
			if string(nested["a"]) != value || string(nested["secret"]) != `"[REDACTED]"` {
				t.Errorf("实际展示 %s，预期保留数字 %s 并脱敏 secret", payload["arguments"], value)
			}
			if !bytes.Equal(raw, original) {
				t.Error("展示处理修改了原始正文")
			}
		})
	}
}

func TestCleanMatchesLongSecretsBeforeShortSecrets(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
		Secrets: []string{
			"",
			"abc",
			"abc-def",
			"abc",
		},
		Headers: map[string]string{
			"Authorization": "Basic auth-token",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := observeCli.Clean("abc-def abc Basic auth-token auth-token")
	want := "[REDACTED] [REDACTED] [REDACTED] [REDACTED]"
	if got != want {
		t.Errorf("实际脱敏文本 %q，预期 %q", got, want)
	}
}

func TestSafePreservesPartialJSONAndZeroValues(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string // 可能是完整 JSON、截断片段或带尾随内容的文本。
		want  string // 脱敏后应保持的原始格式与零值。
	}{
		{
			input: `{"password":"value",`,
			want:  `{"password":"[REDACTED]",`,
		},
		{
			input: `null`,
			want:  `null`,
		},
		{
			input: `{"flag":false,"number":0}`,
			want:  `{"flag":false,"number":0}`,
		},
		{
			input: `{"number":9007199254740993} trailing`,
			want:  `{"number":9007199254740993} trailing`,
		},
	} {
		if got := observeCli.Safe([]byte(tc.input)); got != tc.want {
			t.Errorf("输入 %q 的实际展示 %q，预期 %q", tc.input, got, tc.want)
		}
	}
}

func TestSafeDoesNotInferSSEFromOrdinaryText(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := "data:\n这只是普通数据说明，并不是 SSE。"
	if got := observeCli.Safe([]byte(text)); got != text {
		t.Fatalf("实际普通正文 %q，预期保留 %q", got, text)
	}
	raw, _ := json.Marshal(map[string]string{"question": text})
	var rsp map[string]string
	if err := json.Unmarshal([]byte(observeCli.Safe(raw)), &rsp); err != nil {
		t.Fatal(err)
	}
	if rsp["question"] != text {
		t.Fatalf("实际 JSON 内的正文 %q，预期保留 %q", rsp["question"], text)
	}
}
