// Package task 任务
package task

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	"git.code.oa.com/trpc-go/trpc-go/log"
)

// Client 内容任务客户端
type Client interface {
	Flow(ctx context.Context, name, params string, seq int32) error
}

type client struct {
	taskCli service.Client
}

// NewClient 创建内容任务 client
func NewClient() Client {
	return &client{
		taskCli: service.NewClient(),
	}
}

// Flow 任务流转
func (s *client) Flow(ctx context.Context, _, _ string, _ int32) error {
	log.InfoContextf(ctx, "task flow start")
	return s.taskCli.Flow(ctx)
}
