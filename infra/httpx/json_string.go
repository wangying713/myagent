package httpx

import (
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"
)

// decodeJSONString 拒绝无效 UTF-8 和未配对的 UTF-16 转义，避免默默替换用户问题。
func decodeJSONString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	invalid := errors.New("请求字符串包含无效 Unicode")
	if !utf8.Valid(raw) {
		return "", invalid
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		code, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return "", err
		}
		i += 4
		if code < 0xd800 || code > 0xdfff {
			continue
		}
		if code > 0xdbff || i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return "", invalid
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return "", invalid
		}
		i += 6
	}
	return value, nil
}
