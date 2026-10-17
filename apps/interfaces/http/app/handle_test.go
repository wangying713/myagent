package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/apps/interfaces/http/proto"

	"github.com/gin-gonic/gin"
)

func TestHandleKeepsResponseWithError(t *testing.T) {
	for _, tc := range []struct {
		name   string         // 用例名称。
		rsp    *proto.ChatRsp // 模拟 API 返回的完整或部分结果。
		err    error          // 模拟 API 同时返回的错误。
		status int            // 预期 HTTP 状态码。
	}{
		{
			name:   "成功",
			rsp:    &proto.ChatRsp{Answer: "完整结果"},
			status: 200,
		},
		{
			name:   "部分结果",
			rsp:    &proto.ChatRsp{Answer: "已完成部分"},
			err:    errors.New("内部敏感错误"),
			status: 502,
		},
		{
			name:   "无结果",
			err:    errors.New("内部敏感错误"),
			status: 502,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(requestTimeout(time.Second))
			router.POST("/api/agent/chat", handle(func(_ context.Context, req *proto.ChatReq) (*proto.ChatRsp, error) {
				if req.GetQuestion() != "请求" {
					t.Errorf("实际问题 %q，预期 请求", req.GetQuestion())
				}
				return tc.rsp, tc.err
			}))
			out, _ := invoke(router, "chat", `{"question":"请求"}`)
			var rsp entity.Rsp[*entity.AgentExecutionResult]
			if err := json.Unmarshal(out.Body.Bytes(), &rsp); err != nil {
				t.Fatal(err)
			}
			if out.Code != tc.status || (rsp.Data == nil) != (tc.rsp == nil) {
				t.Fatalf("实际 HTTP %d、响应 %s，预期 HTTP %d 且保留已有数据", out.Code, out.Body.String(), tc.status)
			}
			if tc.rsp != nil && rsp.Data.Answer != tc.rsp.Answer {
				t.Errorf("实际回答 %q，预期 %q", rsp.Data.Answer, tc.rsp.Answer)
			}
			if strings.Contains(out.Body.String(), "内部敏感错误") {
				t.Error("响应泄漏了内部错误")
			}
		})
	}
}

func TestHandleUsesRouterTimeoutAndReleasesContext(t *testing.T) {
	var apiContext context.Context
	api := handle(func(ctx context.Context, _ *proto.ChatReq) (*proto.ChatRsp, error) {
		apiContext = ctx
		return nil, nil
	})
	short, long := gin.New(), gin.New()
	short.Use(requestTimeout(time.Second))
	long.Use(requestTimeout(5 * time.Second))
	short.POST("/api/agent/chat", api)
	long.POST("/api/agent/chat", api)
	for _, tc := range []struct {
		name    string        // 用例名称。
		router  *gin.Engine   // 应用当前超时配置的测试路由。
		timeout time.Duration // 预期传入业务方法的超时时长。
	}{
		{
			name:    "短超时",
			router:  short,
			timeout: time.Second,
		},
		{
			name:    "长超时",
			router:  long,
			timeout: 5 * time.Second,
		},
		{
			name:    "再次短超时",
			router:  short,
			timeout: time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			out, _ := invoke(tc.router, "chat", `{}`)
			after := time.Now()
			if out.Code != http.StatusOK || apiContext == nil {
				t.Fatalf("实际 HTTP %d，预期执行 API 并成功返回", out.Code)
			}
			deadline, ok := apiContext.Deadline()
			if !ok || deadline.Before(before.Add(tc.timeout)) || deadline.After(after.Add(tc.timeout)) {
				t.Errorf("实际截止时间 %v，预期使用当前路由的 %v 超时", deadline, tc.timeout)
			}
			if !errors.Is(apiContext.Err(), context.Canceled) {
				t.Error("API 返回后未释放上下文")
			}
		})
	}
}

func TestHandleRejectsMissingTimeoutAndUnsupportedResponse(t *testing.T) {
	for _, configured := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "缺少超时配置",
			true:  "未适配响应类型",
		}[configured], func(t *testing.T) {
			router := gin.New()
			if configured {
				router.Use(requestTimeout(time.Second))
			}
			called := false
			router.POST("/unsupported", handle(func(context.Context, *proto.ChatReq) (*proto.Usage, error) {
				called = true
				return &proto.Usage{}, nil
			}))
			req := httptest.NewRequest(http.MethodPost, "/unsupported", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			var rsp entity.Rsp[any]
			if err := json.Unmarshal(out.Body.Bytes(), &rsp); err != nil {
				t.Fatal(err)
			}
			if out.Code != 500 || rsp.Code != "internal_error" || rsp.Data != nil || called != configured {
				t.Errorf("实际 HTTP %d、响应 %s、调用 %v，预期统一错误且仅配置完整时调用 API", out.Code, out.Body.String(), called)
			}
		})
	}
}
