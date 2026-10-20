package app

import (
	"errors"
	"net/http"
	"time"

	"saklay/ai/apps/interfaces/http/entity"
	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

func newRouter(agentCli *agentService, timeout time.Duration, observer httpObserver) (*gin.Engine, error) {
	implAgent, err := newImplAgent(agentCli)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, errors.New("HTTP 请求超时必须为正数")
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
	router.Use(traceHTTP(observer), previewHTTP(observer), requestTimeout(timeout))
	// 不输出请求头、panic 原文或堆栈，且不覆盖已经写出的响应。
	router.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		c.Abort()
		if !c.Writer.Written() {
			writeHTTPRsp(c, http.StatusInternalServerError, nil, &httpx.Error{
				Code:    entity.CodeInternalError,
				Message: "服务处理异常，请稍后重试",
			})
		}
	}))
	router.NoRoute(func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusNotFound, nil, &httpx.Error{
			Code:    entity.CodeNotFound,
			Message: "接口不存在",
		})
	})
	router.NoMethod(func(c *gin.Context) {
		writeHTTPRsp(c, http.StatusMethodNotAllowed, nil, &httpx.Error{
			Code:    entity.CodeMethodNotAllowed,
			Message: "接口不支持该请求方法",
		})
	})

	// 路由逐条声明完整地址，便于从 URL 直接定位具名处理方法。
	implAgentDemos := newImplAgentDemos(agentCli)
	router.GET("/api/health", health)
	router.HEAD("/api/health", health)
	router.POST("/api/agent/chat", handle(implAgent.Chat))
	router.POST("/api/agent/messages", handle(implAgent.Messages))
	router.POST("/api/agent/run", handle(implAgent.Run))
	router.POST("/api/agent-demos/overview", handle(implAgentDemos.Overview))
	router.POST("/api/agent-demos/chat", handle(implAgentDemos.Chat))
	router.POST("/api/agent-demos/messages", handle(implAgentDemos.Messages))
	router.POST("/api/agent-demos/tools", handle(implAgentDemos.Tools))
	router.POST("/api/agent-demos/structured", handle(implAgentDemos.Structured))
	router.POST("/api/agent-demos/multiple-tools", handle(implAgentDemos.MultipleTools))
	router.POST("/api/agent-demos/stream", handleAgentDemosStream(implAgentDemos.Stream))
	router.POST("/api/agent-demos/reliability", handle(implAgentDemos.Reliability))
	router.POST("/api/agent-demos/retrieval", handle(implAgentDemos.Retrieval))
	router.POST("/api/agent-demos/workflow", handle(implAgentDemos.Workflow))
	router.POST("/api/agent-demos/observation", handle(implAgentDemos.Observation))
	router.POST("/api/agent-demos/evaluation", handle(implAgentDemos.Evaluation))
	router.POST("/api/agent-demos/delivery", handle(implAgentDemos.Delivery))
	return router, nil
}

func health(c *gin.Context) {
	writeHTTPRsp(c, http.StatusOK, map[string]string{"status": "ok"}, nil)
}
