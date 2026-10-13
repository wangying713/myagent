package consumer

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	authorityService "git.code.oa.com/sport/content-pool/content_backend/domain/content/authority/service"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/IBM/sarama"
)

// kafkaProducer kafka生产者
type kafkaProduce struct {
	kafkaCli kafka.Client
}

// NewKafkaProduce 实例化
func NewKafkaProduce() kafka.KafkaConsumer {
	return &kafkaProduce{
		// 目前只有一个，暂时先这样写，后续可以扩展至自己的实例
		kafkaCli: kafka.NewClientProxy("trpc.kakfa.produce.yuanbao"),
	}
}

// ContentProduceHandle 内容处理
type ContentProduceHandle interface {
	Check(ctx context.Context, info *model.ContentDiff) (bool, error)
	Handle(ctx context.Context, info *model.ContentDiff) (string, error)
}

// contentProduces 内容处理注册
var contentProduces = []ContentProduceHandle{
	authorityService.NewPushMsg(),
}

// Handle 处理
func (c *kafkaProduce) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	log.InfoContextf(ctx, "ContentProduceHandle.Handle key[%s] topic[%s] partition[%d] offset[%d] timestamp[%s] value[%s]",
		msg.Key, msg.Topic, msg.Partition, msg.Offset, msg.Timestamp, msg.Value)
	diffOriginal, err := model.UnmarshalDiffOriginal(msg.Value)
	if err != nil {
		return err
	}
	diff := diffOriginal.ToDiff(ctx)
	if diff == nil {
		// 这种的就忽略了
		return nil
	}
	for _, handle := range contentProduces {
		if err = c.Produce(ctx, diff, handle); err != nil {
			log.InfoContextf(ctx, "kafkaProduce.Produce err: %v, diff: %s", err, helper.JSONStr(diff))
			return err
		}
	}
	return nil
}

// Produce 产生消息内容
func (c *kafkaProduce) Produce(ctx context.Context, diff *model.ContentDiff, handle ContentProduceHandle) error {
	ok, err := handle.Check(ctx, diff)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	m, err := handle.Handle(ctx, diff)
	if err != nil {
		return err
	}
	// 没有数据的就忽略了
	if m == "" {
		return nil
	}
	err = c.kafkaCli.Produce(ctx, []byte(diff.Detail.ID), []byte(m))
	if err != nil {
		return err
	}
	log.InfoContextf(ctx, "kafkaProduce success, key: %s, val: %s", diff.Detail.ID, m)
	return nil
}
