package openobserve

import (
	"encoding/json"
	"strconv"
	"strings"
)

// textStreamEvent 保存展示用 SSE 事件及其中可增量脱敏的文本字段。
type textStreamEvent struct {
	payload any            // 解码后的完整 JSON 事件，保留协议中的 null。
	text    map[string]any // 保存文本字段的对象；无文本事件为 nil。
	key     string         // 文本字段名，模型为 content，HTTP 事件为 delta。
	done    bool           // 是否为独立的 [DONE] 协议标记。
}

// SafeSSE 显式处理 SSE 展示副本，textPath 指定增量文本在事件 JSON 中的位置。
// 路径以点分隔，对象使用字段名，数组使用从零开始的下标，例如 choices.0.delta.content。
// 残缺事件无法可靠解码时遮盖整份预览，不影响实际传输或领域记录原文。
func (c *Client) SafeSSE(raw []byte, textPath string) string {
	text := string(raw)
	path := strings.Split(textPath, ".")
	redactor := c.NewTextRedactor()
	var events []*textStreamEvent
	var lastText *textStreamEvent
	for _, frame := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		var lines []string
		for _, line := range strings.Split(frame, "\n") {
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				lines = append(lines, strings.TrimPrefix(data, " "))
			}
		}
		if len(lines) == 0 {
			continue
		}
		data := strings.Join(lines, "\n")
		event := &textStreamEvent{
			done: data == "[DONE]",
		}
		if !event.done {
			var ok bool
			event.payload, ok = decodeJSON(data)
			if !ok {
				return redactedText
			}
			event.text, event.key = streamTextField(event.payload, path)
			if event.text != nil {
				event.text[event.key] = redactor.Write(event.text[event.key].(string))
				lastText = event
			}
		}
		events = append(events, event)
	}
	if lastText != nil {
		lastText.text[lastText.key] = lastText.text[lastText.key].(string) + redactor.Flush()
	}
	var output strings.Builder
	for _, event := range events {
		data := "[DONE]"
		if !event.done {
			encoded, _ := json.Marshal(c.clean(event.payload, 0))
			data = string(encoded)
		}
		output.WriteString("data: " + data + "\n\n")
	}
	return c.Safe([]byte(output.String()))
}

// streamTextField 只按调用方给出的路径查找文本，不推断模型或 HTTP 响应结构。
func streamTextField(payload any, path []string) (map[string]any, string) {
	for i, key := range path {
		switch value := payload.(type) {
		case map[string]any:
			if i == len(path)-1 {
				if _, ok := value[key].(string); ok {
					return value, key
				}
			}
			payload = value[key]
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(value) {
				return nil, ""
			}
			payload = value[index]
		default:
			return nil, ""
		}
	}
	return nil, ""
}
