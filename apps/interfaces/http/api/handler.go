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

func New(runner *demos.Runner, telemetry *openobserve.Telemetry, cfg Config) (http.Handler, error) {
	if runner == nil || telemetry == nil || cfg.RunTimeout <= 0 {
		return nil, errors.New("HTTP 依赖与请求超时无效")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/demos/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	for _, lesson := range []string{"chat", "messages", "agent"} {
		mux.HandleFunc("POST /api/demos/"+lesson, func(w http.ResponseWriter, r *http.Request) {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				write(w, http.StatusUnsupportedMediaType, Response{Error: &Error{Code: "invalid_content_type", Message: "请使用 application/json"}})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
			defer r.Body.Close()
			input, err := decodeInput(r.Body, lesson)
			if err != nil {
				writeInputError(w, err)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), cfg.RunTimeout)
			defer cancel()
			result, err := runner.Run(ctx, lesson, input)
			// 不把模型或客户端底层错误文本直接发给浏览器。
			status, apiError := classify(err)
			result.Answer = telemetry.Clean(result.Answer)
			write(w, status, Response{Data: result, Error: apiError})
		})
	}
	return mux, nil
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
	if d.Decode(new(any)) != io.EOF {
		return input, demos.ErrInput
	}
	return input, nil
}

func writeInputError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	status, code, message := http.StatusBadRequest, "invalid_input", "请求必须为单个合法 JSON 对象，且只包含本课程允许的字段"
	if errors.As(err, &tooLarge) {
		status, code, message = http.StatusRequestEntityTooLarge, "request_too_large", "请求正文不能超过 32 KiB"
	}
	write(w, status, Response{Error: &Error{Code: code, Message: message}})
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

func write(w http.ResponseWriter, status int, value any) {
	// 先编码再写响应头，编码失败不能发送半个成功响应。
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "响应编码失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// 客户端断开时没有第二个响应渠道，忽略写入失败，不重试业务。
	_, _ = w.Write(body)
}
