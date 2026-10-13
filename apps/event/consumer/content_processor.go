package consumer

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	contentRepo "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	crawlService "git.code.oa.com/sport/content-pool/content_backend/domain/content/crawl/service"
	inPoolService "git.code.oa.com/sport/content-pool/content_backend/domain/content/team_memory/service"
	errcodes "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/trpc-go/errors"
	pkgErrors "git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/IBM/sarama"
	"github.com/samber/lo"
)

// contentProcessor 根据流水做内容变更逻辑处理
type contentProcessor struct {
	contentRepoCli contentRepo.Content
}

// NewContentProcessorConsumer 内容实例化
func NewContentProcessorConsumer() kafka.KafkaConsumer {
	return &contentProcessor{
		contentRepoCli: contentRepo.NewContent(),
	}
}

// ContentProcessorHandle 内容处理
type ContentProcessorHandle interface {
	// Handle 处理 慎重返回 error
	Handle(ctx context.Context, info *model.ContentDiff) (map[string]string, error)
}

// contentProcessors 内容处理注册
var contentProcessors = []ContentProcessorHandle{
	classicsService.NewAutoCandidate(), // 经典内容池自动入候选池
	classicsService.NewInitTime(),      // 经典内容池审核时间
	inPoolService.NewInPool(),          // 懂球故事入候选池
	crawlService.NewCompetitor(),       // 抓取内容豁免配置
	crawlService.NewNamiTransfer(),     // 纳米世界杯内容转移
}

// Handle 处理
func (s *contentProcessor) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	log.InfoContextf(ctx, "contentProcessor Handle key[%s] topic[%s] partition[%d] offset[%d] timestamp[%s] value[%s]",
		msg.Key, msg.Topic, msg.Partition, msg.Offset, msg.Timestamp, msg.Value)
	diffOriginal, err := model.UnmarshalDiffOriginal(msg.Value)
	if err != nil {
		return err
	}
	diff := diffOriginal.ToDiff(ctx)
	if diff == nil {
		return errors.Errorf(errcodes.ParameterIllegal, "diff is nil")
	}
	values := make(map[string]string)
	for _, handle := range contentProcessors {
		m, err := handle.Handle(ctx, diff)
		if err != nil {
			return err
		}
		values = lo.Assign(values, m)
	}
	// 调用 content域 repo ModifyField 方法
	err = s.contentRepoCli.SetField(ctx, diff.Detail.GetContentID(), model.RtxContentBackend.String(), values)
	return pkgErrors.Wrap(err, "update.ModifyField")
}
