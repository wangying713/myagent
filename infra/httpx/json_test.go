package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

func TestDecodeJSONContract(t *testing.T) {
	router := newDecodeRouter(t)
	for _, tc := range decodeCases() {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/types", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, request)
			if out.Code != tc.status || out.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("输入 %s 实际 HTTP %d、正文 %s，预期 HTTP %d 且禁止缓存", tc.body, out.Code, out.Body.String(), tc.status)
			}
			if tc.code != "" {
				var response testRsp[any]
				if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || response.Error == nil || response.Error.Code != tc.code {
					t.Fatalf("实际响应 %s，预期错误码 %s", out.Body.String(), tc.code)
				}
				return
			}
			var response struct {
				Name    string   `json:"name"`    // 用于验证解码结果的名称。
				Labels  []string `json:"labels"`  // 用于验证数组字段解码结果的名称列表。
				Omitted bool     `json:"omitted"` // 问题列表是否省略，显式空数组为 false。
			}
			if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if tc.name == "正常字段" && (response.Name != "类型" || len(response.Labels) != 1 || response.Labels[0] != "甲" || response.Omitted) {
				t.Errorf("实际解码数据 %+v，预期保留字段值与存在性", response)
			}
			if tc.name == "省略数组" && !response.Omitted {
				t.Error("省略数组被误认为已提供")
			}
			if tc.name == "显式空数组" && (response.Omitted || len(response.Labels) != 0) {
				t.Error("显式空数组丢失存在性或生成了元素")
			}
		})
	}
}

