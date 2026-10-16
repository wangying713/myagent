package app

import (
	"errors"
	"net/http"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

func newRouter(service *agent.Service, timeout time.Duration, observer httpObserver) (*gin.Engine, error) {
	agentAPI, err := newAgentAPI(service)
	if err != nil {
		return nil, err
	}
	chatAdapter, err := httpx.NewAdapter(timeout, chatRspJSON, classifyAgentError, writeHTTPRsp)
	if err != nil {
		return nil, err
	}
	messagesAdapter, err := httpx.NewAdapter(timeout, messagesRspJSON, classifyAgentError, writeHTTPRsp)
	if err != nil {
		return nil, err
	}
	runAdapter, err := httpx.NewAdapter(timeout, runRspJSON, classifyAgentError, writeHTTPRsp)
	if err != nil {
		return nil, err
	}
	if observer == nil {
		return nil, errors.New("HTTP 观测依赖不能为空")
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	router.Use(traceHTTP(observer))
	// 不输出请求头、panic 原文或堆栈，且不覆盖已经写出的响应。
	router.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		c.Abort()
		if !c.Writer.Written() {
			writeHTTPRsp(c, http.StatusInternalServerError, nil, &httpx.Error{Code: entity.CodeInternalError, Message: "服务处理异常，请稍后重试"})
		}
	}))
	router.NoRoute(func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusNotFound, nil, &httpx.Error{Code: entity.CodeNotFound, Message: "接口不存在"})
	})
	router.NoMethod(func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusMethodNotAllowed, nil, &httpx.Error{Code: entity.CodeMethodNotAllowed, Message: "接口不支持该请求方法"})
	})

	// 路由逐条声明完整地址，便于从 URL 直接定位具名处理方法。
	router.GET("/api/health", health)
	router.HEAD("/api/health", health)
	router.POST("/api/agent/chat", httpx.Adapt(chatAdapter, agentAPI.Chat))
	router.POST("/api/agent/messages", httpx.Adapt(messagesAdapter, agentAPI.Messages, prepareAgentMessages))
	router.POST("/api/agent/run", httpx.Adapt(runAdapter, agentAPI.Run))
	return router, nil
}

func health(c *gin.Context) {
	writeHTTPRsp(c, http.StatusOK, map[string]string{"status": "ok"}, nil)
}
