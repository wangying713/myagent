// Package httpx 提供可复用的 HTTP JSON 解析、响应编码和传输错误，不依赖业务包。
package httpx

import (
	"errors"
	"net/http"
)

// Error 保存可对外返回的稳定错误码和说明，由 HTTP 入口决定外层包装。
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RequestError 只描述传输层失败，不承载领域错误。
type RequestError struct {
	Status int
	Detail Error
}

// Error 返回传输错误的公开说明。
func (e *RequestError) Error() string {
	return e.Detail.Message
}

func requestError(err error) *RequestError {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &RequestError{
			Status: http.StatusRequestEntityTooLarge,
			Detail: Error{
				Code:    "request_too_large",
				Message: "请求正文不能超过 32 KiB",
			},
		}
	}
	return &RequestError{
		Status: http.StatusBadRequest,
		Detail: Error{
			Code:    "invalid_input",
			Message: "请求必须为单个合法 JSON 对象，字段、类型及值必须符合接口定义",
		},
	}
}

// ClassifyReqError 将解码错误转换为 HTTP 状态和稳定的传输错误，不决定响应外层格式。
// err 应为非 nil 解码错误，未分类的错误按无效输入处理。
func ClassifyReqError(err error) (int, *Error) {
	var failure *RequestError
	if !errors.As(err, &failure) {
		failure = requestError(err)
	}
	return failure.Status, &failure.Detail
}
