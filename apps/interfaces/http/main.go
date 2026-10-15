// HTTP 入口组装配置、模型、观测和三课，运行期间共用连接，每次请求独立执行。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sakelei/ai/apps/config"
	"sakelei/ai/domain/agent"
	"sakelei/ai/infra/deepseek"
	"sakelei/ai/infra/openobserve"

	"github.com/gin-gonic/gin"
)

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
	telemetry, err := openobserve.New(ctx, cfg.Observe, nil, os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	listener, err := net.Listen("tcp", cfg.HTTP.Address)
	if err != nil {
		return errors.New("HTTP 端口无法监听，请检查地址及端口占用")
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: cfg.HTTP.RunTimeout + 10*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	fmt.Fprintln(os.Stdout, "Go 后台接口已启动：http://"+listener.Addr().String())
	select {
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("HTTP 服务异常退出")
		}
		return nil
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		server.Close()
		<-serveErrors
		return errors.New("HTTP 服务退出超时")
	}
	<-serveErrors
	return nil
}
