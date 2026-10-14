package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sakelei/ai/domain/agent/service"
)

func lookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}

func TestLoadExplicitFileAndEnvironmentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DEEPSEEK_MODEL=文件模型\nDEEPSEEK_API_KEY=文件密钥\nTELEMETRY_ENABLED=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, lookup(map[string]string{"DEEPSEEK_MODEL": "环境模型", "DEEPSEEK_API_KEY": ""}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model.Model != "环境模型" || cfg.Model.APIKey != "" || cfg.Observe.Enabled {
		t.Errorf("优先级结果实际 %+v，环境变量包括空值应覆盖文件", cfg.Model.Model)
	}
	if cfg.Limits != service.DefaultLimits() {
		t.Errorf("默认预算实际 %+v，预期 %+v", cfg.Limits, service.DefaultLimits())
	}
	if _, err := Load("", lookup(map[string]string{"TELEMETRY_ENABLED": "false"})); err != nil {
		t.Errorf("未指定文件时不应读取 .env：%v", err)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for name, values := range map[string]map[string]string{
		"错误开关":        {"TELEMETRY_ENABLED": "yes"},
		"缺少鉴权":        {},
		"负数预算":        {"TELEMETRY_ENABLED": "false", "AGENT_MAX_MODEL_CALLS": "-1"},
		"非法超时":        {"TELEMETRY_ENABLED": "false", "DEEPSEEK_TIMEOUT": "nan"},
		"带凭据 URL":     {"TELEMETRY_ENABLED": "false", "DEEPSEEK_BASE_URL": "https://name:private-password@example.com/v1"},
		"带 query URL": {"TELEMETRY_ENABLED": "false", "DEEPSEEK_BASE_URL": "https://example.com/v1?key=secret"},
		"过小正文":        {"TELEMETRY_ENABLED": "false", "TELEMETRY_CONTENT_MAX_CHARS": "10"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load("", lookup(values))
			if err == nil {
				t.Fatal("无效配置未被拒绝")
			}
			if strings.Contains(err.Error(), "private-password") || strings.Contains(err.Error(), "key=secret") {
				t.Error("配置错误泄漏了敏感原值")
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	headers, err := ParseHeaders("Authorization=Basic%20abc+=,stream-name=default")
	if err != nil || headers["Authorization"] != "Basic abc+=" {
		t.Fatalf("header 解码实际 %v/%v，预期保留加号与等号", headers, err)
	}
	for _, raw := range []string{"Authorization", "Authorization=x,authorization=y", "Authorization=%0Asecret", "Authorization=%GG"} {
		if _, err := ParseHeaders(raw); err == nil {
			t.Errorf("无效 header %q 未被拒绝", raw)
		}
	}
}
