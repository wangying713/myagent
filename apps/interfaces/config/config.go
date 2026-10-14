// Package config 从显式配置文件和环境读取配置，不修改进程环境。
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"sakelei/ai/domain/agent/service"
	"sakelei/ai/infra/deepseek"
	"sakelei/ai/infra/openobserve"
)

// Settings 按职责组合配置，凭据对象不应作为日志打印。
type Settings struct {
	Model   deepseek.Config
	Observe openobserve.Config
	Limits  service.Limits
}

// Load 只读取明确指定的文件；lookup 中存在的环境变量始终优先，包括空值。
func Load(envFile string, lookup func(string) (string, bool)) (Settings, error) {
	var settings Settings
	values := map[string]string{}
	if envFile != "" {
		var err error
		values, err = godotenv.Read(envFile)
		if err != nil {
			return settings, errors.New("无法读取指定的配置文件，请检查路径和格式")
		}
	}
	get := func(key, fallback string) string {
		if lookup != nil {
			if value, ok := lookup(key); ok {
				return value
			}
		}
		if value, ok := values[key]; ok {
			return value
		}
		return fallback
	}
	boolean := func(key string, fallback bool) (bool, error) {
		value := strings.ToLower(strings.TrimSpace(get(key, strconv.FormatBool(fallback))))
		switch value {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, fmt.Errorf("%s 必须为 true 或 false", key)
		}
	}
	integer := func(key string, fallback int) (int, error) {
		value, err := strconv.Atoi(get(key, strconv.Itoa(fallback)))
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s 必须为正整数", key)
		}
		return value, nil
	}
	settings.Model.APIKey = get("DEEPSEEK_API_KEY", "")
	settings.Model.Model = get("DEEPSEEK_MODEL", "deepseek-flash")
	settings.Model.BaseURL = strings.TrimRight(get("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"), "/")
	timeout, err := time.ParseDuration(get("DEEPSEEK_TIMEOUT", "60s"))
	if err != nil || timeout <= 0 {
		return settings, errors.New("DEEPSEEK_TIMEOUT 必须是正数时长，例如 60s")
	}
	settings.Model.Timeout = timeout
	settings.Observe.ServiceName = get("OTEL_SERVICE_NAME", "go-agent-learning")
	settings.Observe.Endpoint = get("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:5080/api/default/v1/traces")
	settings.Observe.Enabled, err = boolean("TELEMETRY_ENABLED", true)
	if err != nil {
		return settings, err
	}
	settings.Observe.CaptureContent, err = boolean("TELEMETRY_CAPTURE_CONTENT", true)
	if err != nil {
		return settings, err
	}
	settings.Observe.MaxChars, err = integer("TELEMETRY_CONTENT_MAX_CHARS", 32768)
	if err != nil || settings.Observe.MaxChars < 256 {
		return settings, errors.New("TELEMETRY_CONTENT_MAX_CHARS 至少为 256")
	}
	settings.Observe.Headers, err = ParseHeaders(get("OTEL_EXPORTER_OTLP_TRACES_HEADERS", ""))
	if err != nil {
		return settings, err
	}
	if settings.Observe.Enabled {
		hasAuth := false
		for key, value := range settings.Observe.Headers {
			if strings.EqualFold(key, "Authorization") && value != "" {
				hasAuth = true
			}
		}
		if !hasAuth {
			return settings, errors.New("启用观测时请配置 OTEL_EXPORTER_OTLP_TRACES_HEADERS 中的 Authorization")
		}
	}
	settings.Observe.Secrets = []string{settings.Model.APIKey}
	for _, endpoint := range []string{settings.Model.BaseURL, settings.Observe.Endpoint} {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return settings, errors.New("服务端点应为不含凭据、查询参数或片段的 HTTP(S) URL")
		}
	}
	if !strings.HasSuffix(settings.Observe.Endpoint, "/v1/traces") || settings.Observe.ServiceName == "" || settings.Model.Model == "" {
		return settings, errors.New("请检查模型名、服务名和以 /v1/traces 结尾的观测端点")
	}
	settings.Limits.ModelCalls, err = integer("AGENT_MAX_MODEL_CALLS", 5)
	if err != nil {
		return settings, err
	}
	settings.Limits.ToolCalls, err = integer("AGENT_MAX_TOOL_CALLS", 8)
	if err != nil {
		return settings, err
	}
	settings.Limits.TokenBudget, err = integer("AGENT_TOKEN_BUDGET", 8000)
	return settings, err
}

// ParseHeaders 解析逗号分隔的 key=value，允许值包含等号和百分号编码。
func ParseHeaders(value string) (map[string]string, error) {
	headers := make(map[string]string)
	seen := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		key, val, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || seen[strings.ToLower(key)] {
			return nil, errors.New("观测 header 应为不重复的 key=value 列表")
		}
		decoded, err := url.PathUnescape(strings.TrimSpace(val))
		if err != nil || strings.ContainsAny(key+decoded, "\r\n") {
			return nil, errors.New("观测 header 编码或换行无效")
		}
		seen[strings.ToLower(key)] = true
		headers[key] = decoded
	}
	return headers, nil
}
