package httpx

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCaptureBodiesKeepsActualReadWriteAndBoundsCopies(t *testing.T) {
	const body = "abcdefghijk"
	for _, limit := range []int{0, 4, 64} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			handler := gin.New()
			called := false
			handler.Use(CaptureBodies(limit, func(c *gin.Context, request, response []byte) {
				called = true
				want := body
				if limit < len(body) {
					want = body[:limit] + "…[TRUNCATED]"
				}
				if string(request) != want || string(response) != want || c.Writer.Status() != 201 {
					t.Errorf("实际记录 %q / %q / %d，预期 %q / %q / 201", request, response, c.Writer.Status(), want, want)
				}
			}))
			handler.POST("/copy", func(c *gin.Context) {
				raw, err := io.ReadAll(c.Request.Body)
				if err != nil || string(raw) != body {
					t.Fatalf("实际读取 %q，错误 %v，预期完整正文", raw, err)
				}
				c.Status(http.StatusCreated)
				if _, err := c.Writer.Write(raw[:3]); err != nil {
					t.Fatal(err)
				}
				if _, err := c.Writer.WriteString(string(raw[3:])); err != nil {
					t.Fatal(err)
				}
			})
			req := httptest.NewRequest(http.MethodPost, "/copy", strings.NewReader(body))
			originalBody := req.Body
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != 201 || out.Body.String() != body || called != (limit > 0) || req.Body != originalBody {
				t.Errorf("实际 HTTP %d %q、回调 %v，预期完整响应、按开关回调并还原请求体", out.Code, out.Body.String(), called)
			}
		})
	}
}

func TestCaptureBodiesDoesNotReadAhead(t *testing.T) {
	handler := gin.New()
	handler.Use(CaptureBodies(64, func(_ *gin.Context, request, _ []byte) {
		if string(request) != "abc" {
			t.Errorf("实际采集 %q，预期只包含处理方法读取的 abc", request)
		}
	}))
	handler.POST("/partial", func(c *gin.Context) {
		body := make([]byte, 3)
		if _, err := io.ReadFull(c.Request.Body, body); err != nil {
			t.Fatal(err)
		}
		c.Status(http.StatusBadRequest)
	})
	req := httptest.NewRequest(http.MethodPost, "/partial", strings.NewReader("abcdef"))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	rest, err := io.ReadAll(req.Body)
	if err != nil || string(rest) != "def" {
		t.Errorf("实际剩余正文 %q、错误 %v，预期 def，观测不得额外消费请求", rest, err)
	}
}
