package httpx

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

// Adapter 统一处理超时和响应，具体数据编码、业务错误映射和响应包装由调用方注入。
// 构造后不修改配置，可供多个接口和并发请求共用。
type Adapter[Rsp any] struct {
	timeout  time.Duration
	encode   func(Rsp) any
	classify func(error) (int, *Error)
	write    func(*gin.Context, int, any, *Error)
}

func NewAdapter[Rsp any](timeout time.Duration, encode func(Rsp) any, classify func(error) (int, *Error), write func(*gin.Context, int, any, *Error)) (*Adapter[Rsp], error) {
	if timeout <= 0 || encode == nil || classify == nil || write == nil {
		return nil, errors.New("HTTP 适配器需要正数超时、响应编码、错误映射和响应写出函数")
	}
	return &Adapter[Rsp]{timeout: timeout, encode: encode, classify: classify, write: write}, nil
}

// reqPointer 保证请求由 Go 分配，且其指针实现生成消息接口。
type reqPointer[Req any] interface {
	*Req
	proto.Message
}

// Adapt 将 (ctx, req) -> (rsp, err) 方法适配为 Gin 处理器。
// afterDecode 只在解码成功后执行，供 API 显式保留字段存在性等传输语义。
// 超时从解码完成后开始，与现有接口约定一致；方法返回错误时也编码其响应。
func Adapt[Req, Rsp any, PReq reqPointer[Req]](
	adapter *Adapter[Rsp],
	handle func(context.Context, PReq) (Rsp, error),
	afterDecode ...func(PReq, map[string]bool),
) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := PReq(new(Req))
		present, err := DecodeProtoJSON(c, req)
		if err != nil {
			status, failure := ClassifyReqError(err)
			adapter.write(c, status, nil, failure)
			return
		}
		for _, prepare := range afterDecode {
			prepare(req, present)
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), adapter.timeout)
		defer cancel()
		rsp, err := handle(ctx, req)
		status, failure := adapter.classify(err)
		adapter.write(c, status, adapter.encode(rsp), failure)
	}
}
