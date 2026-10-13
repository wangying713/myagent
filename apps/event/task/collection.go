package task

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/defs"
	contentModel "git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/common/util"
	baseEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/entity"
	contentRepo "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	contentService "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	classicsEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/entity"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	errcode "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/database/mysql"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rainbow"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/mario"
	"git.code.oa.com/sport/content-pool/content_backend/model"
	contentInsert "git.code.oa.com/sport/content-pool/content_backend/service/content_insert"
	toolsDefs "git.code.oa.com/sport/content-pool/content_backend/tools/defs"
	areaDefs "git.code.oa.com/sport/feed/area/public/defs"
	areaModel "git.code.oa.com/sport/feed/area/public/model"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-go/log"
	pb "git.code.oa.com/trpcprotocol/sports/content_backend_content_insert"
	"github.com/samber/lo"
	"github.com/spf13/cast"
)

const (
	classicsSceneFlag  = "classicCollection" // 合集场景标识
	classicsItemsLimit = 100                 // 合集内容最大数量
)

// Collection 内容合集任务
type Collection interface {
	Handle(ctx context.Context, name, params string, seq int32) error
	InsertHandle(ctx context.Context, name, params string, seq int32) error
}

type collection struct {
	rainbow            rainbow.Client
	marioCli           mario.Mario
	taskCli            service.Client
	contentRepoCli     contentRepo.Content
	indexCli           contentService.Index
	collectionCli      classicsService.Collection
	collectionItemsCli classicsService.CollectionItems
	insertCli          contentInsert.Client
	feedCli            mysql.OperationFeedClient
}

// NewCollection 实例化
func NewCollection() Collection {
	return &collection{
		marioCli:           mario.NewMario(),
		taskCli:            service.NewClient(),
		indexCli:           contentService.NewIndex(),
		contentRepoCli:     contentRepo.NewContent(),
		collectionCli:      classicsService.NewCollection(),
		collectionItemsCli: classicsService.NewCollectionItems(),
		insertCli:          contentInsert.NewClient(),
		rainbow:            rainbow.NewClient(),
		feedCli:            mysql.NewMysqlXSportsAbtestClient(),
	}
}

// Handle 处理内容合
func (c *collection) Handle(ctx context.Context, _, _ string, seq int32) error {
	info, err := c.taskCli.Pop(ctx, defs.Collection, util.GetContainerName())
	if errors.Code(err) == errcode.NoData {
		return nil
	}
	log.InfoContextf(ctx, "collection.Handle start, param: %s", info.Param)
	param := &classicsEntity.CollectionTask{}
	if err = json.Unmarshal([]byte(info.Param), param); err != nil {
		// json 错误，丢弃错误
		log.ErrorContextf(ctx, "collection.Handle unmarshal err: %v, param: %v", err, info.Param)
		return nil
	}

	switch param.Type {
	case classicsEntity.CollectionOpTypePush:
		if err = c.publishCollection(ctx, param); err != nil {
			log.ErrorContextf(ctx, "collection.Handle fail, err: %v", err)
			return err
		}
	case classicsEntity.CollectionOpTypeDelete:
		if err = c.collectionDelete(ctx, param, info.Operator); err != nil {
			log.ErrorContextf(ctx, "collection.Handle fail, err: %v", err)
			return err
		}
	case classicsEntity.CollectionOpTypeUpsert:
		if err = c.upsertCollection(ctx, param.CollectionIDs); err != nil {
			log.ErrorContextf(ctx, "collection.Handle fail, err: %v", err)
			return err
		}
	default:
		// 不支持的操作类型
	}
	return c.taskCli.Commit(ctx, defs.Collection, info.TaskID, "")
}

// InsertHandle 强插合集内容任务
func (c *collection) InsertHandle(ctx context.Context, _, _ string, seq int32) error {
	// 更新强插合集
	if err := c.updateCollection(ctx); err != nil {
		// 忽略错误，下次可以继续更新
		log.ErrorContextf(ctx, "collection.Handle.updateCollection err: %v", err)
	}
	return nil
}

// upsertCollection 合集变更，变更信息推送至新任务
func (c *collection) upsertCollection(ctx context.Context, collectionIDs []uint32) error {
	list, err := c.collectionCli.List(ctx, &classicsEntity.CollectionReq{
		IDs:          collectionIDs,
		IgnoreDelete: true,
	})
	if err != nil {
		return err
	}
	for _, each := range list {
		items, err := c.collectionItemsCli.List(ctx, each.ID)
		if err != nil {
			return err
		}
		contentIDs := lo.Map(items, func(item *classicsEntity.CollectionItems, index int) string {
			return item.ContentID
		})
		if err = c.addTask(ctx, each, contentIDs); err != nil {
			log.ErrorContextf(ctx, "collection.addTask fail, err: %v, collectionID: %d", err, each.ID)
			return err
		}
	}
	return nil
}

