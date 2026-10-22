package openobserve

import "strings"

// authState 表示鉴权前缀、值及已遮盖部分的读取阶段。
type authState uint8

const (
	authSearch      authState = iota // 查找单词边界上的鉴权前缀。
	authNeedSpace                    // Bearer 或 Basic 后尚未读取到必需的空白。
	authNeedToken                    // 已读取空白，等待鉴权值。
	authSkipSpace                    // 待定前缀过长，已保守遮盖，跳过剩余空白。
	authSkipToken                    // 已遮盖 Bearer 或 Basic 的值，跳过剩余值。
	authNeedSKToken                  // sk- 后尚未达到最短密钥长度。
	authSkipSKToken                  // 已遮盖 sk- 密钥，跳过剩余值。
)

const (
	maxAuthPrefixBytes = 256 // 待定鉴权前缀的最大字节数，超出后保守遮盖。
	minSKTokenChars    = 8   // sk- 格式至少包含的密钥字符数，与普通脱敏规则一致。
)

// authRedactor 增量匹配鉴权文本；确认敏感值后只保留状态，不保存值。
type authRedactor struct {
	state        authState // 当前读取阶段，零值表示查找前缀。
	pending      string    // 尚未确认的短前缀或 sk- 短值，最多 maxAuthPrefixBytes 字节。
	previousWord bool      // 前一个原始字节是否为 ASCII 单词字符，用于判断词边界。
}

// Write 同步处理新片段，不重复扫描已处理的内容。
func (r *authRedactor) Write(text string) string {
	var output strings.Builder
	for i := 0; i < len(text); i++ {
		r.writeByte(text[i], &output)
	}
	return output.String()
}

// Flush 输出最终未构成鉴权值的前缀，并重置状态。
func (r *authRedactor) Flush() string {
	text := r.pending
	*r = authRedactor{}
	return text
}

// writeByte 按当前阶段接收一个字节；不匹配的字节回到前缀搜索。
func (r *authRedactor) writeByte(char byte, output *strings.Builder) {
	switch r.state {
	case authNeedSpace, authNeedToken:
		r.writeValueStart(char, output)
	case authSkipSpace:
		if authSpace(char) {
			return
		}
		if authToken(char) {
			r.state = authSkipToken
			r.previousWord = authWord(char)
			return
		}
		r.state = authSearch
		r.writeByte(char, output)
	case authSkipToken, authSkipSKToken:
		valid := authToken(char)
		if r.state == authSkipSKToken {
			valid = authWord(char) || char == '-'
		}
		if valid {
			r.previousWord = authWord(char)
			return
		}
		r.state = authSearch
		r.writeByte(char, output)
	case authNeedSKToken:
		if authWord(char) || char == '-' {
			r.pending += string(char)
			if len(r.pending) == len("sk-")+minSKTokenChars {
				output.WriteString(redactedText)
				r.pending = ""
				r.state = authSkipSKToken
				r.previousWord = authWord(char)
			}
			return
		}
		r.releasePrefix(output)
		r.writeByte(char, output)
	default:
		r.writePrefix(char, output)
	}
}

// writePrefix 只在词边界尝试匹配短标记，普通文本可立即写出。
func (r *authRedactor) writePrefix(char byte, output *strings.Builder) {
	if r.pending == "" {
		if r.previousWord || !strings.ContainsRune("bBsS", rune(char)) {
			output.WriteByte(char)
			r.previousWord = authWord(char)
			return
		}
	}
	candidate := r.pending + string(char)
	lower := strings.ToLower(candidate)
	for _, prefix := range []string{
		"bearer",
		"basic",
		"sk-",
	} {
		if !strings.HasPrefix(prefix, lower) {
			continue
		}
		r.pending = candidate
		if lower == prefix {
			r.state = authNeedSpace
			if prefix == "sk-" {
				r.state = authNeedSKToken
			}
		}
		return
	}
	r.releasePrefix(output)
	r.writeByte(char, output)
}

// writeValueStart 暂存短鉴权前缀；确认值后立即输出遮盖标记。
func (r *authRedactor) writeValueStart(char byte, output *strings.Builder) {
	if authSpace(char) {
		r.pending += string(char)
		r.state = authNeedToken
		if len(r.pending) >= maxAuthPrefixBytes {
			output.WriteString(redactedText)
			r.pending = ""
			r.previousWord = false
			r.state = authSkipSpace
		}
		return
	}
	if r.state == authNeedToken && authToken(char) {
		output.WriteString(redactedText)
		r.pending = ""
		r.state = authSkipToken
		r.previousWord = authWord(char)
		return
	}
	r.releasePrefix(output)
	r.writeByte(char, output)
}

// releasePrefix 写出未构成鉴权文本的前缀，保留原始词边界。
func (r *authRedactor) releasePrefix(output *strings.Builder) {
	output.WriteString(r.pending)
	if r.pending != "" {
		r.previousWord = authWord(r.pending[len(r.pending)-1])
	}
	r.pending = ""
	r.state = authSearch
}

// authWord 与正则表达式的 ASCII 单词字符定义一致。
func authWord(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_'
}

// authSpace 与普通鉴权正则使用的空白字符定义一致。
func authSpace(char byte) bool {
	return strings.ContainsRune(" \t\n\r\f", rune(char))
}

// authToken 判断 Bearer、Basic 值允许的协议字符。
func authToken(char byte) bool {
	return authWord(char) || strings.ContainsRune(".~+/=-", rune(char))
}
