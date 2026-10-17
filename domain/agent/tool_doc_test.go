package agent

import (
	"strings"
	"testing"
)

func TestToolDescriptionReadsMethodDocumentation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "值接收者",
			source: `package sample
type Tools struct{}
// multiply 计算两个数的乘积。
func (Tools) multiply() {}
`,
			want: "计算两个数的乘积。",
		},
		{
			name: "区分同名函数和其他接收者并保留段落",
			source: `package sample
type Tools struct{}
type Other struct{}
// multiply 这是普通函数。
func multiply() {}
// multiply 这是其他接收者的方法。
func (Other) multiply() {}
// multiply 计算两个数的乘积。
// 涉及乘法时使用。
//
// 参数必须是有限数值。
func (*Tools) multiply() {}
`,
			want: "计算两个数的乘积。\n涉及乘法时使用。\n\n参数必须是有限数值。",
		},
		{
			name: "没有标识符前缀的正文",
			source: `package sample
type Tools struct{}
// 计算两个数的乘积。
func (Tools) multiply() {}
`,
			want: "计算两个数的乘积。",
		},
		{
			name: "标识符独占首行",
			source: `package sample
type Tools struct{}
// multiply
// 计算两个数的乘积。
func (Tools) multiply() {}
`,
			want: "计算两个数的乘积。",
		},
		{
			name: "不截断正文中的相似前缀",
			source: `package sample
type Tools struct{}
// multiplyValues 表示另一种描述写法。
func (Tools) multiply() {}
`,
			want: "multiplyValues 表示另一种描述写法。",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toolDescription(tc.source, "Tools", "multiply")
			if err != nil {
				t.Fatalf("读取方法文档: %v", err)
			}
			if got != tc.want {
				t.Errorf("方法文档为 %q，预期为 %q", got, tc.want)
			}
		})
	}
}

func TestToolDescriptionRejectsInvalidDocumentation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name:   "无效源码",
			source: "package",
		},
		{
			name: "只有同名普通函数",
			source: `package sample
// multiply 计算乘积。
func multiply() {}
`,
		},
		{
			name: "只有其他接收者的同名方法",
			source: `package sample
type Other struct{}
// multiply 计算乘积。
func (Other) multiply() {}
`,
		},
		{
			name: "缺少方法注释",
			source: `package sample
type Tools struct{}
func (Tools) multiply() {}
`,
		},
		{
			name: "只有实现注释",
			source: `package sample
type Tools struct{}
func (Tools) multiply() {
    // 计算乘积。
}
`,
		},
		{
			name: "注释没有正文",
			source: `package sample
type Tools struct{}
// multiply
func (Tools) multiply() {}
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toolDescription(tc.source, "Tools", "multiply")
			if err == nil || got != "" {
				t.Fatalf("无效文档返回 %q/%v，预期返回错误且无描述", got, err)
			}
			if !strings.Contains(err.Error(), "Tools.multiply") {
				t.Errorf("错误 %q 未标明目标方法 Tools.multiply", err)
			}
		})
	}
}

func TestMultiplyToolDescriptionDoesNotDependOnWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	definition := (MultiplyTools{}).Definitions()[0]
	if definition.Function.Description != "计算两个数的乘积。涉及乘法时使用。" {
		t.Errorf("切换目录后的工具描述为 %q，预期从嵌入源码读取乘法说明", definition.Function.Description)
	}
}
