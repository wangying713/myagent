package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"sakelei/ai/apps/interfaces/http/api"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/httpx"
)

func newRouter(service *agent.Service, timeout time.Duration) (*gin.Engine, error) {
	agentAPI, err := api.NewAgent(service, timeout)
	if err != nil {
		return nil, err
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	// 不输出请求头、panic 原文或堆栈，且不覆盖已经写出的响应。
	router.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		c.Abort()
		if !c.Writer.Written() {
			httpx.WriteResult(c, http.StatusInternalServerError, nil, &httpx.Error{Code: "internal_error", Message: "服务处理异常，请稍后重试"})
		}
	}))
	router.NoRoute(func(c *gin.Context) {
		httpx.WriteResult(c, http.StatusNotFound, nil, &httpx.Error{Code: "not_found", Message: "接口不存在"})
	})
	router.NoMethod(func(c *gin.Context) {
		httpx.WriteResult(c, http.StatusMethodNotAllowed, nil, &httpx.Error{Code: "method_not_allowed", Message: "接口不支持该请求方法"})
	})

	// 路由逐条声明完整地址，便于从 URL 直接定位具名处理方法。
	router.GET("/api/health", health)
	router.HEAD("/api/health", health)
	router.POST("/api/agent/chat", agentAPI.Chat)
	router.POST("/api/agent/messages", agentAPI.Messages)
	router.POST("/api/agent/run", agentAPI.Run)
	return router, nil
}

func health(c *gin.Context) {
	httpx.Write(c, http.StatusOK, map[string]string{"status": "ok"})
}
