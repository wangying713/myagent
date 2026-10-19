// Package app 组装并运行 HTTP 应用，管理服务、模型和观测资源的生命周期。
package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"saklay/ai/apps/config"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/deepseek"
	"saklay/ai/infra/openobserve"

	"github.com/gin-gonic/gin"
)

const telemetryFlushTimeout = 5 * time.Second

// Run 使用显式配置组装并运行 HTTP 应用，ctx 取消时执行有界退出和资源清理。
// 正常停止返回 nil；依赖初始化、监听或 HTTP 清理失败时返回错误。
// 观测刷新失败仅输出提示，不覆盖原有的退出结果。
func Run(ctx context.Context, cfg config.Settings) error {
	// 应用启动时设置运行模式，路由构造不修改框架全局状态。
	gin.SetMode(gin.ReleaseMode)
	telemetry, err := openobserve.New(ctx, cfg.Observe, nil, os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), telemetryFlushTimeout)
		defer cancel()
		if telemetry.Close(flushCtx) != nil {
			fmt.Fprintln(os.Stderr, "[观测] 退出刷新未完成，请按 trace ID 核查")
		}
	}()
	chatCli, err := deepseek.New(cfg.Model, telemetry, nil)
	if err != nil {
		return err
	}
	defer chatCli.Close()
	agentCli, err := agent.NewClient(chatCli, telemetry, cfg.Limits)
	if err != nil {
		return err
	}
	handler, err := newRouter(agentCli, cfg.HTTP.RunTimeout, telemetry)
	if err != nil {
		return err
	}
	return runHTTP(ctx, handler, cfg.HTTP)
}
