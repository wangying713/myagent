package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/deepseek"
	"saklay/ai/infra/httpx"
	"saklay/ai/infra/openobserve"

	"github.com/gin-gonic/gin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

// 按管理台接收的 HTTP JSON 解码，避免用生成类型掩盖线上的格式变化。
type Response = httpx.Response[struct {
	Completed   bool              `json:"completed"`
	Answer      string            `json:"answer"`
	TraceID     string            `json:"trace_id"`
	ModelCalls  int               `json:"model_calls"`
	ToolCalls   int               `json:"tool_calls"`
	KnownTokens int               `json:"known_tokens"`
	Steps       []json.RawMessage `json:"steps"`
}]

type memoryExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
	fail  bool
}

func (e *memoryExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	if e.fail {
		return errors.New("上报失败 test-api-key")
	}
	return nil
}
func (e *memoryExporter) Shutdown(context.Context) error { return nil }

func setup(t *testing.T, model http.HandlerFunc, limits agent.Limits, timeout time.Duration, exporter *memoryExporter) (*gin.Engine, *openobserve.Telemetry) {
	t.Helper()
	upstream := httptest.NewServer(model)
	t.Cleanup(upstream.Close)
	telemetry, err := openobserve.New(context.Background(), openobserve.Config{Enabled: true, ServiceName: "test", MaxChars: 32768, CaptureContent: true, Secrets: []string{"test-api-key"}}, exporter, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := telemetry.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	client, err := deepseek.New(deepseek.Config{APIKey: "test-api-key", BaseURL: upstream.URL, Model: "test-model", Timeout: time.Second}, telemetry, upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	runner, err := agent.NewService(client, telemetry, limits)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newRouter(runner, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return handler, telemetry
}

func invoke(handler http.Handler, lesson, body string) (*httptest.ResponseRecorder, Response) {
	path := "/api/agent/" + lesson
	if lesson == "agent" {
		path = "/api/agent/run"
	}
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	var response Response
	json.Unmarshal(w.Body.Bytes(), &response)
	return w, response
}

func reply(w http.ResponseWriter, message any, reason string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": reason}}, "usage": map[string]int{"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10}})
}

func TestThreeLessonsAndParentSpans(t *testing.T) {
	for _, lesson := range []string{"chat", "messages", "agent"} {
		t.Run(lesson, func(t *testing.T) {
			exporter := &memoryExporter{}
			handler, telemetry := setup(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-api-key" {
					t.Error("模型鉴权没有由后端发送")
				}
				var input agent.Request
				if json.NewDecoder(r.Body).Decode(&input) != nil {
					t.Error("模型收到无效请求")
					w.WriteHeader(400)
					return
				}
				if input.Model != "test-model" || input.Thinking.Type != "disabled" || input.MaxTokens != 512 || input.Stream {
					t.Errorf("课程模型默认值发生变化：%+v", input)
				}
				switch lesson {
				case "chat":
					reply(w, map[string]string{"role": "assistant", "content": "递归是在函数中调用自身。"}, "stop")
				case "messages":
					if len(input.Messages) == 2 {
						reply(w, map[string]string{"role": "assistant", "content": "记住了豆包。", "extra": "历史扩展字段"}, "stop")
						return
					}
					if len(input.Messages) != 4 || input.Messages[2].Content != "记住了豆包。" || input.Messages[3].Content != agent.SecondQuestion {
						t.Errorf("第二轮未携带真实历史：%+v", input.Messages)
					}
					reply(w, map[string]string{"role": "assistant", "content": "豆包"}, "stop")
				case "agent":
					if len(input.Messages) == 2 {
						reply(w, map[string]any{"role": "assistant", "content": nil, "tool_calls": []agent.ToolCall{{ID: "multiply-1", Type: "function", Function: agent.FunctionCall{Name: "multiply", Arguments: `{"a":17,"b":23}`}}}}, "tool_calls")
						return
					}
					last := input.Messages[len(input.Messages)-1]
					if last.Role != "tool" || last.ToolCallID != "multiply-1" || !strings.Contains(last.Content, "391") {
						t.Errorf("工具结果没有关联回填：%+v", last)
					}
					reply(w, map[string]string{"role": "assistant", "content": "一共 391 支笔。"}, "stop")
				}
			}, agent.DefaultLimits(), time.Second, exporter)
			w, response := invoke(handler, lesson, `{}`)
			wantModels, wantTools, wantSpans := 1, 0, 2
			if lesson == "messages" {
				wantModels, wantSpans = 2, 3
			}
			if lesson == "agent" {
				wantModels, wantTools, wantSpans = 2, 1, 4
			}
			if w.Code != 200 || response.Error != nil || !response.Data.Completed || response.Data.ModelCalls != wantModels || response.Data.ToolCalls != wantTools || response.Data.KnownTokens != wantModels*10 || len(response.Data.Steps) != wantSpans-1 {
				t.Fatalf("课程实际响应：%d %s", w.Code, w.Body.String())
			}
			if err := telemetry.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			exporter.mu.Lock()
			defer exporter.mu.Unlock()
			if len(exporter.spans) != wantSpans {
				t.Fatalf("实际 %d 个 span，预期 %d", len(exporter.spans), wantSpans)
			}
			var root sdktrace.ReadOnlySpan
			for _, s := range exporter.spans {
				if s.Name() == "agent.run" {
					root = s
				}
			}
			if root == nil {
				t.Fatal("缺少运行根 span")
			}
			for _, s := range exporter.spans {
				if s.Name() != "agent.run" && (s.Parent().SpanID() != root.SpanContext().SpanID() || s.SpanContext().TraceID() != root.SpanContext().TraceID()) {
					t.Error("模型或工具没有挂到同次运行的根 span")
				}
			}
			if response.Data.TraceID != root.SpanContext().TraceID().String() {
				t.Error("页面 trace ID 与实际记录不一致")
			}
		})
	}
}

