package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/typepb"
)

func TestDecodeProtoJSONContract(t *testing.T) {
	// 使用标准生成消息验证复用，通用包不依赖 Agent 的 proto。
	router := gin.New()
	router.POST("/types", func(c *gin.Context) {
		var request typepb.Type
		present, err := httpx.DecodeProtoJSON(c, &request)
		if err != nil {
			writeTestReqError(c, err, nil)
			return
		}
		if err := httpx.Write(c, http.StatusOK, struct {
			Name    string          `json:"name"`
			Oneofs  []string        `json:"oneofs"`
			Present map[string]bool `json:"present"`
		}{
			Name:    request.GetName(),
			Oneofs:  request.GetOneofs(),
			Present: present,
		}); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		name, body, contentType, code string
		status                        int
	}{
		{
			name:        "正常字段",
			body:        `{"name":"类型","oneofs":["甲"]}`,
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
			body:        `{"oneofs":[]}`,
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
			body:        `{"oneofs":[null]}`,
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
			body:        `{"oneofs":"甲"}`,
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
	} {
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
				Name    string          `json:"name"`
				Oneofs  []string        `json:"oneofs"`
				Present map[string]bool `json:"present"`
			}
			if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if tc.name == "正常字段" && (response.Name != "类型" || len(response.Oneofs) != 1 || response.Oneofs[0] != "甲" || !response.Present["name"] || !response.Present["oneofs"]) {
				t.Errorf("实际解码数据 %+v，预期保留字段值与存在性", response)
			}
			if tc.name == "省略数组" && response.Present["oneofs"] {
				t.Error("省略数组被误认为已提供")
			}
			if tc.name == "显式空数组" && (!response.Present["oneofs"] || len(response.Oneofs) != 0) {
				t.Error("显式空数组丢失存在性或生成了元素")
			}
		})
	}
}
