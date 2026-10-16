package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

// 使用与 Agent 无关的请求，验证本包可供其他后台模块复用。
type catalogRequest struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func catalogRouter() *gin.Engine {
	router := gin.New()
	router.POST("/catalog", func(c *gin.Context) {
		var request catalogRequest
		if err := httpx.DecodeJSON(c, &request, "name", "count"); err != nil {
			writeTestReqError(c, err, nil)
			return
		}
		writeTestRsp(c, http.StatusOK, request, nil)
	})
	return router
}

func TestDecodeJSONContract(t *testing.T) {
	router := catalogRouter()
	for _, tc := range []struct {
		name, body, contentType, code string
		status                        int
	}{
		{name: "正常类型", body: `{"name":"数据目录","count":2}`, contentType: "application/json; charset=utf-8", status: 200},
		{name: "空对象", body: `{}`, contentType: "application/json", status: 200},
		{name: "仅校验传输类型", body: `{"name":"","count":-1}`, contentType: "application/json", status: 200},
		{name: "缺少类型", body: `{}`, status: 415, code: "invalid_content_type"},
		{name: "非 JSON 类型", body: `{}`, contentType: "text/plain", status: 415, code: "invalid_content_type"},
		{name: "根 null", body: `null`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "数组", body: `[]`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "不完整对象", body: `{`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "字段 null", body: `{"name":null}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "重复字段", body: `{"name":"甲","name":"乙"}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "大小写别名", body: `{"Name":"目录"}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "未知字段", body: `{"other":1}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "错误类型", body: `{"count":"2"}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "多个对象", body: `{} {}`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "无效尾部", body: `{} trailing`, contentType: "application/json", status: 400, code: "invalid_input"},
		{name: "正文超限", body: `{"name":"` + strings.Repeat("a", httpx.MaxBodyBytes) + `"}`, contentType: "application/json", status: 413, code: "request_too_large"},
		{name: "尾随空白超限", body: `{}` + strings.Repeat(" ", httpx.MaxBodyBytes), contentType: "application/json", status: 413, code: "request_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/catalog", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, request)
			var response testRsp[catalogRequest]
			if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
				t.Fatalf("响应不是合法 JSON：%s，错误 %v", out.Body.String(), err)
			}
			if out.Code != tc.status || out.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("实际 HTTP %d、响应 %s，预期 HTTP %d 且禁止缓存", out.Code, out.Body.String(), tc.status)
			}
			if tc.code != "" && (response.Error == nil || response.Error.Code != tc.code) {
				t.Errorf("实际错误 %+v，预期 %s", response.Error, tc.code)
			}
			if tc.name == "正常类型" && (response.Data.Name != "数据目录" || response.Data.Count != 2) {
				t.Errorf("解析后数据发生变化：%+v", response.Data)
			}
		})
	}
}

func TestEncodingFailureDoesNotWritePartialSuccess(t *testing.T) {
	out := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(out)
	if err := httpx.Write(ctx, http.StatusOK, make(chan int)); err == nil {
		t.Fatal("不可编码的数据没有返回错误")
	}
	if ctx.Writer.Written() || out.Body.Len() != 0 || out.Header().Get("Content-Type") != "" {
		t.Fatalf("编码失败写出了部分响应：HTTP %d，正文 %s，头部 %v", out.Code, out.Body.String(), out.Header())
	}
	if err := httpx.Write(ctx, http.StatusInternalServerError, map[string]string{"code": "encoding_failed"}); err != nil {
		t.Fatal(err)
	}
	if out.Code != 500 || out.Body.String() != `{"code":"encoding_failed"}` || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("调用方无法完整回退响应：%d %s", out.Code, out.Body.String())
	}
}

// 测试调用方自行定义外层，HTTP 工具不约束其字段布局。
type testRsp[T any] struct {
	Data  T            `json:"data"`
	Error *httpx.Error `json:"error,omitempty"`
}

func writeTestRsp(c *gin.Context, status int, data any, failure *httpx.Error) {
	if err := httpx.Write(c, status, testRsp[any]{Data: data, Error: failure}); err != nil {
		panic("测试响应编码失败")
	}
}

func writeTestReqError(c *gin.Context, err error, data any) {
	status, failure := httpx.ClassifyReqError(err)
	writeTestRsp(c, status, data, failure)
}
