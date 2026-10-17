package agent

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// multiplyToolSource 保留实际参与构建的工具源码，运行时无需访问源码路径。
//
//go:embed tools.go
var multiplyToolSource string

// mustToolDescription 用于显式注册的具体方法，源码或文档错误属于编程错误。
func mustToolDescription[T any](source, method string) string {
	receiver := reflect.TypeFor[T]().Name()
	description, err := toolDescription(source, receiver, method)
	if err != nil {
		panic(err)
	}
	return description
}

// toolDescription 读取指定接收者方法的文档注释，兼容值接收者和指针接收者。
// 只去掉开头的 Go 方法标识符，保留原注释正文及段落，不改写业务语义。
func toolDescription(source, receiver, method string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "tools.go", source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("解析工具方法 %s.%s 的源码: %w", receiver, method, err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != method || function.Recv == nil {
			continue
		}
		receiverType := function.Recv.List[0].Type
		if pointer, ok := receiverType.(*ast.StarExpr); ok {
			receiverType = pointer.X
		}
		identifier, ok := receiverType.(*ast.Ident)
		if !ok || identifier.Name != receiver {
			continue
		}
		description := strings.TrimSpace(function.Doc.Text())
		if description == method || strings.HasPrefix(description, method+" ") || strings.HasPrefix(description, method+"\n") {
			description = strings.TrimSpace(description[len(method):])
		}
		if description == "" {
			return "", fmt.Errorf("工具方法 %s.%s 缺少文档注释正文", receiver, method)
		}
		return description, nil
	}
	return "", fmt.Errorf("源码中找不到工具方法 %s.%s", receiver, method)
}
