package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// jsonFields 从显式 JSON 标签提取字段名，避免维护另一份允许字段列表。
// 请求类型只接受导出且不匿名的字段，未设置标签及重复标签均视为定义错误。
func jsonFields(target reflect.Type) ([]string, error) {
	names := make([]string, 0, target.NumField())
	seen := make(map[string]bool)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if field.PkgPath != "" || field.Anonymous || name == "" || seen[name] {
			return nil, errors.New("请求字段必须使用不重复的显式 JSON 标签")
		}
		names = append(names, name)
		seen[name] = true
	}
	return names, nil
}

// decodeJSONFields 按准确标签解码字段，保留切片省略时的 nil 与显式空数组。
func decodeJSONFields(values map[string]json.RawMessage, target reflect.Value) error {
	for i := 0; i < target.NumField(); i++ {
		name := strings.Split(target.Type().Field(i).Tag.Get("json"), ",")[0]
		if raw, ok := values[name]; ok {
			if err := decodeJSONValue(raw, target.Field(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeJSONValue 对嵌套对象沿用严格字段规则，数组元素同样不能为 null。
func decodeJSONValue(raw json.RawMessage, target reflect.Value) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("请求字段和数组元素不能为 null")
	}
	switch target.Kind() {
	case reflect.Pointer:
		target.Set(reflect.New(target.Type().Elem()))
		return decodeJSONValue(raw, target.Elem())
	case reflect.Struct:
		names, err := jsonFields(target.Type())
		if err != nil {
			return err
		}
		values, err := decodeObject(bytes.NewReader(raw), names)
		if err != nil {
			return err
		}
		return decodeJSONFields(values, target)
	case reflect.Slice:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		target.Set(reflect.MakeSlice(target.Type(), len(items), len(items)))
		for i, item := range items {
			if err := decodeJSONValue(item, target.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := decodeJSONInteger(raw, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetInt(value)
		return nil
	case reflect.String:
		value, err := decodeJSONString(raw)
		if err != nil {
			return err
		}
		target.SetString(value)
		return nil
	default:
		return json.Unmarshal(raw, target.Addr().Interface())
	}
}
