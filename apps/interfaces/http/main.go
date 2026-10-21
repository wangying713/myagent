// HTTP 入口处理启动参数、配置、退出信号和最终错误，服务由 app 运行。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"saklay/ai/apps/config"
	"saklay/ai/apps/interfaces/http/app"
)

func main() {
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
	return app.Run(ctx, *cfg)
}
