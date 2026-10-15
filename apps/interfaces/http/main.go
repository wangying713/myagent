// HTTP 入口组装配置、模型、观测和三课，运行期间共用连接，每次请求独立执行。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saklay/ai/apps/config"
	"saklay/ai/domain/agent"
	"saklay/ai/infra/deepseek"
	"saklay/ai/infra/openobserve"

	"github.com/gin-gonic/gin"
)

const telemetryFlushTimeout = 5 * time.Second

func main() {
	// 框架运行模式由入口设置，避免路由构造修改全局状态。
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "启动或运行失败：", err)
		os.Exit(1)
	}
}

func execute(ctx context.Context) error {
	envFile := flag.String("env-file", "", "显式配置文件路径，环境变量优先")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("不接受位置参数，请使用 -env-file 指定配置文件")
	}
	cfg, err := config.Load(*envFile, os.LookupEnv)
	if err != nil {
		return err
	}
	return run(ctx, cfg)
}

func run(ctx context.Context, cfg config.Settings) error {
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
	handler, err := newRouter(service, cfg.HTTP.RunTimeout)
	if err != nil {
		return err
	}
	return runHTTP(ctx, handler, cfg.HTTP)
}