func (c *collection) addTask(ctx context.Context, collection *classicsEntity.Collection, contentIDs []string) error {
	publishParam := &mario.PublishCollectionReq{
		ID:         fmt.Sprint(collection.ID),
		Name:       collection.Name,
		Type:       fmt.Sprint(collection.Type),
		IsDisabled: cast.ToString(cast.ToInt32(!collection.IsEnable())),
		ContentIDs: strings.Join(contentIDs, ","),
		UpdateTime: time.Now().Format(time.DateTime),
	}
	param := &classicsEntity.CollectionTask{
		Type:  classicsEntity.CollectionOpTypePush,
		Param: helper.JSONStr(publishParam),
	}
	return c.taskCli.Add(ctx, &entity.Info{
		Bucket:   defs.Collection,
		TaskID:   fmt.Sprintf("%s_%x", time.Now().Format(time.DateTime), md5.Sum([]byte(param.String()))),
		Param:    param.String(),
		Operator: "系统",
	})
}

func (c *collection) publishCollection(ctx context.Context, param *classicsEntity.CollectionTask) error {
	req := &mario.PublishCollectionReq{}
	if err := json.Unmarshal([]byte(param.Param), &req); err != nil {
		// json 错误，丢弃错误
		log.ErrorContextf(ctx, "publishCollection.Unmarshal err: %v, param: %+v", err, param)
		return nil
	}
	contentIDs := strings.Split(req.ContentIDs, ",")
	if len(contentIDs) > classicsItemsLimit {
		req.ContentIDs = strings.Join(contentIDs[:classicsItemsLimit], ",")
	}
	return c.marioCli.PublishCollection(ctx, req)
}

// collectionDelete 删除内容合集
// 删除内容合集后，内容合集里的内容需要把合集删除掉
// 这里有个特殊需求：如果内容的主合集被踢出，从次合集中取出最小的当做主合集
func (c *collection) collectionDelete(ctx context.Context, param *classicsEntity.CollectionTask, rtx string) error {
	for _, collectionID := range param.CollectionIDs {
		// 查询全部的内容
		items, err := c.collectionItemsCli.List(ctx, collectionID)
		if err != nil {
			log.InfoContextf(ctx, "collectionList fail, err: %v, collectinID: %d", err, collectionID)
			return err
		}
		contents, err := c.contentRepoCli.BatchGet(ctx, lo.Map(items, func(item *classicsEntity.CollectionItems, _ int) string {
			return item.ContentID
		}))
		if err != nil {
			return err
		}
		var list []*baseEntity.Content
		for _, each := range items {
			if _, ok := contents[each.ContentID]; !ok {
				continue
			}
			list = append(list, &baseEntity.Content{
				Detail: contents[each.ContentID],
			})
		}
		for _, each := range list {
			coll := each.Detail.Video.Collection
			if coll == nil {
				continue
			}
			ret := c.buildCollection(collectionID, coll)
			values := map[string]string{
				contentModel.FieldCollection: ret.String(),
			}
			newRtx := fmt.Sprintf("%s（删除合集触发）", rtx)
			if err = c.contentRepoCli.Modify(ctx, each.Detail.GetContentID(), newRtx, values); err != nil {
				log.ErrorContextf(ctx, "collectionDelete modify collection fail, err: %v", err)
				return err
			}
		}
	}
	return nil
}

// buildCollection 获得删除合集的合集结构
func (c *collection) buildCollection(collID uint32, coll *contentModel.Collection) *contentModel.Collection {
	ret := &contentModel.Collection{
		MainID: coll.MainID,
		SubIDs: coll.SubIDs,
	}
	// 主合集被踢出，从次合集中取出最小的当做主合集
	if coll.MainID == collID {
		ret.MainID = lo.Min(coll.SubIDs)
	}
	// 次合集被踢出
	ret.SubIDs = lo.Filter(coll.SubIDs, func(item uint32, _ int) bool {
		return item != collID && item != ret.MainID
	})
	return ret
}

type feedParam struct {
	id      int32
	info    *model.TbOperationFeed
	content *areaModel.ClassicModule
}

