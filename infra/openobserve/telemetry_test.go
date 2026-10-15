package openobserve

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSafePayloadRedactionAndTruncation(t *testing.T) {
	telemetry, err := New(context.Background(), Config{ServiceName: "test", MaxChars: 256, Secrets: []string{"real-key"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	input := `{"api_key":"real-key","arguments":"{\"password\":\"hidden-value\",\"a\":1}","text":"Bearer token-value"}`
	raw := []byte(input)
	original := bytes.Clone(raw)
	safe := telemetry.Safe(raw)
	for _, secret := range []string{"real-key", "hidden-value", "token-value"} {
		if strings.Contains(safe, secret) {
			t.Errorf("脱敏后保留敏感值：%s", safe)
		}
	}
	if !strings.Contains(safe, "[REDACTED]") {
		t.Error("缺少脱敏标记")
	}
	safe = telemetry.Safe([]byte(strings.Repeat("中文", 200)))
	if !strings.HasSuffix(safe, "…[TRUNCATED]") {
		t.Error("缺少截断标记")
	}
	if !bytes.Equal(raw, original) {
		t.Error("展示脱敏不应修改业务输入")
	}
}
