package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

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

// testRsp 由测试调用方定义外层，验证 HTTP 工具不约束其字段布局。
type testRsp[T any] struct {
	Data  T            `json:"data"`            // 测试接口返回的业务数据。
	Error *httpx.Error `json:"error,omitempty"` // 测试接口返回的传输错误，成功时为 nil。
}

func writeTestRsp(c *gin.Context, status int, data any, failure *httpx.Error) {
	if err := httpx.Write(c, status, testRsp[any]{
		Data:  data,
		Error: failure,
	}); err != nil {
		panic("测试响应编码失败")
	}
}

func writeTestReqError(c *gin.Context, err error, data any) {
	status, failure := httpx.ClassifyReqError(err)
	writeTestRsp(c, status, data, failure)
}
