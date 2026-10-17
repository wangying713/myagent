package agent

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMultiplyToolsDefinitionsPreserveSchema(t *testing.T) {
	definitions := (MultiplyTools{}).Definitions()
	if len(definitions) != 1 {
		t.Fatalf("工具定义数量为 %d，预期为 1", len(definitions))
	}
	definition := definitions[0]
	if definition.Type != "function" || definition.Function.Name != "multiply" {
		t.Errorf("工具协议为 %+v，预期为 function/multiply", definition)
	}
	if definition.Function.Description != "计算两个数的乘积。涉及乘法时使用。" {
		t.Errorf("工具描述为 %q，预期保留原有乘法能力说明", definition.Function.Description)
	}
	want := `{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"],"additionalProperties":false}`
	var gotSchema, wantSchema any
	if err := json.Unmarshal(definition.Function.Parameters, &gotSchema); err != nil {
		t.Fatalf("解码乘法参数 Schema: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantSchema); err != nil {
		t.Fatalf("解码预期 Schema: %v", err)
	}
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Errorf("乘法参数 Schema 为 %s，预期为 %s", definition.Function.Parameters, want)
	}

	// 调用方修改定义后，后续请求仍应获得独立且有效的参数声明。
	definition.Function.Parameters[0] = '!'
	next := (MultiplyTools{}).Definitions()
	var nextSchema any
	if err := json.Unmarshal(next[0].Function.Parameters, &nextSchema); err != nil {
		t.Fatalf("后续定义的 Schema 被修改: %v", err)
	}
	if !reflect.DeepEqual(nextSchema, wantSchema) {
		t.Errorf("后续参数 Schema 为 %s，预期为 %s", next[0].Function.Parameters, want)
	}
}

func TestToolParametersMapScalarFields(t *testing.T) {
	// args 覆盖工具参数 Schema 支持和忽略的标量字段。
	type args struct {
		Text    string  `json:"text"`    // 验证字符串参数映射。
		Enabled bool    `json:"enabled"` // 验证布尔参数映射。
		Count   int64   `json:"count"`   // 验证有符号整数参数映射。
		Size    uint32  `json:"size"`    // 验证无符号整数参数映射。
		Ratio   float32 `json:"ratio"`   // 验证浮点参数映射。
		Hidden  string  `json:"-"`       // 验证 json:"-" 字段被跳过。
		private string  // 验证未导出字段被跳过。
	}
	parameters, err := toolParameters(reflect.TypeFor[args]())
	if err != nil {
		t.Fatalf("映射标量参数: %v", err)
	}
	var got struct {
		Type       string `json:"type"` // JSON Schema 声明的数据类型。
		Properties map[string]struct {
			Type string `json:"type"` // JSON Schema 声明的数据类型。
		} `json:"properties"` // 参数名称与对应的数据类型声明。
		Required             []string `json:"required"`             // 必须提供的参数名称列表。
		AdditionalProperties *bool    `json:"additionalProperties"` // 是否允许未声明的额外字段。
	}
	if err := json.Unmarshal(parameters, &got); err != nil {
		t.Fatalf("解码标量参数 Schema: %v", err)
	}
	if got.Type != "object" || got.AdditionalProperties == nil || *got.AdditionalProperties {
		t.Errorf("对象约束为 %s，预期禁止额外字段", parameters)
	}
	wantTypes := map[string]string{
		"text":    "string",
		"enabled": "boolean",
		"count":   "integer",
		"size":    "integer",
		"ratio":   "number",
	}
	if len(got.Properties) != len(wantTypes) {
		t.Fatalf("属性数量为 %d，预期为 %d", len(got.Properties), len(wantTypes))
	}
	for name, want := range wantTypes {
		if got.Properties[name].Type != want {
			t.Errorf("参数 %q 类型为 %q，预期为 %q", name, got.Properties[name].Type, want)
		}
	}
	wantRequired := []string{
		"text",
		"enabled",
		"count",
		"size",
		"ratio",
	}
	if !reflect.DeepEqual(got.Required, wantRequired) {
		t.Errorf("必填参数为 %v，预期为 %v", got.Required, wantRequired)
	}
}

func TestToolParametersRejectUnsupportedDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name   string       // 用例名称。
		typeOf reflect.Type // 用于验证参数定义是否合法的 Go 类型。
	}{
		{
			name:   "非结构体",
			typeOf: reflect.TypeFor[float64](),
		},
		{
			name: "缺少 json 名称",
			typeOf: reflect.TypeFor[struct {
				Value float64 // 验证缺少显式 json 名称的字段会被拒绝。
			}](),
		},
		{
			name: "可选标签",
			typeOf: reflect.TypeFor[struct {
				Value float64 `json:"value,omitempty"` // 验证不支持的可选标签会被拒绝。
			}](),
		},
		{
			name: "重复名称",
			typeOf: reflect.StructOf([]reflect.StructField{
				{
					Name: "First",
					Type: reflect.TypeFor[float64](),
					Tag:  `json:"value"`,
				},
				{
					Name: "Second",
					Type: reflect.TypeFor[float64](),
					Tag:  `json:"value"`,
				},
			}),
		},
		{
			name: "JSON 数字不能按底层字符串映射",
			typeOf: reflect.TypeFor[struct {
				Value json.Number `json:"value"` // 验证具名类型不能按底层字符串直接映射。
			}](),
		},
		{
			name: "嵌入字段",
			typeOf: reflect.TypeFor[struct {
				MultiplyTools `json:"tools"` // 用于验证嵌入字段会被拒绝。
			}](),
		},
		{
			name: "嵌套对象",
			typeOf: reflect.TypeFor[struct {
				Value multiplyArgs `json:"value"` // 验证嵌套对象参数会被拒绝。
			}](),
		},
		{
			name: "可空指针",
			typeOf: reflect.TypeFor[struct {
				Value *float64 `json:"value"` // 验证可空指针参数会被拒绝。
			}](),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parameters, err := toolParameters(tc.typeOf)
			if err == nil || parameters != nil {
				t.Fatalf("不支持的定义返回 %s/%v，预期返回错误且无 Schema", parameters, err)
			}
		})
	}
}
