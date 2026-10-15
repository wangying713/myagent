// Package api 适配三课 HTTP 接口，负责请求校验、超时与稳定的错误响应。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/demos"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/openobserve"
)

type Config struct {
	Address    string
	RunTimeout time.Duration
}

type Response struct {
	Data  demos.Result `json:"data"`
	Error *Error       `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(runner *demos.Runner, telemetry *openobserve.Telemetry, cfg Config) (*gin.Engine, error) {
	if runner == nil || telemetry == nil || cfg.RunTimeout <= 0 {
		return nil, errors.New("HTTP 依赖与请求超时无效")
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, errors.New("Gin 代理配置无效")
	}
	// 默认 recovery 会打印请求头和 panic 值；这里只返回固定错误，避免泄漏凭据。
	router.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		c.Abort()
		if !c.Writer.Written() {
			write(c, http.StatusInternalServerError, Response{Error: &Error{Code: "internal_error", Message: "服务处理异常，请稍后重试"}})
		}
	}))
	router.NoRoute(func(c *gin.Context) {
		write(c, http.StatusNotFound, Response{Error: &Error{Code: "not_found", Message: "接口不存在"}})
	})
	router.NoMethod(func(c *gin.Context) {
		write(c, http.StatusMethodNotAllowed, Response{Error: &Error{Code: "method_not_allowed", Message: "接口不支持该请求方法"}})
	})
	routes := router.Group("/api/demos")
	health := func(c *gin.Context) {
		write(c, http.StatusOK, map[string]string{"status": "ok"})
	}
	routes.GET("/health", health)
	routes.HEAD("/health", health)
	for _, lesson := range []string{"chat", "messages", "agent"} {
		routes.POST("/"+lesson, func(c *gin.Context) {
			r := c.Request
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				write(c, http.StatusUnsupportedMediaType, Response{Error: &Error{Code: "invalid_content_type", Message: "请使用 application/json"}})
				return
			}
			r.Body = http.MaxBytesReader(c.Writer, r.Body, 32<<10)
			defer r.Body.Close()
			input, err := decodeInput(r.Body, lesson)
			if err != nil {
				writeInputError(c, err)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), cfg.RunTimeout)
			defer cancel()
			result, err := runner.Run(ctx, lesson, input)
			// 不把模型或客户端底层错误文本直接发给浏览器。
			status, apiError := classify(err)
			result.Answer = telemetry.Clean(result.Answer)
			write(c, status, Response{Data: result, Error: apiError})
		})
	}
	return router, nil
}

// 明确拒绝 null、重复键、大小写别名和跨课程字段，避免模糊输入触发付费请求。
func decodeInput(body io.Reader, lesson string) (demos.Input, error) {
	var input demos.Input
	d := json.NewDecoder(body)
	start, err := d.Token()
	if err != nil {
		return input, err
	}
	if start != json.Delim('{') {
		return input, demos.ErrInput
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return input, err
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return input, demos.ErrInput
		}
		seen[name] = true
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return input, err
		}
		switch {
		case name == "question" && lesson != "messages":
			if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &input.Question) != nil || input.Question == "" {
				return input, demos.ErrInput
			}
		case name == "questions" && lesson == "messages":
			if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &input.Questions) != nil {
				return input, demos.ErrInput
			}
		default:
			return input, demos.ErrInput
		}
	}
	if _, err := d.Token(); err != nil {
		return input, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		if err == nil {
			return input, demos.ErrInput
		}
		return input, err
	}
	return input, nil
}

func writeInputError(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	status, code, message := http.StatusBadRequest, "invalid_input", "请求必须为单个合法 JSON 对象，且只包含本课程允许的字段"
	if errors.As(err, &tooLarge) {
		status, code, message = http.StatusRequestEntityTooLarge, "request_too_large", "请求正文不能超过 32 KiB"
	}
	write(c, status, Response{Error: &Error{Code: code, Message: message}})
}

func classify(err error) (int, *Error) {
	if err == nil {
		return http.StatusOK, nil
	}
	switch {
	case errors.Is(err, demos.ErrInput):
		return http.StatusBadRequest, &Error{Code: "invalid_input", Message: "聊天与工具课程需要一个问题；历史课程需要两个非空问题，每个问题最多 4096 字符"}
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, &Error{Code: "timeout", Message: "执行超时，已停止后续调用"}
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, &Error{Code: "canceled", Message: "执行已取消"}
	case errors.Is(err, agent.ErrBudgetExceeded):
		return http.StatusConflict, &Error{Code: "budget_exceeded", Message: "达到运行预算，任务未完成"}
	case errors.Is(err, agent.ErrInvalidResponse):
		return http.StatusBadGateway, &Error{Code: "invalid_model_response", Message: "模型未返回完整回答或工具协议无效"}
	default:
		return http.StatusBadGateway, &Error{Code: "model_request_failed", Message: "模型服务调用失败，请通过执行记录和 trace ID 排查"}
	}
}

func write(c *gin.Context, status int, value any) {
	// 先编码再写响应头，编码失败不能发送半个成功响应。
	body, err := json.Marshal(value)
	if err != nil {
		c.String(http.StatusInternalServerError, "响应编码失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	// 复用统一 JSON 编码；Gin 负责写响应，不因客户端断开重试业务。
	c.Data(status, "application/json; charset=utf-8", body)
}
