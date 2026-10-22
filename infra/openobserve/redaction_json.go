package openobserve

import "strings"

// jsonRedactionState 表示敏感 JSON 字段的增量读取阶段。
type jsonRedactionState uint8

const (
	jsonSearchKey    jsonRedactionState = iota // 查找敏感字段名。
	jsonNeedColon                              // 字段名结束，等待冒号。
	jsonNeedValue                              // 冒号结束，等待非空白的值。
	jsonSkipQuoted                             // 已遮盖带引号的值，等待未转义的结束引号。
	jsonSkipUnquoted                           // 已遮盖非引号值，等待分隔符。
)

// streamJSONKeys 是普通敏感字段规则对应的有限 JSON 键名。
var streamJSONKeys = []string{
	`"authorization"`,
	`"api_key"`,
	`"api-key"`,
	`"apikey"`,
	`"password"`,
	`"secret"`,
	`"access_token"`,
	`"access-token"`,
	`"accesstoken"`,
	`"refresh_token"`,
	`"refresh-token"`,
	`"refreshtoken"`,
}

// jsonRedactor 在值开始时输出遮盖标记，只保存引号和转义状态。
type jsonRedactor struct {
	state   jsonRedactionState // 当前读取阶段，零值表示查找字段名。
	pending string             // 尚未确定的字段名前缀，长度小于最长敏感键名。
	escaped bool               // 引号值中的前一个字节是否为未配对的反斜杠。
}

// Write 按新字节推进状态，不缓存敏感值正文。
func (r *jsonRedactor) Write(text string) string {
	var output strings.Builder
	for i := 0; i < len(text); i++ {
		r.writeByte(text[i], &output)
	}
	return output.String()
}

// Flush 输出未构成敏感字段的短前缀，并重置状态。
func (r *jsonRedactor) Flush() string {
	text := r.pending
	*r = jsonRedactor{}
	return text
}

// writeByte 遮盖字段值，同时保留字段名、空白和外层分隔符。
func (r *jsonRedactor) writeByte(char byte, output *strings.Builder) {
	switch r.state {
	case jsonNeedColon:
		if authSpace(char) {
			output.WriteByte(char)
		} else if char == ':' {
			output.WriteByte(char)
			r.state = jsonNeedValue
		} else {
			r.state = jsonSearchKey
			r.writeByte(char, output)
		}
	case jsonNeedValue:
		if authSpace(char) {
			output.WriteByte(char)
		} else if strings.ContainsRune(",}]", rune(char)) {
			r.state = jsonSearchKey
			r.writeByte(char, output)
		} else {
			output.WriteString(`"` + redactedText + `"`)
			r.state = jsonSkipUnquoted
			if char == '"' {
				r.state = jsonSkipQuoted
			}
		}
	case jsonSkipQuoted:
		if r.escaped {
			r.escaped = false
		} else if char == '\\' {
			r.escaped = true
		} else if char == '"' {
			r.state = jsonSearchKey
		}
	case jsonSkipUnquoted:
		if authSpace(char) || strings.ContainsRune(",}]", rune(char)) {
			r.state = jsonSearchKey
			r.writeByte(char, output)
		}
	default:
		r.writeKey(char, output)
	}
}

// writeKey 仅暂存可能构成敏感字段名的短前缀。
func (r *jsonRedactor) writeKey(char byte, output *strings.Builder) {
	if r.pending == "" && char != '"' {
		output.WriteByte(char)
		return
	}
	candidate := r.pending + string(char)
	lower := strings.ToLower(candidate)
	for _, key := range streamJSONKeys {
		if !strings.HasPrefix(key, lower) {
			continue
		}
		r.pending = candidate
		if lower == key {
			output.WriteString(candidate)
			r.pending = ""
			r.state = jsonNeedColon
		}
		return
	}
	output.WriteString(r.pending)
	r.pending = ""
	r.writeByte(char, output)
}
