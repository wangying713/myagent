package openobserve

import (
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	redactedText       = "[REDACTED]"   // 展示副本中的敏感值替代文本。
	truncationSuffix   = "…[TRUNCATED]" // 超过正文字符上限时追加的截断标记。
	maxJSONDepth       = 32             // JSON 脱敏允许递归的最大层数。
	requestPayloadKey  = "app.request"  // 输入正文的观测属性名。
	responsePayloadKey = "app.response" // 输出正文的观测属性名。
)

// collectSecrets 整理配置中的脱敏词，丢弃空值、去重并优先匹配长词。
func collectSecrets(cfg Config) []string {
	unique := make(map[string]struct{})
	for _, value := range cfg.Secrets {
		unique[value] = struct{}{}
	}
	for _, value := range cfg.Headers {
		unique[value] = struct{}{}
		if _, token, ok := strings.Cut(value, " "); ok {
			unique[token] = struct{}{}
		}
	}
	delete(unique, "")
	secrets := make([]string, 0, len(unique))
	for value := range unique {
		secrets = append(secrets, value)
	}
	sort.Slice(secrets, func(i, j int) bool {
		if len(secrets[i]) == len(secrets[j]) {
			return secrets[i] < secrets[j]
		}
		return len(secrets[i]) > len(secrets[j])
	})
	return secrets
}

// sensitiveKey 匹配 JSON 中的敏感字段名。
var sensitiveKey = regexp.MustCompile(`(?i)authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token`)

// authValue 匹配常见鉴权文本和密钥格式。
var authValue = regexp.MustCompile(`(?i)\b(?:Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+|\bsk-[A-Za-z0-9_-]{8,}`)

// jsonSecret 遮盖文本片段中 JSON 形式的敏感字段值。
var jsonSecret = regexp.MustCompile(`(?i)("(?:authorization|api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token)"\s*:\s*)("(?:\\.|[^"\\])*"|[^,}\]\s]+)`)

// Clean 遮盖已配置密钥及常见鉴权文本，返回展示副本，不限制正文长度。
func (c *Client) Clean(text string) string {
	for _, secret := range c.secrets {
		text = strings.ReplaceAll(text, secret, redactedText)
	}
	return jsonSecret.ReplaceAllString(authValue.ReplaceAllString(text, redactedText), `${1}"`+redactedText+`"`)
}

// clean 只处理 decodeJSON 产生的 JSON 值，替换为字符串后仍保证可编码。
func (c *Client) clean(value any, depth int) any {
	if depth >= maxJSONDepth {
		return redactedText
	}
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if sensitiveKey.MatchString(key) {
				v[key] = redactedText
			} else {
				v[key] = c.clean(child, depth+1)
			}
		}
	case []any:
		for i, child := range v {
			v[i] = c.clean(child, depth+1)
		}
	case string:
		if nested, ok := decodeJSON(v); ok {
			encoded, _ := json.Marshal(c.clean(nested, depth+1))
			return string(encoded)
		}
		return c.Clean(v)
	}
	return value
}

// Safe 返回脱敏、限长的展示副本，不修改实际请求、响应或回填消息。
func (c *Client) Safe(raw []byte) string {
	text := string(raw)
	if value, ok := decodeJSON(text); ok {
		encoded, _ := json.Marshal(c.clean(value, 0))
		text = string(encoded)
	}
	text = c.Clean(text)
	runes := []rune(text)
	if len(runes) > c.maxChars {
		return string(runes[:c.maxChars]) + truncationSuffix
	}
	return text
}

// Payload 在启用正文采集时记录脱敏、限长的正文和截断标记，不修改 raw。
func (c *Client) Payload(span trace.Span, key string, raw []byte) {
	if c.ContentLimit() == 0 || !span.IsRecording() {
		return
	}
	text := c.Safe(raw)
	span.SetAttributes(attribute.String(key, text), attribute.Bool(key+".truncated", strings.HasSuffix(text, truncationSuffix)))
	if key == requestPayloadKey || key == responsePayloadKey {
		previewKey := "gen_ai.input.messages"
		if key == responsePayloadKey {
			previewKey = "gen_ai.output.messages"
		}
		// 直接交给 OpenObserve 的 JSON 预览，不添加对话角色或消息包装。
		span.SetAttributes(attribute.String(previewKey, text))
	}
}

// decodeJSON 只接受单个完整 JSON 值，保留数值原文并拒绝尾随内容。
func decodeJSON(text string) (any, bool) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, false
	}
	return value, true
}
