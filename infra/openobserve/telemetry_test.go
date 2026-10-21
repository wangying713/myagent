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
