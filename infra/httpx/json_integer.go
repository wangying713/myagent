package httpx

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// decodeJSONInteger 保留已有整数输入规则：接受数字、数字字符串和整数值指数形式。
// 只做十进制字符串运算，避免浮点舍入和超大指数引起的内存分配。
func decodeJSONInteger(raw json.RawMessage, bits int) (int64, error) {
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, err
	}
	text := number.String()
	if text == "" || !json.Valid([]byte(text)) {
		return 0, errors.New("请求整数格式无效")
	}
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(text), "e")
	fraction := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fraction = len(mantissa) - dot - 1
		mantissa = mantissa[:dot] + mantissa[dot+1:]
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return 0, nil
	}
	scale := int64(0)
	if hasExponent {
		value, err := strconv.ParseInt(exponent, 10, 32)
		if err != nil {
			return 0, err
		}
		scale = value
	}
	scale -= int64(fraction)
	if scale < 0 {
		end := int64(len(digits)) + scale
		if end <= 0 || strings.Trim(digits[end:], "0") != "" {
			return 0, errors.New("请求整数不能包含非零小数部分")
		}
		digits = digits[:end]
	} else {
		if int64(len(digits))+scale > 19 {
			return 0, errors.New("请求整数超出范围")
		}
		digits += strings.Repeat("0", int(scale))
	}
	if negative {
		digits = "-" + digits
	}
	return strconv.ParseInt(digits, 10, bits)
}
