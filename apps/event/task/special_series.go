package task

import (
	"context"
	"fmt"
	"sort"
	"time"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/defs"
	commonModel "git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/common/model"
	"git.code.oa.com/sport/content-pool/content_backend/domain/series/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/series/repo"
	"git.code.oa.com/sport/content-pool/content_backend/domain/series/service"
	errcode "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/comment"
	contentQuery "git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/content_query"
	contentStorage "git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/content_storage"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/indexBuilder"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/sport/trpc-go/xsync"
	"git.code.oa.com/trpc-go/trpc-go/errs"
	pbBuilder "git.code.oa.com/trpcprotocol/content_pool/index_builder_sports_content_pool_index_builder"
	pbComment "git.code.oa.com/trpcprotocol/hermes/comment_hermes_comment"
	pbStorage "git.code.oa.com/trpcprotocol/sport/content_storage"
	pbBasic "git.code.oa.com/trpcprotocol/sport_feed/common_basic"
	"git.woa.com/sport/common/content"
	pkgContent "git.woa.com/sport/common/content"
	"github.com/samber/lo"
	"github.com/spf13/cast"
)

// SpecialSeries 专题任务
type SpecialSeries interface {
	RefreshSeriesStats(ctx context.Context, name, params string, seq int32) error
}

type specialSeries struct {
	seriesSer      service.SpecialSeries
	seriesHisRepo  repo.SpecialSeriesHistory
	pbProxy        contentQuery.Client
	contentStorage contentStorage.Client
	commentCli     comment.Client
	indexBuilder   indexBuilder.Client
}

// NewSpecialSeries 创建内容任务 client
func NewSpecialSeries() SpecialSeries {
	return &specialSeries{
		seriesSer:      service.NewSpecialSeries(),
		seriesHisRepo:  repo.NewSpecialSeriesHistory(),
		pbProxy:        contentQuery.NewClient(),
		contentStorage: contentStorage.NewClient(),
		commentCli:     comment.NewClient(),
		indexBuilder:   indexBuilder.NewClient(),
	}
}

// RefreshSeriesStats  获取专题内容 阅读数量 && 评论数量
func (s *specialSeries) RefreshSeriesStats(ctx context.Context, _, _ string, _ int32) error {
	// 查询生效中的专题内容
	ret, err := s.seriesSer.List(ctx, &entity.SpecialSeriesListReq{
		Status: []model.TimeRangeStatus{model.TimeRangeStatusProcessing},
		Limit:  100,
	})
	if err != nil {
		return err
	}
	for _, v := range ret.List {
		// 获取专题内容 ID
		rsp, err := s.seriesSer.Contents(ctx, v.ID)
		if err != nil {
			return errors.Wrap(err, "refreshSeriesStats.ContentIDs")
		}
		contentIDs := lo.Map(rsp.List, func(item *entity.Content, _ int) string {
			return item.ContentID
		})
		// 统计专题内容数据
		if err = s.stats(ctx, v.ID, contentIDs); err != nil {
			return errors.Wrap(err, "refreshSeriesStats.stats")
		}
		// 更新专题评论列表
		if err = s.updateComment(ctx, v.ID, contentIDs); err != nil {
			return errors.Wrap(err, "refreshSeriesStats.updateComment")
		}
		// 更新Feed封面图
		if err = s.updateCoverPic(ctx, v, rsp.List); err != nil {
			return errors.Wrap(err, "refreshSeriesStats.updateCoverPic")
		}
	}
	return nil
}

// updateComment 更新专题评论列表 【保证评论数量与评论列表更新频率一致】
func (s *specialSeries) updateComment(ctx context.Context, sid string, contentIDs []string) error {
	rootIds := lo.Map(contentIDs, func(contentID string, _ int) string {
		_, id := defs.GenSrcAndID(contentID)
		return id
	})
	rootIds = append(rootIds, sid)
	sync := xsync.NewAsync(time.Second)
	for _, v := range rootIds {
		rootId := v
		sync.Add(cast.ToString(rootId), func() (interface{}, error) {
			// 获取内容详情
			rsp, err := s.commentCli.List(ctx, &pbComment.HListRequest{
				RootId: rootId,
				Count:  100,
			})
			if err != nil {
				return nil, errors.Wrapf(err, "updateComment.List rootId = %s", rootId)
			}
			var index []*pbBuilder.IndexItem
			arr := append(rsp.Cards, rsp.EliteCards...)
			for _, item := range arr {
				id := item.GetComment().GetId()
				if id == "" {
					continue
				}
				index = append(index, &pbBuilder.IndexItem{
					K: item.GetComment().GetId(),
					S: cast.ToFloat64(item.GetComment().GetExtendInfo()["score"]),
				})
			}
			return index, nil
		})
	}
	var index []*pbBuilder.IndexItem
	_, err := sync.WaitWithErr(func(tagID string, d interface{}) {
		index = append(d.([]*pbBuilder.IndexItem), index...)
	})
	if len(err) > 0 {
		return errs.Newf(errcode.InternalErr, "updateComment err = %v", helper.JSONStr(err))
	}
	// 去重
	index = lo.UniqBy(index, func(item *pbBuilder.IndexItem) string {
		return item.GetK()
	})
	// 排序
	sort.SliceStable(index, func(i, j int) bool {
		if index[i].GetS() == index[j].GetS() {
			return index[i].GetK() > index[j].GetK() // 分数相同，按ID降序
		}
		return index[i].GetS() < index[j].GetS() // 按分数升序
	})
	// 保存索引
	return s.indexBuilder.IndexUpdateAll(ctx, &pbBuilder.IndexUpdateAllReq{
		Pk:        fmt.Sprintf("100_13027_%s", sid),
		From:      "content_backend",
		IndexItem: lo.Subset(index, 0, 1000),
	})
}

