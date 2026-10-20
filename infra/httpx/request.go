package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"slices"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes 限制整个 JSON 请求正文的字节数，包含 JSON 后的尾随空白。
const MaxBodyBytes = 32 << 10

// DecodeJSON 根据结构体的 JSON 标签严格解码单个请求对象，每次请求使用独立目标。
// 拒绝未知字段、重复字段、字段别名和 null，保留省略切片与显式空数组的区别。
// dst 必须是非 nil 结构体指针；解码失败时调用方应丢弃目标中的部分数据。
func DecodeJSON(c *gin.Context, dst any) error {
	target := reflect.ValueOf(dst)
	if target.Kind() != reflect.Pointer || target.IsNil() || target.Elem().Kind() != reflect.Struct {
		return requestError(errors.New("解码目标必须是非 nil 结构体指针"))
	}
	target = target.Elem()
	names, err := jsonFields(target.Type())
	if err != nil {
		return requestError(err)
	}
	values, err := readJSONObject(c, names)
	if err != nil {
		return err
	}
	if err := decodeJSONFields(values, target); err != nil {
		return requestError(err)
	}
	return nil
}

func readJSONObject(c *gin.Context, fields []string) (map[string]json.RawMessage, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, &RequestError{
			Status: http.StatusUnsupportedMediaType,
			Detail: Error{
				Code:    "invalid_content_type",
				Message: "请使用 application/json",
			},
		}
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
