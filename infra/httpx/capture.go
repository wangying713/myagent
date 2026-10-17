package httpx

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

// CaptureBodies 按实际读写记录正文的有界副本，处理结束后同步调用 record。
// limit 为每份正文的字节上限；超过时附加截断标记，不预读请求或改变实际读写结果。
// 调用方负责内容开关及脱敏；本中间件不采集请求头、URL 或业务元数据。
func CaptureBodies(limit int, record func(c *gin.Context, request, response []byte)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 || record == nil {
			c.Next()
			return
		}
		request := &bodyCapture{limit: limit}
		response := &bodyCapture{limit: limit}
		body, writer := c.Request.Body, c.Writer
		if body != nil {
			c.Request.Body = &captureReader{
				ReadCloser: body,
				capture:    request,
			}
		}
		c.Writer = &captureWriter{
			ResponseWriter: writer,
			capture:        response,
		}
		defer func() {
			c.Request.Body, c.Writer = body, writer
		}()
		c.Next()
		record(c, request.bytes(), response.bytes())
	}
}

// bodyCapture 保存正文的有界副本及截断状态。
type bodyCapture struct {
	body      bytes.Buffer // 已采集的正文副本。
	limit     int          // 正文副本的最大字节数，不含截断标记。
	truncated bool         // 是否存在未保留的正文内容。
}

func (b *bodyCapture) append(raw []byte) {
	n := min(len(raw), b.limit-b.body.Len())
	b.body.Write(raw[:n])
	b.truncated = b.truncated || n < len(raw)
}

func (b *bodyCapture) appendString(text string) {
	n := min(len(text), b.limit-b.body.Len())
	b.body.WriteString(text[:n])
	b.truncated = b.truncated || n < len(text)
}

func (b *bodyCapture) bytes() []byte {
	if b.truncated {
		return append(b.body.Bytes(), []byte("…[TRUNCATED]")...)
	}
	return b.body.Bytes()
}

// captureReader 随实际读取采集请求体，沿用底层关闭行为。
type captureReader struct {
	io.ReadCloser              // 原始请求体，保留读取与关闭行为。
	capture       *bodyCapture // 随实际读取累计的请求正文副本。
}

func (r *captureReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.capture.append(p[:n])
	return n, err
}

// captureWriter 随实际成功写出采集响应体，保留 Gin 写入接口。
type captureWriter struct {
	gin.ResponseWriter              // 原始响应写入器，保留 Gin 响应能力。
	capture            *bodyCapture // 仅记录实际成功写出的响应正文。
}

func (w *captureWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.capture.append(p[:n])
	return n, err
}

func (w *captureWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.capture.appendString(s[:n])
	return n, err
}