func TestDecodeJSONUnicode(t *testing.T) {
	router := newDecodeRouter(t)
	for _, tc := range []struct {
		name  string // 场景名称。
		input string // name 字段的原始 JSON 字符串。
		want  string // 预期解码文字；空字符串表示拒绝输入。
	}{
		{
			name:  "合法代理对",
			input: `"\ud83d\ude00"`,
			want:  "😀",
		},
		{
			name:  "保留转义文本",
			input: `"\\ud800"`,
			want:  `\ud800`,
		},
		{
			name:  "孤立高代理",
			input: `"\ud800"`,
		},
		{
			name:  "孤立低代理",
			input: `"\udc00"`,
		},
		{
			name:  "错误代理对",
			input: `"\ud800\u0041"`,
		},
		{
			name:  "无效 UTF-8",
			input: "\"" + string([]byte{0xff}) + "\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/types", strings.NewReader(`{"name":`+tc.input+`}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if tc.want == "" {
				if out.Code != http.StatusBadRequest {
					t.Errorf("无效 Unicode 实际 HTTP %d，预期 400", out.Code)
				}
				return
			}
			var rsp decodeRequest
			if err := json.Unmarshal(out.Body.Bytes(), &rsp); err != nil {
				t.Fatal(err)
			}
			if out.Code != http.StatusOK || rsp.Name != tc.want {
				t.Errorf("实际 HTTP %d、文字 %q，预期 HTTP 200、文字 %q", out.Code, rsp.Name, tc.want)
			}
		})
	}
}

func TestDecodeJSONRejectsInvalidTargets(t *testing.T) {
	// 运行时构造冲突标签，避免测试样本自身被 vet 当作业务定义错误。
	duplicate := reflect.StructOf([]reflect.StructField{
		{
			Name: "First",
			Type: reflect.TypeFor[string](),
			Tag:  `json:"name"`,
		},
		{
			Name: "Second",
			Type: reflect.TypeFor[string](),
			Tag:  `json:"name"`,
		},
	})
	for _, target := range []any{
		nil,
		decodeRequest{},
		(*decodeRequest)(nil),
		&struct {
			Name string // 缺少显式 JSON 标签的字段。
		}{},
		reflect.New(duplicate).Interface(),
	} {
		out := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(out)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/types", strings.NewReader(`{}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		if err := httpx.DecodeJSON(ctx, target); err == nil {
			t.Errorf("无效解码目标 %T 未被拒绝", target)
		}
	}
}

// decodeRequest 验证严格解码不依赖应用层数据类型。
type decodeRequest struct {
	Name          string        `json:"name"`           // 普通字符串输入。
	Labels        []string      `json:"labels"`         // 列表输入；nil 表示省略，空切片表示显式空数组。
	SourceContext *decodeSource `json:"source_context"` // 嵌套对象；nil 表示省略。
}

// decodeSource 是严格嵌套对象的测试数据。
type decodeSource struct {
	FileName string `json:"file_name"` // 用于测试准确字段名的文件名。
}

// decodeCase 保存 HTTP 请求与预期结果。
type decodeCase struct {
	name        string // 场景名称。
	body        string // 实际请求正文。
	contentType string // 实际 Content-Type，空字符串表示缺失。
	code        string // 预期错误码；空字符串表示不检查错误码。
	status      int    // 预期 HTTP 状态码。
}

func newDecodeRouter(t *testing.T) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.POST("/types", func(c *gin.Context) {
		var request decodeRequest
		if err := httpx.DecodeJSON(c, &request); err != nil {
			writeTestReqError(c, err, nil)
			return
		}
		if err := httpx.Write(c, http.StatusOK, struct {
			Name    string   `json:"name"`    // 用于验证解码结果的名称。
			Labels  []string `json:"labels"`  // 用于验证数组字段解码结果的名称列表。
			Omitted bool     `json:"omitted"` // 问题列表是否省略，显式空数组为 false。
		}{
			Name:    request.Name,
			Labels:  request.Labels,
			Omitted: request.Labels == nil,
		}); err != nil {
			t.Error(err)
		}
	})
	return router
}

func decodeCases() []decodeCase {
	var cases []decodeCase
	cases = append(cases, validAndContentTypeCases()...)
	cases = append(cases, objectAndFieldCases()...)
	cases = append(cases, typeAndBodyLimitCases()...)
	cases = append(cases, nestedObjectCases()...)
	return cases
}

func nestedObjectCases() []decodeCase {
	return []decodeCase{
		{
			name:        "嵌套字段正常",
			body:        `{"source_context":{"file_name":"资料"}}`,
			contentType: "application/json",
			status:      200,
		},
		{
			name:        "嵌套字段重复",
			body:        `{"source_context":{"file_name":"甲","file_name":"乙"}}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "嵌套未知字段",
			body:        `{"source_context":{"other":"甲"}}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "嵌套字段别名",
			body:        `{"source_context":{"File_Name":"甲"}}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "嵌套字段 null",
			body:        `{"source_context":{"file_name":null}}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
	}
}

func validAndContentTypeCases() []decodeCase {
	return []decodeCase{
		{
			name:        "正常字段",
			body:        `{"name":"类型","labels":["甲"]}`,
			contentType: "application/json; charset=utf-8",
			status:      200,
		},
		{
			name:        "省略数组",
			body:        `{}`,
			contentType: "application/json",
			status:      200,
		},
		{
			name:        "显式空数组",
			body:        `{"labels":[]}`,
			contentType: "application/json",
			status:      200,
		},
		{
			name:   "缺少类型",
			body:   `{}`,
			status: 415,
			code:   "invalid_content_type",
		},
		{
			name:        "非 JSON 类型",
			body:        `{}`,
			contentType: "text/plain",
			status:      415,
			code:        "invalid_content_type",
		},
		{
			name:        "根 null",
			body:        `null`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "不完整对象",
			body:        `{`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
	}
}
func objectAndFieldCases() []decodeCase {
	return []decodeCase{
		{
			name:        "数组",
			body:        `[]`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "字段 null",
			body:        `{"name":null}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "数组元素 null",
			body:        `{"labels":[null]}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "重复字段",
			body:        `{"name":"甲","name":"乙"}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "大小写别名",
			body:        `{"Name":"类型"}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "驼峰别名",
			body:        `{"sourceContext":{}}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "原始字段名",
			body:        `{"source_context":{}}`,
			contentType: "application/json",
			status:      200,
		},
	}
}
func typeAndBodyLimitCases() []decodeCase {
	return []decodeCase{
		{
			name:        "未知字段",
			body:        `{"other":1}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "字符串错误类型",
			body:        `{"name":1}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "数组错误类型",
			body:        `{"labels":"甲"}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "多个对象",
			body:        `{} {}`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "无效尾部",
			body:        `{} trailing`,
			contentType: "application/json",
			status:      400,
			code:        "invalid_input",
		},
		{
			name:        "正文超限",
			body:        `{"name":"` + strings.Repeat("a", httpx.MaxBodyBytes) + `"}`,
			contentType: "application/json",
			status:      413,
			code:        "request_too_large",
		},
		{
			name:        "尾随空白超限",
			body:        `{}` + strings.Repeat(" ", httpx.MaxBodyBytes),
			contentType: "application/json",
			status:      413,
			code:        "request_too_large",
		},
	}
}
