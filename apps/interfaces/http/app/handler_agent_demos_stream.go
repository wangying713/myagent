package app

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// handleAgentDemosStream 只适配 SSE 传输；参数、超时和错误仍沿用普通接口的约定。
// 每个 data 事件都是统一响应外层，最终事件的 done=true；断流不能视为成功。
func handleAgentDemosStream(api func(context.Context, *entity.DemosStreamReq) (iter.Seq2[*entity.DemosStreamRsp, error], error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := c.GetDuration(requestTimeoutKey{})
		if timeout <= 0 {
			writeHTTPRsp(c, http.StatusInternalServerError, nil, &httpx.Error{
				Code:    entity.CodeInternalError,
				Message: "服务未配置请求超时",
			})
			return
		}
		req := &entity.DemosStreamReq{}
		if err := httpx.DecodeJSON(c, req); err != nil {
			status, failure := httpx.ClassifyReqError(err)
			writeHTTPRsp(c, status, nil, failure)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		events, err := api(ctx, req)
		if err != nil {
			writeAPIResult(c, nil, err)
			return
		}
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		span := trace.SpanFromContext(ctx)
		traceID := ""
		if span.IsRecording() {
			traceID = span.SpanContext().TraceID().String()
		}
		for rsp, err := range events {
			_, failure := classifyAgentError(err)
			envelope := entity.Rsp[any]{
				Code:    entity.CodeOK,
				Data:    rsp,
				TraceID: traceID,
				Msg:     "成功",
			}
			if failure != nil {
				envelope.Code = failure.Code
				envelope.Msg = failure.Message
				span.SetStatus(codes.Error, "流式回答未完成")
			}
			encoded, encodeErr := json.Marshal(envelope)
			if encodeErr != nil {
				span.SetStatus(codes.Error, "流式响应编码失败")
				return
			}
			if _, writeErr := c.Writer.Write(append(append([]byte("data: "), encoded...), '\n', '\n')); writeErr != nil {
				cancel()
				return
			}
			c.Writer.Flush()
		}
	}
}
