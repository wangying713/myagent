package agent

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// toolParameterSchema 只承载当前工具参数需要的 JSON Schema 字段。
type toolParameterSchema struct {
	Type                 string                         `json:"type"`                           // JSON Schema 数据类型。
	Properties           map[string]toolParameterSchema `json:"properties,omitempty"`           // 对象属性名及其参数约束。
	Required             []string                       `json:"required,omitempty"`             // 必须提供的参数名称列表。
	AdditionalProperties *bool                          `json:"additionalProperties,omitempty"` // 是否允许额外字段，nil 表示不声明此约束。
}

// mustToolParameters 用于代码中确定的参数类型；定义错误属于编程错误，立即中断。
// 每次调用独立映射和编码，避免工具定义之间共享可修改的 JSON 数据。
func mustToolParameters[T any]() json.RawMessage {
	parameters, err := toolParameters(reflect.TypeFor[T]())
	if err != nil {
		panic(err)
	}
	return parameters
}

// toolParameters 将扁平结构体映射为严格对象 Schema，所有参数均为必填。
// 仅支持显式 json 名称的内置标量字段；跳过未导出和 json:"-" 字段。
// 嵌入字段、标签选项及其他类型显式报错，避免声明出与 Go 参数不一致的协议。
func toolParameters(t reflect.Type) (json.RawMessage, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("工具参数 %s 必须是结构体", t)
	}
	additionalProperties := false
	schema := toolParameterSchema{
		Type:                 "object",
		Properties:           make(map[string]toolParameterSchema),
		AdditionalProperties: &additionalProperties,
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() || field.Tag.Get("json") == "-" {
			continue
		}
		name := field.Tag.Get("json")
		if field.Anonymous || name == "" || strings.Contains(name, ",") {
			return nil, fmt.Errorf("工具参数字段 %s.%s 必须显式声明不带选项的 json 名称", t, field.Name)
		}
		if _, exists := schema.Properties[name]; exists {
			return nil, fmt.Errorf("工具参数 %s 的 json 名称 %q 重复", t, name)
		}
		// 具名类型可能有自定义 JSON 表现，不能只根据底层 Kind 推断协议。
		if field.Type.PkgPath() != "" {
			return nil, fmt.Errorf("工具参数字段 %s.%s 不支持具名类型 %s", t, field.Name, field.Type)
		}
		var property toolParameterSchema
		switch field.Type.Kind() {
		case reflect.String:
			property.Type = "string"
		case reflect.Bool:
			property.Type = "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			property.Type = "integer"
		case reflect.Float32, reflect.Float64:
			property.Type = "number"
		default:
			return nil, fmt.Errorf("工具参数字段 %s.%s 不支持类型 %s", t, field.Name, field.Type)
		}
		schema.Properties[name] = property
		schema.Required = append(schema.Required, name)
	}
	parameters, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("编码工具参数 %s 的 JSON Schema: %w", t, err)
	}
	return parameters, nil
}
