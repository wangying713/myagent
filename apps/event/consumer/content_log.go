package consumer

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/log/entity"
	serviceLog "git.code.oa.com/sport/content-pool/content_backend/domain/content/log/service"
	errcodes "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/IBM/sarama"
)

// contentLog 内容日志
type contentLog struct {
	log serviceLog.Client
}

// NewContentLogConsumer 内容日志实例化
func NewContentLogConsumer() kafka.KafkaConsumer {
	return &contentLog{
		log: serviceLog.NewClient(),
	}
}

// ContentLogHandle 内容日志
type ContentLogHandle interface {
	// Handle 处理 慎重返回 error
	Handle(ctx context.Context, info *model.ContentDiff) ([]*entity.Log, error)
}

// contentLogs 内容日志注册
var contentLogs = []ContentLogHandle{
	service.NewLogContentLevel(),         // 定级日志
	service.NewLogTimeliness(),           // 分发时效日志
	classicsService.NewLogClassicsType(), // 经典内容审核状态
	classicsService.NewLogClassicsTag(),  // 经典内容标签变更
	classicsService.NewLogCollection(),   // 合集变更
}

// Handle 处理
func (s *contentLog) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	log.InfoContextf(ctx, "contentLog Handle key[%s] topic[%s] partition[%d] offset[%d] timestamp[%s] value[%s]",
		msg.Key, msg.Topic, msg.Partition, msg.Offset, msg.Timestamp, msg.Value)
	diffOriginal, err := model.UnmarshalDiffOriginal(msg.Value)
	if err != nil {
		return err
	}
	diff := diffOriginal.ToDiff(ctx)
	if diff == nil {
		return errors.Errorf(errcodes.ParameterIllegal, "diff is nil")
	}
	var logs []*entity.Log
	for _, handle := range contentLogs {
		rsp, err := handle.Handle(ctx, diff)
		if err != nil {
			return err
		}
		if rsp == nil {
			continue
		}
		logs = append(logs, rsp...)
	}
	return s.log.Add(ctx, diff.Detail.GetContentID(), logs)
}
