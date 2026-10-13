package consumer

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/monitor/service"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/IBM/sarama"
)

// contentNotify 根据流水做内容通知处理
type contentNotify struct {
	cli service.Client
}

// NewContentNotifyConsumer 内容通知实例化
func NewContentNotifyConsumer() kafka.KafkaConsumer {
	return &contentNotify{
		cli: service.NewClient(),
	}
}

// ContentNotifyHandle 内容处理
type ContentNotifyHandle interface {
	// Handle 处理 慎重返回 error
	Handle(ctx context.Context, info *model.ContentDiff) (map[string]string, error)
}

// Handle 处理
func (s *contentNotify) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	log.InfoContextf(ctx, "contentNotify.Handle key: %s, value: %s", msg.Key, string(msg.Value))
	diffOriginal, err := model.UnmarshalDiffOriginal(msg.Value)
	if err != nil {
		// 忽略错误记录日志
		log.ErrorContextf(ctx, "contentNotify.Handle unmarshal failed skip err: %v", err)
		return nil
	}
	diff := diffOriginal.ToDiff(ctx)
	if diff == nil {
		log.ErrorContextf(ctx, "contentNotify.Handle diffOriginal diff is nil, val: %+v", diffOriginal)
		return nil
	}
	err = s.cli.ContentNotify(ctx, diff)
	if err != nil {
		log.ErrorContextf(ctx, "contentNotify.Handle ContentNotify failed err: %v", err)
		return err
	}
	return nil
}
