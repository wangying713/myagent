package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/typepb"
)

type typeResult struct {
	Name string `json:"name"`
}

func typeAdapter(t *testing.T, timeout time.Duration) *httpx.Adapter[*typeResult] {
	t.Helper()
	adapter, err := httpx.NewAdapter(timeout, func(response *typeResult) any { return response }, func(err error) (int, *httpx.Error) {
		switch {
		case err == nil:
			return http.StatusOK, nil
		case errors.Is(err, context.Canceled):
			return http.StatusRequestTimeout, &httpx.Error{Code: "canceled", Message: "执行已取消"}
		case errors.Is(err, context.DeadlineExceeded):
			return http.StatusGatewayTimeout, &httpx.Error{Code: "timeout", Message: "执行超时"}
		default:
			return http.StatusBadGateway, &httpx.Error{Code: "upstream_failed", Message: "上游调用失败"}
		}
	}, writeTestRsp)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func typeRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/types", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestAdaptKeepsResponseWithError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *typeResult
		err      error
		status   int
	}{
		{name: "成功", response: &typeResult{Name: "完整结果"}, status: 200},
		{name: "错误与部分结果", response: &typeResult{Name: "已完成部分"}, err: errors.New("内部敏感错误"), status: 502},
		{name: "错误与空响应", err: errors.New("内部敏感错误"), status: 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := typeAdapter(t, time.Second)
			router := gin.New()
			router.POST("/types", httpx.Adapt(adapter, func(ctx context.Context, req *typepb.Type) (*typeResult, error) {
				if req.GetName() != "请求" {
					t.Errorf("实际请求名 %q，预期 请求", req.GetName())
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Error("API 上下文没有截止时间")
				}
				return tc.response, tc.err
			}))
			out := httptest.NewRecorder()
			router.ServeHTTP(out, typeRequest(`{"name":"请求"}`))
			var response testRsp[*typeResult]
			if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if out.Code != tc.status || (response.Data == nil) != (tc.response == nil) {
				t.Fatalf("实际 HTTP %d、正文 %s，预期 HTTP %d 并保留响应", out.Code, out.Body.String(), tc.status)
			}
			if tc.response != nil && response.Data.Name != tc.response.Name {
				t.Errorf("实际数据 %+v，预期 %+v", response.Data, tc.response)
			}
			if tc.err == nil && response.Error != nil || tc.err != nil && (response.Error == nil || response.Error.Code != "upstream_failed") {
				t.Errorf("实际错误 %+v，预期按注入的规则映射", response.Error)
			}
			if strings.Contains(out.Body.String(), "内部敏感错误") || out.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("响应泄漏内部错误或未禁止缓存：%s", out.Body.String())
			}
		})
	}
}

func TestAdaptPreparesRequestAfterSuccessfulDecode(t *testing.T) {
	adapter := typeAdapter(t, time.Second)
	router := gin.New()
	prepared, called := false, false
	router.POST("/types", httpx.Adapt(adapter, func(_ context.Context, req *typepb.Type) (*typeResult, error) {
		called = true
		if !prepared || req.Oneofs == nil {
			t.Error("API 没有收到保留显式空数组的请求")
		}
		return &typeResult{Name: req.GetName()}, nil
	}, func(req *typepb.Type, present map[string]bool) {
		prepared = true
		if present["oneofs"] && len(req.Oneofs) == 0 {
			req.Oneofs = []string{}
		}
	}))
	out := httptest.NewRecorder()
	router.ServeHTTP(out, typeRequest(`{"oneofs":[]}`))
	if out.Code != 200 || !called || !prepared {
		t.Fatalf("实际 HTTP %d、准备 %v、调用 %v，预期成功准备并调用", out.Code, prepared, called)
	}
	prepared, called = false, false
	out = httptest.NewRecorder()
	router.ServeHTTP(out, typeRequest(`{"unknown":true}`))
	var response testRsp[*typeResult]
	if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if out.Code != 400 || called || prepared || response.Data != nil || response.Error == nil || response.Error.Code != "invalid_input" {
		t.Errorf("非法输入实际 HTTP %d、正文 %s、准备 %v、调用 %v，预期提前拒绝", out.Code, out.Body.String(), prepared, called)
	}
}

