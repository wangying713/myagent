// Package task 修改权威资讯分类
package task

import (
	"context"
	"time"

	"git.code.oa.com/sport/content-pool/content_backend/application/interfaces"
	contentModel "git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/common/model"
	authorityService "git.code.oa.com/sport/content-pool/content_backend/domain/content/authority/service"
	baseEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/entity"
	contentRepo "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	contentService "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
)

// 扫描数据开始发文时间
var authorityStartTime = time.Date(2023, 10, 1, 0, 0, 0, 0, time.Local)

// HistoryAuthorityYuanbao 权威资讯历史数据推送元宝
type HistoryAuthorityYuanbao interface {
	Handle(ctx context.Context, name, params string, seq int32) error
}

type historyAuthorityYuanbao struct {
	taskCli        service.Client
	contentRepoCli contentRepo.Content
	indexCli       contentService.Index
	modifyCli      interfaces.ContentModify
	authorityCli   *authorityService.PushMsg
	kafkaCli       kafka.Client
}

// NewHistoryAuthorityYuanbao 实例化
func NewHistoryAuthorityYuanbao() HistoryAuthorityYuanbao {
	return &historyAuthorityYuanbao{
		taskCli:        service.NewClient(),
		indexCli:       contentService.NewIndex(),
		contentRepoCli: contentRepo.NewContent(),
		modifyCli:      classicsService.NewModifyClassicsType(),
		authorityCli:   authorityService.NewPushMsg(),
		kafkaCli:       kafka.NewClientProxy("trpc.kakfa.produce.yuanbao"),
	}
}

func (c *historyAuthorityYuanbao) Handle(ctx context.Context, _, _ string, _ int32) error {
	req := c.getReq()
	log.InfoContextf(ctx, "historyAuthorityYuanbao.handle start, req: %s", helper.JSONStr(req))
	// 单次处理数量
	loopNum := req.Limit / maxLimit
	req.Limit = maxLimit
	for i := 0; i <= int(loopNum); i++ {
		log.InfoContextf(ctx, "historyAuthorityYuanbao scroll 批次: %d, req: %s", i, helper.JSONStr(req))
		ret, err := c.indexCli.ListScroll(ctx, req)
		if err != nil {
			return err
		}
		for _, each := range ret.List {
			err := c.Produce(ctx, each.Detail)
			if err != nil {
				return err
			}
		}
		req.ScrollData = ret.ScrollData
	}
	log.InfoContext(ctx, "historyAuthorityYuanbao.handle end")
	return nil
}

func (c *historyAuthorityYuanbao) getReq() *baseEntity.IndexScroll {
	return &baseEntity.IndexScroll{
		IndexParam: &baseEntity.IndexParam{
			Index: baseEntity.EsIndexAll,
			SportsCtime: &model.TimeRange{
				Start: authorityStartTime,
				End:   time.Now(),
			},
			AuthorityType: []string{"201"},
			IsDisabled:    "0",
			Limit:         120000,
		},
		ScrollData: "",
	}
}

// Produce 产生消息内容
func (c *historyAuthorityYuanbao) Produce(ctx context.Context, detail *contentModel.Content) error {
	diff := &contentModel.ContentDiff{
		Detail: detail,
		Diff: map[string]*contentModel.Diff{
			contentModel.FieldAuthorityType: {},
		},
	}
	ok, err := c.authorityCli.Check(ctx, diff)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	val, err := c.authorityCli.Handle(ctx, diff)
	if err != nil {
		return err
	}
	// 没有数据的就忽略了
	if val == "" {
		log.InfoContextf(ctx, "historyAuthorityYuanbao.Produce no data, key: %s", diff.Detail.ID)
		return nil
	}
	err = c.kafkaCli.Produce(ctx, []byte(diff.Detail.ID), []byte(val))
	if err != nil {
		return err
	}
	log.InfoContextf(ctx, "historyAuthorityYuanbao.Produce success, key: %s, val: %s", diff.Detail.ID, val)
	return nil
}
