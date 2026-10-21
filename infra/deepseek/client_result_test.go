package deepseek

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"saklay/ai/domain/agent"
	"saklay/ai/infra/openobserve"
)

// resultTransport 用固定响应或错误验证客户端结果契约，不连接外部服务。
type resultTransport func(*http.Request) (*http.Response, error)

// RoundTrip 返回用例指定的响应，保持标准库固定签名。
func (f resultTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newResultClient 组装只使用内存传输的模型客户端。
func newResultClient(t *testing.T, transport resultTransport) *Client {
	t.Helper()
	observeCli, err := openobserve.New(t.Context(), openobserve.Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := observeCli.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	chatCli, err := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://model.test/v1",
		Model:   "test-model",
		Timeout: time.Second,
	}, observeCli, &http.Client{
		Transport: transport,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(chatCli.Close)
	return chatCli
}

func TestModelPreflightErrorsReturnNoResult(t *testing.T) {
	calls := 0
	chatCli := newResultClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("前置拒绝不应请求模型")
	})
	for _, req := range []agent.Request{
		{},
		{
			Stream: true,
		},
		{
			Messages: []agent.Message{
				{
					Raw: []byte("{"),
				},
			},
		},
	} {
		result, err := chatCli.Chat(t.Context(), req)
		if result != nil || err == nil {
			t.Fatalf("实际模型结果 %+v、错误 %v，预期前置失败返回 nil 和错误", result, err)
		}
	}
	stream, err := chatCli.Stream(t.Context(), agent.StreamInput{})
	if stream != nil || err == nil {
		t.Fatalf("实际流式结果 %+v、错误 %v，预期前置失败返回 nil 和错误", stream, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := chatCli.Chat(ctx, agent.Request{})
	if result != nil || !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("实际结果 %+v、错误 %v、调用 %d 次，预期 nil、取消错误且不发起请求", result, err, calls)
	}
}

func TestModelResponseErrorKeepsOriginalPayload(t *testing.T) {
	const body = `{"choices":[]}`
	chatCli := newResultClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	result, err := chatCli.Chat(t.Context(), agent.Request{
		Messages: []agent.Message{
			{
				Role:    agent.RoleUser,
				Content: "问题",
			},
		},
	})
	if result == nil || !errors.Is(err, agent.ErrInvalidResponse) {
		t.Fatalf("实际结果 %+v、错误 %v，预期保留原文并返回协议错误", result, err)
	}
	if string(result.Raw) != body || len(result.RequestJSON) == 0 {
		t.Fatalf("实际原始响应 %q、请求 %q，预期保留已取得的请求与响应", result.Raw, result.RequestJSON)
	}
}

func TestStreamFailureKeepsReceivedText(t *testing.T) {
	const body = "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"部分回答\"}}]}\n\n"
	chatCli := newResultClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	text := ""
	result, err := chatCli.Stream(t.Context(), agent.StreamInput{
		Request: agent.Request{
			Messages: []agent.Message{
				{
					Role:    agent.RoleUser,
					Content: "问题",
				},
			},
		},
		Emit: func(part string) error {
			text += part
			return nil
		},
	})
	if result == nil || !errors.Is(err, agent.ErrInvalidResponse) {
		t.Fatalf("实际流式结果 %+v、错误 %v，预期未完成且保留结果", result, err)
	}
	if text != "部分回答" || result.Message.Content != text || len(result.Raw) == 0 || len(result.RequestJSON) == 0 {
		t.Fatalf("实际分段 %q、结果 %+v，预期保留已接收的文字和原文", text, result)
	}
}
