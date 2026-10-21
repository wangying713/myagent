// Package config 只读取环境变量与显式指定的文件，不修改进程环境或自动读取 old。
package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/deepseek"
	"saklay/ai/infra/openobserve"

	"github.com/joho/godotenv"
)

// Settings 保存校验后的启动设置，由入口显式传给各组件。
type Settings struct {
	Model   deepseek.Config    // 模型客户端配置。
	Observe openobserve.Config // 链路观测与正文采集配置。
	HTTP    HTTPSettings       // HTTP 监听与业务超时配置。
	Limits  agent.Limits       // 每次业务运行的调用与用量预算。
}

// HTTPSettings 保存监听地址和业务执行超时；业务计时从请求解码及准备完成后开始。
type HTTPSettings struct {
	Address    string        // HTTP 服务监听地址，格式为 host:port。
	RunTimeout time.Duration // 请求解码和准备完成后的业务执行超时。
}

// configValue 按环境、显式文件和默认值的优先级取得配置文本。
type configValue func(key, fallback string) string

// Load 读取显式指定的文件并校验启动设置；失败时不返回部分配置。
// envFile 为空时不读文件；lookup 为 nil 时不读取进程环境，显式空值仍覆盖文件和默认值。
func Load(envFile string, lookup func(string) (string, bool)) (*Settings, error) {
	values := map[string]string{}
	if envFile != "" {
		var err error
		values, err = godotenv.Read(envFile)
		if err != nil {
			return nil, errors.New("无法读取指定的配置文件，请检查路径与格式")
		}
	}
	get := newConfigValue(values, lookup)
	settings := defaultSettings(get)
	if err := loadServiceTiming(settings, get); err != nil {
		return nil, err
	}
	if err := loadObserveSettings(settings, get); err != nil {
		return nil, err
	}
	if err := validateEndpoints(settings); err != nil {
		return nil, err
	}
	settings.Observe.Secrets = []string{settings.Model.APIKey}
	if err := loadLimits(settings, get); err != nil {
		return nil, err
	}
	return settings, nil
}

// newConfigValue 保留显式空值，且不修改进程环境。
func newConfigValue(values map[string]string, lookup func(string) (string, bool)) configValue {
	return func(key, fallback string) string {
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
}

// defaultSettings 填写尚不需要解析的模型、观测和 HTTP 设置。
func defaultSettings(get configValue) *Settings {
	return &Settings{
		Model: deepseek.Config{
			APIKey:  get("DEEPSEEK_API_KEY", ""),
			Model:   get("DEEPSEEK_MODEL", "deepseek-flash"),
			BaseURL: get("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"),
		},
		Observe: openobserve.Config{
			ServiceName: get("OTEL_SERVICE_NAME", "agent"),
			Endpoint:    get("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:5080/api/default/v1/traces"),
		},
		HTTP: HTTPSettings{
			Address: get("HTTP_ADDRESS", "127.0.0.1:8080"),
		},
	}
}

// loadServiceTiming 解析时长并检查 HTTP 监听地址。
func loadServiceTiming(settings *Settings, get configValue) error {
	var err error
	settings.Model.Timeout, err = time.ParseDuration(get("DEEPSEEK_TIMEOUT", "60s"))
	if err != nil || settings.Model.Timeout <= 0 {
		return errors.New("DEEPSEEK_TIMEOUT 必须为正数时长，例如 60s")
	}
	settings.HTTP.RunTimeout, err = time.ParseDuration(get("HTTP_RUN_TIMEOUT", "120s"))
	if err != nil || settings.HTTP.RunTimeout <= 0 {
		return errors.New("HTTP_RUN_TIMEOUT 必须为正数时长，例如 120s")
	}
	if _, port, err := net.SplitHostPort(settings.HTTP.Address); err != nil || port == "" {
		return errors.New("HTTP_ADDRESS 应为 host:port")
	}
	return nil
}

// configBool 只接受 true 和 false 文本，忽略大小写和两端空白。
func configBool(get configValue, key string, fallback bool) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(get(key, strconv.FormatBool(fallback))))
	if value != "true" && value != "false" {
		return false, errors.New(key + " 必须为 true 或 false")
	}
	return value == "true", nil
}

// configInteger 只接受正整数。
func configInteger(get configValue, key string, fallback int) (int, error) {
	value, err := strconv.Atoi(get(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return 0, errors.New(key + " 必须为正整数")
	}
	return value, nil
}

// loadObserveSettings 校验采集选项与鉴权配置。
func loadObserveSettings(settings *Settings, get configValue) error {
	var err error
	settings.Observe.Enabled, err = configBool(get, "TELEMETRY_ENABLED", true)
	if err != nil {
		return err
	}
	settings.Observe.CaptureContent, err = configBool(get, "TELEMETRY_CAPTURE_CONTENT", true)
	if err != nil {
		return err
	}
	settings.Observe.MaxChars, err = configInteger(get, "TELEMETRY_CONTENT_MAX_CHARS", 32768)
	if err != nil || settings.Observe.MaxChars < 256 {
		return errors.New("TELEMETRY_CONTENT_MAX_CHARS 至少为 256")
	}
	settings.Observe.Headers, err = parseObserveHeaders(get("OTEL_EXPORTER_OTLP_TRACES_HEADERS", ""))
	if err != nil {
		return err
	}
	if settings.Observe.Enabled {
		for key, value := range settings.Observe.Headers {
			if strings.EqualFold(key, "Authorization") && value != "" {
				return nil
			}
		}
		return errors.New("启用观测时请填写 OTEL_EXPORTER_OTLP_TRACES_HEADERS 的 Authorization")
	}
	return nil
}

// parseObserveHeaders 拒绝换行、重复字段和无效的 key=value 格式。
func parseObserveHeaders(text string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, item := range strings.Split(text, ",") {
		if strings.TrimSpace(item) == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, errors.New("观测 header 应为 key=value 列表")
		}
		value, err := url.PathUnescape(strings.TrimSpace(value))
		if err != nil || strings.ContainsAny(key+value, "\r\n") {
			return nil, errors.New("观测 header 编码无效")
		}
		for existing := range headers {
			if strings.EqualFold(existing, key) {
				return nil, errors.New("观测 header 不能重复")
			}
		}
		headers[key] = value
	}
	return headers, nil
}

// validateEndpoints 拒绝凭据、查询参数、无效服务名和非 Traces 端点。
func validateEndpoints(settings *Settings) error {
	for _, endpoint := range []string{
		settings.Model.BaseURL,
		settings.Observe.Endpoint,
	} {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("服务地址应为不含凭据和查询参数的 HTTP(S) URL")
		}
	}
	if settings.Observe.ServiceName == "" || !strings.HasSuffix(settings.Observe.Endpoint, "/v1/traces") {
		return errors.New("观测服务名不能为空，端点应以 /v1/traces 结束")
	}
	return nil
}

// loadLimits 校验模型、工具和 token 的单次运行预算。
func loadLimits(settings *Settings, get configValue) error {
	defaults := agent.DefaultLimits()
	var err error
	settings.Limits.ModelCalls, err = configInteger(get, "AGENT_MAX_MODEL_CALLS", defaults.ModelCalls)
	if err != nil {
		return err
	}
	settings.Limits.ToolCalls, err = configInteger(get, "AGENT_MAX_TOOL_CALLS", defaults.ToolCalls)
	if err != nil {
		return err
	}
	settings.Limits.Tokens, err = configInteger(get, "AGENT_TOKEN_BUDGET", defaults.Tokens)
	return err
}
