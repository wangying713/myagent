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

// Run 使用显式配置组装依赖，运行到服务停止，并按顺序释放资源。
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
	client, err := deepseek.New(cfg.Model, telemetry, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	service, err := agent.NewService(client, telemetry, cfg.Limits)
	if err != nil {
		return err
	}
	handler, err := newRouter(service, cfg.HTTP.RunTimeout, telemetry)
	if err != nil {
		return err
	}
	return runHTTP(ctx, handler, cfg.HTTP)
}
