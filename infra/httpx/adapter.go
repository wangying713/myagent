package httpx

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

// Adapter 统一处理超时和响应，具体数据编码与业务错误映射由 API 注入。
// 构造后不修改配置，可供多个接口和并发请求共用。
type Adapter[Response any] struct {
	timeout  time.Duration
	encode   func(Response) any
	classify func(error) (int, *Error)
}

func NewAdapter[Response any](timeout time.Duration, encode func(Response) any, classify func(error) (int, *Error)) (*Adapter[Response], error) {
	if timeout <= 0 || encode == nil || classify == nil {
		return nil, errors.New("HTTP 适配器需要正数超时、响应编码和错误映射")
	}
	return &Adapter[Response]{timeout: timeout, encode: encode, classify: classify}, nil
}

// requestPointer 保证请求由 Go 分配，且其指针实现生成消息接口。
type requestPointer[Request any] interface {
	*Request
	proto.Message
}

// Adapt 将 (ctx, req) -> (rsp, err) 方法适配为 Gin 处理器。
// afterDecode 只在解码成功后执行，供 API 显式保留字段存在性等传输语义。
// 超时从解码完成后开始，与现有接口约定一致；方法返回错误时也编码其响应。
func Adapt[Request, Response any, PRequest requestPointer[Request]](
	adapter *Adapter[Response],
	handle func(context.Context, PRequest) (Response, error),
	afterDecode ...func(PRequest, map[string]bool),
) gin.HandlerFunc {
	return func(c *gin.Context) {
		request := PRequest(new(Request))
		present, err := DecodeProtoJSON(c, request)
		if err != nil {
			var response Response
			WriteRequestError(c, err, adapter.encode(response))
			return
		}
		for _, prepare := range afterDecode {
			prepare(request, present)
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), adapter.timeout)
		defer cancel()
		response, err := handle(ctx, request)
		status, failure := adapter.classify(err)
		WriteResult(c, status, adapter.encode(response), failure)
	}
}
