// Package httpx 提供可复用的 HTTP JSON 解析、响应编码和传输错误，不依赖业务包。
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RequestError 只描述传输层失败，不承载领域错误。
type RequestError struct {
	Status int
	Detail Error
}

func (e *RequestError) Error() string { return e.Detail.Message }

func requestError(err error) *RequestError {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &RequestError{Status: http.StatusRequestEntityTooLarge, Detail: Error{Code: "request_too_large", Message: "请求正文不能超过 32 KiB"}}
	}
	return &RequestError{Status: http.StatusBadRequest, Detail: Error{Code: "invalid_input", Message: "请求必须为单个合法 JSON 对象，字段、类型及值必须符合接口定义"}}
}

// WriteRequestError 允许调用方传入其响应数据的零值，保持各接口的响应契约。
func WriteRequestError(c *gin.Context, err error, data any) {
	var failure *RequestError
	if !errors.As(err, &failure) {
		failure = requestError(err)
	}
	WriteResult(c, failure.Status, data, &failure.Detail)
}