func TestCustomQuestionsReachModel(t *testing.T) {
	for _, tc := range []struct {
		lesson, body string
		questions    []string
	}{
		{lesson: "chat", body: `{"question":"解释 Go 接口"}`, questions: []string{"解释 Go 接口"}},
		{lesson: "messages", body: `{"questions":["请记住名字小白","刚才的名字是什么"]}`, questions: []string{"请记住名字小白", "刚才的名字是什么"}},
		{lesson: "agent", body: `{"question":"计算 3 乘 5"}`, questions: []string{"计算 3 乘 5"}},
	} {
		t.Run(tc.lesson, func(t *testing.T) {
			var calls atomic.Int32
			handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
				var request agent.Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("模型请求解码失败：%v", err)
					w.WriteHeader(400)
					return
				}
				index := int(calls.Add(1)) - 1
				if index >= len(tc.questions) || len(request.Messages) == 0 {
					t.Errorf("实际第 %d 次模型请求、%d 条消息，预期最多 %d 次且非空", index+1, len(request.Messages), len(tc.questions))
					w.WriteHeader(400)
					return
				}
				last := request.Messages[len(request.Messages)-1]
				if last.Role != "user" || last.Content != tc.questions[index] {
					t.Errorf("模型实际收到 %+v，预期自定义问题 %q", last, tc.questions[index])
				}
				reply(w, map[string]string{"role": "assistant", "content": last.Content}, "stop")
			}, agent.DefaultLimits(), time.Second, &memoryExporter{})
			out, response := invoke(handler, tc.lesson, tc.body)
			if out.Code != 200 || !response.Data.Completed || response.Data.Answer != tc.questions[len(tc.questions)-1] || int(calls.Load()) != len(tc.questions) {
				t.Errorf("实际 HTTP %d、响应 %s、调用 %d 次，预期使用自定义问题完成 %d 次调用", out.Code, out.Body.String(), calls.Load(), len(tc.questions))
			}
		})
	}
}

