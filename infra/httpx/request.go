package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

const MaxBodyBytes = 32 << 10

// DecodeJSON 只接收单个 JSON 对象，顶层拒绝重复字段、未知字段、大小写别名和 null。
// fields 由具体接口显式声明；本包不认识任何业务字段，也不校验业务语义。
func DecodeJSON(c *gin.Context, dst any, fields ...string) error {
	values, err := readJSONObject(c, fields)
	if err != nil {
		return err
	}
	// 按目标请求类型验证字段值；前面的 token 检查补足标准解码器的宽松行为。
	raw, err := json.Marshal(values)
	if err != nil {
		return requestError(err)
	}
	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.DisallowUnknownFields()
	if err := typed.Decode(dst); err != nil {
		return requestError(err)
	}
	return nil
}

func readJSONObject(c *gin.Context, fields []string) (map[string]json.RawMessage, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, &RequestError{Status: http.StatusUnsupportedMediaType, Detail: Error{Code: "invalid_content_type", Message: "请使用 application/json"}}
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	defer body.Close()
	values, err := decodeObject(body, fields)
	if err != nil {
		return nil, requestError(err)
	}
	return values, nil
}

func decodeObject(body io.Reader, fields []string) (map[string]json.RawMessage, error) {
	invalid := errors.New("JSON 请求结构无效")
	d := json.NewDecoder(body)
	start, err := d.Token()
	if err != nil {
		return nil, err
	}
	if start != json.Delim('{') {
		return nil, invalid
	}
	values := make(map[string]json.RawMessage)
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok || !slices.Contains(fields, name) {
			return nil, invalid
		}
		if _, exists := values[name]; exists {
			return nil, invalid
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, invalid
		}
		values[name] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		if err == nil {
			return nil, invalid
		}
		return nil, err
	}
	return values, nil
}
