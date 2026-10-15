package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response[T any] struct {
	Data  T      `json:"data"`
	Error *Error `json:"error,omitempty"`
}

func WriteResult(c *gin.Context, status int, data any, failure *Error) {
	Write(c, status, Response[any]{Data: data, Error: failure})
}

// Write 先完成编码再写响应头，避免发送半个成功响应，不重试业务请求。
func Write(c *gin.Context, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"data":null,"error":{"code":"internal_error","message":"响应编码失败"}}`)
	}
	c.Header("Cache-Control", "no-store")
	c.Data(status, "application/json; charset=utf-8", body)
}
