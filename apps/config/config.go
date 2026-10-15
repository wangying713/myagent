// Package config 只读取环境变量与显式指定的文件，不修改进程环境或自动读取 old。
package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/deepseek"
	"sakelei/ai/infra/openobserve"
)

type Settings struct {
	Model   deepseek.Config
	Observe openobserve.Config
	HTTP    HTTPSettings
	Limits  agent.Limits
}

// HTTPSettings 保存 HTTP 进程启动设置，入口将请求超时显式传给 API。
type HTTPSettings struct {
	Address    string
	RunTimeout time.Duration
}

func Load(envFile string, lookup func(string) (string, bool)) (Settings, error) {
	values := map[string]string{}
	if envFile != "" {
		var err error
		values, err = godotenv.Read(envFile)
		if err != nil {
			return Settings{}, errors.New("无法读取指定的配置文件，请检查路径与格式")
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
	settings := Settings{
		Model:   deepseek.Config{APIKey: get("DEEPSEEK_API_KEY", ""), Model: get("DEEPSEEK_MODEL", "deepseek-flash"), BaseURL: get("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")},
		Observe: openobserve.Config{ServiceName: get("OTEL_SERVICE_NAME", "go-agent-demos"), Endpoint: get("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:5080/api/default/v1/traces")},
		HTTP:    HTTPSettings{Address: get("HTTP_ADDRESS", "127.0.0.1:8080")},
	}
	var err error
	settings.Model.Timeout, err = time.ParseDuration(get("DEEPSEEK_TIMEOUT", "60s"))
	if err != nil || settings.Model.Timeout <= 0 {
		return settings, errors.New("DEEPSEEK_TIMEOUT 必须为正数时长，例如 60s")
	}
	settings.HTTP.RunTimeout, err = time.ParseDuration(get("HTTP_RUN_TIMEOUT", "120s"))
	if err != nil || settings.HTTP.RunTimeout <= 0 {
		return settings, errors.New("HTTP_RUN_TIMEOUT 必须为正数时长，例如 120s")
	}
	if _, port, err := net.SplitHostPort(settings.HTTP.Address); err != nil || port == "" {
		return settings, errors.New("HTTP_ADDRESS 应为 host:port")
	}
	boolean := func(key string, fallback bool) (bool, error) {
		value := strings.ToLower(strings.TrimSpace(get(key, strconv.FormatBool(fallback))))
		if value != "true" && value != "false" {
			return false, errors.New(key + " 必须为 true 或 false")
		}
		return value == "true", nil
	}
	integer := func(key string, fallback int) (int, error) {
		value, err := strconv.Atoi(get(key, strconv.Itoa(fallback)))
		if err != nil || value <= 0 {
			return 0, errors.New(key + " 必须为正整数")
		}
		return value, nil
	}
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
	settings.Observe.Headers = make(map[string]string)
	for _, item := range strings.Split(get("OTEL_EXPORTER_OTLP_TRACES_HEADERS", ""), ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return settings, errors.New("观测 header 应为 key=value 列表")
		}
		value, err = url.PathUnescape(strings.TrimSpace(value))
		if err != nil || strings.ContainsAny(key+value, "\r\n") {
			return settings, errors.New("观测 header 编码无效")
		}
		for existing := range settings.Observe.Headers {
			if strings.EqualFold(existing, key) {
				return settings, errors.New("观测 header 不能重复")
			}
		}
		settings.Observe.Headers[key] = value
	}
	if settings.Observe.Enabled {
		auth := false
		for key, value := range settings.Observe.Headers {
			if strings.EqualFold(key, "Authorization") && value != "" {
				auth = true
			}
		}
		if !auth {
			return settings, errors.New("启用观测时请填写 OTEL_EXPORTER_OTLP_TRACES_HEADERS 的 Authorization")
		}
	}
	for _, endpoint := range []string{settings.Model.BaseURL, settings.Observe.Endpoint} {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return settings, errors.New("服务地址应为不含凭据和查询参数的 HTTP(S) URL")
		}
	}
	if settings.Observe.ServiceName == "" || !strings.HasSuffix(settings.Observe.Endpoint, "/v1/traces") {
		return settings, errors.New("观测服务名不能为空，端点应以 /v1/traces 结束")
	}
	settings.Observe.Secrets = []string{settings.Model.APIKey}
	settings.Limits.ModelCalls, err = integer("AGENT_MAX_MODEL_CALLS", 5)
	if err != nil {
		return settings, err
	}
	settings.Limits.ToolCalls, err = integer("AGENT_MAX_TOOL_CALLS", 8)
	if err != nil {
		return settings, err
	}
	settings.Limits.Tokens, err = integer("AGENT_TOKEN_BUDGET", 8000)
	return settings, err
}
