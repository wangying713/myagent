// Package task 修改权威资讯分类
package task

import (
	"context"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/defs"
	"git.code.oa.com/sport/content-pool/content_backend/common/util"
	authorityService "git.code.oa.com/sport/content-pool/content_backend/domain/content/authority/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/authority/value"
	contentService "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	errcode "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-go/log"
)

// Authority 权威资讯任务接口
type Authority interface {
	MoveCate(ctx context.Context, name, params string, seq int32) error
}

type authority struct {
	taskCli      service.Client
	authoritySer authorityService.Authority
	contentCli   contentService.Content
}

// NewAuthority 权威资讯任务实例
func NewAuthority() Authority {
	return &authority{
		taskCli:      service.NewClient(),
		authoritySer: authorityService.NewAuthority(),
		contentCli:   contentService.NewContent(),
	}
}

// MoveCate 定时修改权威资讯分类
func (s *authority) MoveCate(ctx context.Context, _, _ string, _ int32) error {
	info, err := s.taskCli.Pop(ctx, defs.AuthorityPoolName, util.GetContainerName())
	if errors.Code(err) == errcode.NoData {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "authority task pop err")
	}
	log.InfoContextf(ctx, "authority move cate start ,info = %s", helper.JSONStr(info))
	cateTask, err := value.UnmarshalCateTask(info.Param)
	if err != nil {
		return err
	}
	// 获取分类内容 contentID 信息
	contentIDs, err := s.authoritySer.GetIndexByCate(ctx, cateTask.Old.ID)
	if err != nil {
		return err
	}
	// 获取内容信息
	list, err := s.contentCli.BatchGet(ctx, contentIDs)
	if err != nil {
		return err
	}
	// 修改分类
	for _, v := range list {
		if err = s.authoritySer.MoveCate(ctx, cateTask, v.Detail, "authority_move_cate"); err != nil {
			return errors.Wrap(err, "authority move cate err")
		}
	}
	return s.taskCli.Commit(ctx, defs.AuthorityPoolName, info.TaskID, "")
}