func TestAdaptPropagatesCancellationAndCancelsAfterReturn(t *testing.T) {
	for _, callerCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "超时", true: "调用方取消"}[callerCancel], func(t *testing.T) {
			timeout := 50 * time.Millisecond
			if callerCancel {
				// 主动取消用例不与短截止时间竞争；一秒的退出检查仍约束测试。
				timeout = 5 * time.Second
			}
			adapter := typeAdapter(t, timeout)
			router := gin.New()
			started := make(chan struct{})
			router.POST("/types", httpx.Adapt(adapter, func(ctx context.Context, _ *typepb.Type) (*typeResult, error) {
				close(started)
				<-ctx.Done()
				return &typeResult{Name: "取消前的结果"}, ctx.Err()
			}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			out := httptest.NewRecorder()
			done := make(chan struct{})
			go func() {
				router.ServeHTTP(out, typeRequest(`{}`).WithContext(ctx))
				close(done)
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("API 未开始执行")
			}
			if callerCancel {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("API 未响应取消")
			}
			want := 504
			if callerCancel {
				want = 408
			}
			if out.Code != want || !strings.Contains(out.Body.String(), "取消前的结果") {
				t.Errorf("实际 HTTP %d、正文 %s，预期 HTTP %d 并保留部分结果", out.Code, out.Body.String(), want)
			}
		})
	}

	adapter := typeAdapter(t, time.Second)
	var apiContext context.Context
	handler := httpx.Adapt(adapter, func(ctx context.Context, _ *typepb.Type) (*typeResult, error) {
		apiContext = ctx
		return nil, nil
	})
	out := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(out)
	ctx.Request = typeRequest(`{}`)
	handler(ctx)
	if apiContext == nil || !errors.Is(apiContext.Err(), context.Canceled) {
		t.Error("请求结束后没有释放 API 上下文")
	}
}

func TestAdaptDoesNotShareRequestOrResponse(t *testing.T) {
	adapter := typeAdapter(t, time.Second)
	router := gin.New()
	router.POST("/types", httpx.Adapt(adapter, func(_ context.Context, req *typepb.Type) (*typeResult, error) {
		return &typeResult{Name: req.GetName()}, nil
	}))
	var wg sync.WaitGroup
	for _, name := range []string{"甲", "乙", "丙"} {
		wg.Go(func() {
			out := httptest.NewRecorder()
			body, err := json.Marshal(map[string]string{"name": name})
			if err != nil {
				t.Error(err)
				return
			}
			router.ServeHTTP(out, typeRequest(string(body)))
			var response testRsp[*typeResult]
			if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || out.Code != 200 || response.Data == nil || response.Data.Name != name {
				t.Errorf("请求 %q 实际响应 %d %s，预期结果相互独立", name, out.Code, out.Body.String())
			}
		})
	}
	wg.Wait()
}

func TestNewAdapterRejectsInvalidConfig(t *testing.T) {
	encode := func(response *typeResult) any { return response }
	classify := func(error) (int, *httpx.Error) { return http.StatusOK, nil }
	if _, err := httpx.NewAdapter(0, encode, classify, writeTestRsp); err == nil {
		t.Error("零超时未被拒绝")
	}
	if _, err := httpx.NewAdapter[*typeResult](time.Second, nil, classify, writeTestRsp); err == nil {
		t.Error("缺少响应编码未被拒绝")
	}
	if _, err := httpx.NewAdapter(time.Second, encode, nil, writeTestRsp); err == nil {
		t.Error("缺少错误映射未被拒绝")
	}
}

func TestNewAdapterRequiresWriter(t *testing.T) {
	if _, err := httpx.NewAdapter(time.Second, func(rsp *typeResult) any { return rsp }, func(error) (int, *httpx.Error) { return 200, nil }, nil); err == nil {
		t.Error("缺少响应写出函数未被拒绝")
	}
}