func TestInputRejectionDoesNotCallModel(t *testing.T) {
	var calls atomic.Int32
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }, agent.DefaultLimits(), time.Second, &memoryExporter{})
	for _, lesson := range []string{"chat", "agent"} {
		for _, body := range []string{`null`, `[]`, `{`, `{} {}`, `{"question":null}`, `{"question":""}`, `{"Question":"问题"}`, `{"question":"a","question":"b"}`, `{"model":"override"}`, `{"questions":["a","b"]}`, `{"question":"  "}`, `{"question":123}`} {
			w, _ := invoke(handler, lesson, body)
			if w.Code != 400 {
				t.Errorf("接口 %s 输入 %s 实际 HTTP %d，预期 400", lesson, body, w.Code)
			}
		}
	}
	for _, body := range []string{`{"question":"` + strings.Repeat("a", 40<<10) + `"}`, `{}` + strings.Repeat(" ", 40<<10)} {
		w, response := invoke(handler, "chat", body)
		if w.Code != 413 || response.Error == nil || response.Error.Code != "request_too_large" {
			t.Errorf("超大请求实际 HTTP %d，响应 %s，预期 request_too_large", w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`{"questions":[]}`, `{"questions":null}`, `{"questions":["只有一轮"]}`, `{"questions":["a","b","c"]}`, `{"questions":[null,"b"]}`, `{"questions":["a",2]}`, `{"questions":["a",""]}`, `{"questions":["a","b"],"questions":["c","d"]}`, `{"Questions":["a","b"]}`, `{"questions":["a","b"],"model":"override"}`} {
		w, _ := invoke(handler, "messages", body)
		if w.Code != 400 {
			t.Errorf("历史接口输入 %s 实际 HTTP %d，预期 400", body, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/agent/chat", nil)
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, r)
	if out.Code != 405 {
		t.Errorf("GET 课程实际 HTTP %d", out.Code)
	}
	if calls.Load() != 0 {
		t.Fatalf("非法输入触发了 %d 次模型请求", calls.Load())
	}
}

func TestRoutingAndHealthDoNotCallModel(t *testing.T) {
	var calls atomic.Int32
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }, agent.DefaultLimits(), time.Second, &memoryExporter{})
	for _, tc := range []struct {
		method, path, code, allow string
		status                    int
	}{
		{method: "GET", path: "/api/health", status: 200},
		{method: "HEAD", path: "/api/health", status: 200},
		{method: "GET", path: "/api/agent/chat", status: 405, code: "method_not_allowed", allow: "POST"},
		{method: "GET", path: "/api/agent/messages", status: 405, code: "method_not_allowed", allow: "POST"},
		{method: "GET", path: "/api/agent/run", status: 405, code: "method_not_allowed", allow: "POST"},
		{method: "POST", path: "/api/health", status: 405, code: "method_not_allowed", allow: "GET,HEAD"},
		{method: "OPTIONS", path: "/api/agent/chat", status: 405, code: "method_not_allowed", allow: "POST"},
		{method: "POST", path: "/api/agent/missing", status: 404, code: "not_found"},
		{method: "POST", path: "/api/agent/chat/", status: 404, code: "not_found"},
		{method: "POST", path: "/api/agent/Chat", status: 404, code: "not_found"},
		{method: "POST", path: "/api/demos/chat", status: 404, code: "not_found"},
		{method: "POST", path: "/api/demos/messages", status: 404, code: "not_found"},
		{method: "POST", path: "/api/demos/agent", status: 404, code: "not_found"},
		{method: "GET", path: "/api/demos/health", status: 404, code: "not_found"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, request)
			if out.Code != tc.status || out.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(out.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("实际状态 %d，头部 %v，正文 %s；预期状态 %d 和禁止缓存的 JSON 响应", out.Code, out.Header(), out.Body.String(), tc.status)
			}
			if tc.allow != "" {
				for _, method := range strings.Split(tc.allow, ",") {
					if !strings.Contains(out.Header().Get("Allow"), method) {
						t.Errorf("实际 Allow %q，预期包含 %s", out.Header().Get("Allow"), method)
					}
				}
			}
			if tc.code != "" {
				var response Response
				if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || response.Error == nil || response.Error.Code != tc.code {
					t.Errorf("实际错误响应 %s，预期 %s", out.Body.String(), tc.code)
				}
			} else if tc.method == "GET" && out.Body.String() != `{"status":"ok"}` {
				t.Errorf("实际健康响应 %s，预期 status=ok", out.Body.String())
			}
		})
	}
	if calls.Load() != 0 {
		t.Errorf("健康检查与无效路由触发了 %d 次模型请求，预期 0", calls.Load())
	}
}

func TestContentTypeValidation(t *testing.T) {
	var calls atomic.Int32
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		reply(w, map[string]string{"role": "assistant", "content": "正常回答"}, "stop")
	}, agent.DefaultLimits(), time.Second, &memoryExporter{})
	for _, contentType := range []string{"", "text/plain", "application/json; charset=utf-8"} {
		request := httptest.NewRequest(http.MethodPost, "/api/agent/chat", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", contentType)
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, request)
		want := http.StatusUnsupportedMediaType
		if contentType == "application/json; charset=utf-8" {
			want = http.StatusOK
		}
		if out.Code != want {
			t.Errorf("Content-Type %q 实际 HTTP %d，预期 %d；响应 %s", contentType, out.Code, want, out.Body.String())
		}
	}
	if calls.Load() != 1 {
		t.Errorf("模型调用 %d 次，预期只有合法 JSON 触发 1 次", calls.Load())
	}
}

func TestRecoveryDoesNotExposePanicOrRewriteResponse(t *testing.T) {
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { t.Error("异常路由不应请求模型") }, agent.DefaultLimits(), time.Second, &memoryExporter{})
	handler.GET("/panic", func(c *gin.Context) { panic("test-api-key") })
	handler.GET("/written-panic", func(c *gin.Context) {
		c.String(http.StatusAccepted, "已经写出的响应")
		panic("test-api-key")
	})
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set("X-API-Key", "header-secret")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, request)
	var response Response
	if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || out.Code != 500 || response.Error == nil || response.Error.Code != "internal_error" || response.Data.Completed {
		t.Fatalf("异常响应实际 %d %s，预期未完成的 internal_error", out.Code, out.Body.String())
	}
	if strings.Contains(out.Body.String(), "test-api-key") || strings.Contains(out.Body.String(), "header-secret") {
		t.Error("异常响应暴露了 panic 或请求头")
	}
	out = httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/written-panic", nil))
	if out.Code != http.StatusAccepted || out.Body.String() != "已经写出的响应" {
		t.Errorf("恢复流程覆盖了已写响应：%d %s", out.Code, out.Body.String())
	}
	out = httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if out.Code != http.StatusOK {
		t.Error("异常后健康检查不可用")
	}
}