// getFeedParam 获取强插参数
// 目前只针对经典必看来获取，其他暂时不支持
func (c *collection) getFeedParam(ctx context.Context) (feedMap map[int32]*feedParam, todayList []int32, err error) {
	whereMap := map[string]interface{}{
		"action":     toolsDefs.TopFeed,
		"scene_flag": classicsSceneFlag,
		"type":       areaDefs.ClassicModule1150,
	}
	feedList, err := c.feedCli.QueryMap(ctx, whereMap, nil, "")
	if err != nil {
		return
	}
	feedMap = map[int32]*feedParam{}
	var param *model.FeedParam
	var classicsContent *areaModel.ClassicModule
	for _, each := range feedList {
		param, err = each.UnMarshalParam()
		if err != nil {
			return
		}
		classicsContent, err = param.UnmarshalContentToClassicModule()
		if err != nil {
			return
		}
		if classicsContent.CollectionType == 0 {
			ids := lo.Map(classicsContent.List, func(item *areaModel.CollectionItem, index int) int32 {
				return item.ID
			})
			todayList = append(todayList, ids...)
		}
		feedMap[classicsContent.CollectionType] = &feedParam{
			id:      each.ID,
			info:    each,
			content: classicsContent,
		}
	}
	return
}

// getCollectionTypeMap 获取合集列表，剔除今日特辑
func (c *collection) getCollectionTypeMap(ctx context.Context, todayList []int32) (map[int32][]*classicsEntity.Collection, error) {
	var types []string
	for _, typ := range c.rainbow.CollectionTypes() {
		if typ.Value == 0 {
			continue
		}
		types = append(types, fmt.Sprint(typ.Value))
	}
	// 查询全部合集
	list, err := c.collectionCli.List(ctx, &classicsEntity.CollectionReq{
		Type: strings.Join(types, ","),
	})
	if err != nil {
		return nil, err
	}
	ret := make(map[int32][]*classicsEntity.Collection)
	for _, each := range list {
		if lo.Contains(todayList, int32(each.ID)) {
			continue
		}
		typ := int32(each.Type)
		ret[typ] = append(ret[typ], each)
	}
	return ret, nil
}

// updateCollection 跟新强插合集数据
// 把合集的数据查出来，剔除今日特辑，保留顺序，更新到强插合集里面
func (c *collection) updateCollection(ctx context.Context) error {
	paramMap, todayList, err := c.getFeedParam(ctx)
	if err != nil {
		return err
	}
	collectionMap, err := c.getCollectionTypeMap(ctx, todayList)
	if err != nil {
		return err
	}
	t := time.Now()
	for typ, each := range collectionMap {
		newList := lo.Map(each, func(item *classicsEntity.Collection, index int) *areaModel.CollectionItem {
			return &areaModel.CollectionItem{
				ID: int32(item.ID),
				Filter: areaModel.Filter{
					StartTime:  t.Format(time.DateTime),
					EndTime:    t.AddDate(100, 0, 0).Format(time.DateTime),
					UpdateTime: t.Format(time.DateTime),
					Operator:   "系统",
					IsDisabled: false,
				},
			}
		})
		req := &areaModel.ClassicModule{
			CollectionType:     typ,
			CollectionTypeName: c.rainbow.CollectionTypeDesc(typ),
		}
		if oldList, ok := paramMap[typ]; ok {
			req.List = c.mergeList(newList, oldList.content.List)
			return c.insertCli.FeedUpdate(ctx, &pb.FeedUpdateReq{
				IgnoreEmpty: true,
				Id:          oldList.id,
				Param: &pb.FeedParam{
					Content: req.String(),
					AdInfos: "[]",
				},
			})
		}
		err = c.insertCli.FeedCreate(ctx, &pb.FeedCreateReq{
			Type:      areaDefs.ClassicModule1150,
			SceneFlag: classicsSceneFlag,
			StartTime: t.Format(time.DateTime),
			EndTime:   t.AddDate(100, 0, 0).Format(time.DateTime),
			Action:    string(toolsDefs.TopFeed),
			Param: &pb.FeedParam{
				Content: req.String(),
				AdInfos: "[]",
			},
		})
	}
	return nil
}

// mergeList 合并列表数据
// 1. 保留老的顺序
// 2. 新的覆盖旧的
// 3. 新的插入到老的后面
func (c *collection) mergeList(newList, oldList []*areaModel.CollectionItem) []*areaModel.CollectionItem {
	// 创建一个 map 用于快速查找 oldList 中的 ID 和对应的 Item
	oldMap := make(map[int32]*areaModel.CollectionItem)
	for _, item := range oldList {
		oldMap[item.ID] = item
	}
	newMap := make(map[int32]*areaModel.CollectionItem)
	for _, item := range newList {
		newMap[item.ID] = item
	}
	addedIDs := make(map[int32]bool)
	var result []*areaModel.CollectionItem
	// 保留老的顺序
	for _, each := range oldList {
		if _, ok := newMap[each.ID]; ok {
			result = append(result, each)
			addedIDs[each.ID] = true
		}
	}
	// 把新的塞进去
	for _, each := range newList {
		if _, ok := addedIDs[each.ID]; ok {
			continue
		}
		result = append(result, each)
	}
	return result
}
