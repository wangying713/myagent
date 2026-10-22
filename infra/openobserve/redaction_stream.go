package openobserve

import "strings"

// TextRedactor 为一次文本流保存匹配状态，不可跨请求共享。
// 暂存大小由配置中的密钥长度及固定协议前缀决定，不随回答长度增长。
type TextRedactor struct {
	literals []literalRedactor // 按普通脱敏的优先顺序处理配置密钥。
	auth     authRedactor      // 常见鉴权文本的增量匹配状态。
	json     jsonRedactor      // 敏感 JSON 字段的增量匹配状态。
}

// literalRedactor 只保留尚未匹配完成的固定密钥前缀。
type literalRedactor struct {
	secret  string // 当前阶段负责遮盖的非空固定密钥。
	pending string // 与密钥开头重合的尾部，长度小于密钥字节数。
}

// NewTextRedactor 创建请求独占的增量脱敏器，沿用普通展示文本的处理顺序。
func (c *Client) NewTextRedactor() *TextRedactor {
	redactor := &TextRedactor{}
	for _, secret := range c.secrets {
		redactor.literals = append(redactor.literals, literalRedactor{
			secret: secret,
		})
	}
	return redactor
}

// Write 逐阶段处理新片段，只暂存尚未确认的密钥或协议前缀。
func (r *TextRedactor) Write(text string) string {
	for i := range r.literals {
		text = r.literals[i].Write(text)
	}
	return r.json.Write(r.auth.Write(text))
}

// Flush 在结束时顺序刷新各阶段；消费者取消后不再调用。
func (r *TextRedactor) Flush() string {
	var text string
	for i := range r.literals {
		text = r.literals[i].Write(text) + r.literals[i].Flush()
	}
	text = r.auth.Write(text) + r.auth.Flush()
	return r.json.Write(text) + r.json.Flush()
}

// Write 替换完整密钥，只留下最长的待匹配前缀。
func (r *literalRedactor) Write(text string) string {
	text = r.pending + text
	var output strings.Builder
	for {
		index := strings.Index(text, r.secret)
		if index < 0 {
			break
		}
		output.WriteString(text[:index])
		output.WriteString(redactedText)
		text = text[index+len(r.secret):]
	}
	end := len(text)
	for length := min(len(text), len(r.secret)-1); length > 0; length-- {
		if strings.HasSuffix(text, r.secret[:length]) {
			end -= length
			break
		}
	}
	output.WriteString(text[:end])
	r.pending = strings.Clone(text[end:])
	return output.String()
}

// Flush 输出最终未构成密钥的尾部，并释放暂存内容。
func (r *literalRedactor) Flush() string {
	text := r.pending
	r.pending = ""
	return text
}