func TestFailuresKeepStepsAndDoNotLeakCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		status     int
		handler    http.HandlerFunc
	}{
		{"服务错误", "model_request_failed", 502, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(503)
			io.WriteString(w, `{"api_key":"test-api-key","message":"Bearer hidden-value"}`)
		}},
		{"非 JSON", "invalid_model_response", 502, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "not json") }},
		{"回答截断", "invalid_model_response", 502, func(w http.ResponseWriter, r *http.Request) {
			reply(w, map[string]string{"role": "assistant", "content": "半句"}, "length")
		}},
		{"token 用量非法", "invalid_model_response", 502, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"回答"},"finish_reason":"stop"}],"usage":{"total_tokens":-1}}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, _ := setup(t, tc.handler, agent.DefaultLimits(), time.Second, &memoryExporter{})
			w, response := invoke(handler, "chat", `{}`)
			if w.Code != tc.status || response.Error == nil || response.Error.Code != tc.code || response.Data.Completed || len(response.Data.Steps) != 1 {
				t.Fatalf("失败响应 %d %s", w.Code, w.Body.String())
			}
			if bytes.Contains(w.Body.Bytes(), []byte("test-api-key")) || bytes.Contains(w.Body.Bytes(), []byte("hidden-value")) {
				t.Error("HTTP 响应泄漏了凭据")
			}
		})
	}
}

func TestRequestDeadlineAndCallerCancellation(t *testing.T) {
	for _, callerCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "总超时", true: "调用方取消"}[callerCancel], func(t *testing.T) {
			timeout := 50 * time.Millisecond
			if callerCancel {
				timeout = 5 * time.Second
			}
			started, finished, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
				close(finished)
			}, agent.DefaultLimits(), timeout, &memoryExporter{})
			t.Cleanup(func() { close(release) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, "/api/agent/chat", strings.NewReader(`{}`)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { handler.ServeHTTP(w, req); close(done) }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("模型没有进入等待")
			}
			if callerCancel {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("HTTP 处理未退出")
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("模型网络请求没有收到取消")
			}
			want := 504
			if callerCancel {
				want = 408
			}
			if w.Code != want {
				t.Fatalf("取消状态实际 %d，预期 %d", w.Code, want)
			}
		})
	}
}

func TestBudgetAndObservabilityFailureIsolation(t *testing.T) {
	model := func(w http.ResponseWriter, r *http.Request) {
		reply(w, map[string]string{"role": "assistant", "content": "记住了豆包。"}, "stop")
	}
	handler, telemetry := setup(t, model, agent.Limits{ModelCalls: 1, ToolCalls: 8, Tokens: 8000}, time.Second, &memoryExporter{fail: true})
	w, response := invoke(handler, "messages", `{}`)
	if w.Code != 409 || response.Error == nil || response.Error.Code != "budget_exceeded" || response.Data.ModelCalls != 1 || response.Data.Answer == "" {
		t.Fatalf("预算未保留已完成第一轮：%s", w.Body.String())
	}
	if err := telemetry.Flush(context.Background()); err != nil {
		t.Fatal("上报失败不应改变业务错误")
	}
	w, response = invoke(handler, "chat", `{}`)
	if w.Code != 200 || !response.Data.Completed {
		t.Fatalf("观测失败影响了后续业务：%s", w.Body.String())
	}
}

func TestConcurrentRunsDoNotShareHistory(t *testing.T) {
	handler, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		var input agent.Request
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(400)
			return
		}
		if len(input.Messages) != 2 {
			t.Errorf("一次聊天出现其他请求的历史：%d", len(input.Messages))
		}
		reply(w, map[string]string{"role": "assistant", "content": input.Messages[1].Content}, "stop")
	}, agent.DefaultLimits(), time.Second, &memoryExporter{})
	var wg sync.WaitGroup
	for _, question := range []string{"甲的独立问题", "乙的独立问题", "丙的独立问题"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, err := protojson.Marshal(&entity.ChatRequest{Question: &question})
			if err != nil {
				t.Errorf("编码聊天请求：%v", err)
				return
			}
			w, response := invoke(handler, "chat", string(body))
			if w.Code != 200 || response.Data.Answer != question || response.Data.ModelCalls != 1 || len(response.Data.Steps) != 1 {
				t.Errorf("并发运行混入其他结果：%s", w.Body.String())
			}
		}()
	}
	wg.Wait()
}
