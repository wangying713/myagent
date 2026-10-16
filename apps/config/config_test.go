package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitFileAndEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DEEPSEEK_MODEL=文件模型\nDEEPSEEK_API_KEY=文件密钥\nTELEMETRY_ENABLED=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, func(name string) (string, bool) {
		if name == "DEEPSEEK_MODEL" {
			return "环境模型", true
		}
		if name == "DEEPSEEK_API_KEY" {
			return "", true
		}
		return "", false
	})
	if err != nil || cfg.Model.Model != "环境模型" || cfg.Model.APIKey != "" || cfg.Observe.Enabled {
		t.Fatalf("显式配置与环境优先级不正确：%v", err)
	}
	if cfg.HTTP.Address != "127.0.0.1:8080" || cfg.Limits.ModelCalls != 5 {
		t.Error("默认地址或课程预算发生变化")
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, values := range []map[string]string{
		{"TELEMETRY_ENABLED": "yes"},
		{"AGENT_MAX_MODEL_CALLS": "0"},
		{"HTTP_RUN_TIMEOUT": "-1s"},
		{"DEEPSEEK_BASE_URL": "https://user:secret@example.com/v1"},
		{"OTEL_EXPORTER_OTLP_TRACES_HEADERS": "Authorization=%0Asecret"},
		{"OTEL_EXPORTER_OTLP_TRACES_HEADERS": "Authorization=x,authorization=y"},
	} {
		_, err := Load("", func(key string) (string, bool) {
			if value, ok := values[key]; ok {
				return value, true
			}
			if key == "TELEMETRY_ENABLED" {
				return "false", true
			}
			return "", false
		})
		if err == nil {
			t.Fatalf("无效配置未拒绝：%v", values)
		}
	}
}
