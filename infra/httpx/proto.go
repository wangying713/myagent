package httpx

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// DecodeProtoJSON 按生成消息的原始字段名严格解析 HTTP JSON。
// 返回实际出现的顶层字段，供调用方区分 repeated 字段的省略与显式空数组。
// 正文限制、重复字段、null 和名称校验与 DecodeJSON 一致，不接受字段名别名。
func DecodeProtoJSON(c *gin.Context, dst proto.Message) (map[string]bool, error) {
	if dst == nil || !dst.ProtoReflect().IsValid() {
		return nil, requestError(errors.New("目标消息不能为空"))
	}
	fields := dst.ProtoReflect().Descriptor().Fields()
	names := make([]string, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		names = append(names, string(fields.Get(i).Name()))
	}
	values, err := readJSONObject(c, names)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, requestError(err)
	}
	if err := protojson.Unmarshal(raw, dst); err != nil {
		return nil, requestError(err)
	}
	present := make(map[string]bool, len(values))
	for name := range values {
		present[name] = true
	}
	return present, nil
}
