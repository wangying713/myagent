package openobserve

import "fmt"

// MinContentMaxChars 是观测正文字符上限允许的最小值。
const MinContentMaxChars = 256

// Config 保存私有观测设置及展示正文的脱敏规则，由启动入口显式传入。
type Config struct {
	Enabled     bool              // 是否创建并上报链路观测数据。
	ServiceName string            // 链路数据中的服务名称。
	TracerName  string            // 创建业务与模型 span 的追踪器名称，由应用显式提供。
	Endpoint    string            // OTLP HTTP Traces 上报地址。
	Headers     map[string]string // OTLP 上报请求头，可包含鉴权信息。
	// CaptureContent 只控制 span 正文采集，执行记录仍可生成脱敏展示副本。
	CaptureContent bool
	// MaxChars 限制展示正文的 Unicode 字符数，额外的截断标记不计入该上限。
	MaxChars int
	Secrets  []string // 展示正文时需要遮盖的额外敏感值。
}

// validate 检查客户端必需配置；启动默认值和环境解析由 apps/config 负责。
func (cfg Config) validate() error {
	if cfg.ServiceName == "" || cfg.TracerName == "" {
		return fmt.Errorf("观测服务名和追踪器名称不能为空")
	}
	if cfg.MaxChars < MinContentMaxChars {
		return fmt.Errorf("观测正文上限至少为 %d 字符", MinContentMaxChars)
	}
	return nil
}