// stats 统计数据
func (s *specialSeries) stats(ctx context.Context, sid string, contentIDs []string) error {
	// 计算专题内容阅读数量
	value, err := s.getValue(ctx, sid, contentIDs)
	if err != nil {
		return errors.Wrapf(err, "refreshSeriesStats getReads sid = %s", sid)
	}
	if len(value) == 0 {
		return nil
	}
	// 更新内容阅读数量
	err = s.contentStorage.SetField(ctx, &pbStorage.SetFieldReq{
		Src:    int32(content.SrcSpecialSeries),
		Id:     sid,
		Remark: "special_series_stats",
		Value:  value,
	})
	return errors.Wrap(err, "refreshSeriesStats.SetField")
}

// getValue 获取专题内容阅读数量 明细
func (s *specialSeries) getValue(ctx context.Context, sid string, contentIDs []string) (map[string]string, error) {
	// 获取内容详情
	infos, err := s.pbProxy.Details(ctx, contentIDs)
	if err != nil {
		return nil, err
	}

	readNum, err := s.getReadNum(ctx, sid, infos)
	if err != nil {
		return nil, err
	}
	replyNum, err := s.getReplyNum(ctx, sid, infos)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		commonModel.FieldReadNum: cast.ToString(readNum),
		pkgContent.FieldReplyNum: cast.ToString(replyNum),
	}, nil
}

// getReplyNum 获取回复数量
func (s *specialSeries) getReplyNum(ctx context.Context, sid string, infos map[string]*content.Info) (int64, error) {
	num, err := s.commentCli.Count(ctx, sid)
	if err != nil {
		return 0, err
	}
	for _, item := range infos {
		num += cast.ToInt32(item.GetValue(pkgContent.FieldReplyNum))
	}
	return int64(num), nil
}

// getReplyNum 获取回复数量
func (s *specialSeries) getReadNum(ctx context.Context, sid string, infos map[string]*content.Info) (int64, error) {
	var (
		uniq = make(map[string]struct{})
		res  []*repo.HistoryReadNumber
	)
	// 获取专题内容阅读数量
	for _, item := range infos {
		res = append(res, &repo.HistoryReadNumber{
			ContentID:  item.ContentID(),
			ReadNumber: item.GetPageView(),
		})
		uniq[item.ContentID()] = struct{}{}
	}

	// 获取专题内容历史评论数量
	history, err := s.seriesHisRepo.GetReadNumber(ctx, sid)
	if err != nil {
		return 0, err
	}
	// 获取历史阅读数量&&排除重复
	for _, v := range history {
		if _, ok := uniq[v.ContentID]; ok {
			continue
		}
		res = append(res, v)
	}
	// 保存历史阅读数量
	if err = s.seriesHisRepo.SaveReadNumber(ctx, sid, res); err != nil {
		return 0, errors.Wrap(err, "special_series_read.SaveReadNumber")
	}
	return lo.SumBy(res, func(item *repo.HistoryReadNumber) int64 {
		return cast.ToInt64(item.ReadNumber)
	}), nil
}

// updateCoverPic 刷新封面图
func (s *specialSeries) updateCoverPic(ctx context.Context, info *entity.SpecialSeries, specialContents []*entity.Content) error {
	// 锁定封面图
	if lo.Contains(info.LockField, entity.LockFieldCoverPicture) {
		return nil
	}
	specialContent, ok := lo.Find(specialContents, func(item *entity.Content) bool {
		src, _ := defs.GenSrcAndID(item.ContentID)
		return lo.Contains([]content.Src{content.SrcNews, content.SrcTopic}, src)
	})
	if !ok {
		return nil
	}
	pic := specialContent.Pic
	// 如果相同，不进行更新
	if pic == info.CoverPicture.GetLight() {
		return nil
	}
	info.CoverPicture = &pbBasic.LightDark{
		Light: pic,
		Dark:  "",
	}
	_, err := s.seriesSer.Save(ctx, info)
	return err
}
