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
// 限制请求正文大小，拒绝顶层重复字段、null、未知字段和字段名别名。
// dst 必须是非 nil 消息指针；解码失败时调用方应丢弃其中可能已经写入的字段。
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
