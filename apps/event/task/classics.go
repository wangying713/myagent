// Package task 修改权威资讯分类
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.code.oa.com/sport/content-pool/content_backend/application/interfaces"
	"git.code.oa.com/sport/content-pool/content_backend/common/content/defs"
	commonModel "git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/common/util"
	baseEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/entity"
	contentRepo "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	contentService "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	errcode "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rainbow"
	"git.code.oa.com/sport/content-pool/content_backend/tools"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-go/log"
)

const (
	maxLimit = 1000 // 单次最大查询数量
)

// Classics 经典内容池任务接口
type Classics interface {
	HandleImportPool(ctx context.Context, name, params string, seq int32) error
}

type classics struct {
	taskCli        service.Client
	contentRepoCli contentRepo.Content
	indexCli       contentService.Index
	modifyCli      interfaces.ContentModify
}

// NewClassics 经典内容池任务实例
func NewClassics() Classics {
	return &classics{
		taskCli:        service.NewClient(),
		indexCli:       contentService.NewIndex(),
		contentRepoCli: contentRepo.NewContent(),
		modifyCli:      classicsService.NewModifyClassicsType(),
	}
}

type infoMsg struct {
	successIDs []string
	errIDs     []string
	skipIDs    []string
	task       *entity.Info
	total      uint32
}

// HandleImportPool 根据筛选条件，将数据导入至经典内容池，操作来源于合格池导入经典内容池
func (c *classics) HandleImportPool(ctx context.Context, _, _ string, _ int32) error {
	info, err := c.taskCli.Pop(ctx, defs.ClassicsPoolName, util.GetContainerName())
	if err != nil {
		if errors.Code(err) == errcode.NoData {
			return nil
		}
		if err != nil {
			return errors.Wrapf(err, "classics task pop err")
		}
	}
	log.InfoContextf(ctx, "classics_import_task start")
	req := &baseEntity.IndexScroll{}
	if err = json.Unmarshal([]byte(info.Param), req); err != nil {
		return errors.Wrapf(err, "classics_task_pop param err")
	}
	log.InfoContextf(ctx, "classics_task_pop param: %s", helper.JSONStr(req))
	msg := &infoMsg{
		task: info,
	}
	// 单次处理数量
	loopNum := req.Limit / maxLimit
	req.Limit = maxLimit
	for i := 0; i <= int(loopNum); i++ {
		ret, err := c.indexCli.ListScroll(ctx, req)
		log.InfoContextf(ctx, "classics_task_list scroll: %s， err: %v", helper.JSONStr(ret), err)
		if err != nil {
			return err
		}
		for _, each := range ret.List {
			msg.total++
			if each.Detail.Classics.Type.Validate() {
				msg.skipIDs = append(msg.skipIDs, each.Detail.ID)
				continue
			}
			if err = c.modifyClassicsType(ctx, commonModel.ClassicsTypePending, each, info.Operator); err != nil {
				msg.errIDs = append(msg.errIDs, each.Detail.ID)
				log.ErrorContextf(ctx, "modifyClassicsType err: %v, each: %s", err, helper.JSONStr(each))
				continue
			}
			msg.successIDs = append(msg.successIDs, each.Detail.ID)
		}
		if msg.total >= ret.Total {
			break
		}
		req.ScrollData = ret.ScrollData
	}

	title, content := c.formatMsg(ctx, msg)
	if err = senEmail(ctx, title, content, info.Operator); err != nil {
		// 只是邮件发从失败，记录日志，忽略错误
		log.ErrorContextf(ctx, "sendMsg err: %v", err)
	}
	log.InfoContextf(ctx, "classics task pop end")
	return c.taskCli.Commit(ctx, defs.ClassicsPoolName, info.TaskID, "")
}

// modifyClassicsType 经典内容审核状态变更
func (c *classics) modifyClassicsType(ctx context.Context, classicsType commonModel.ClassicsType, info *baseEntity.Content, rtx string) error {
	param := &interfaces.ContentModifyReq{
		Value: fmt.Sprint(classicsType),
		Rtx:   rtx,
		Info:  info.Detail,
	}
	values, err := c.modifyCli.Handle(ctx, param)
	if err != nil {
		return err
	}
	return c.contentRepoCli.Modify(ctx, info.Detail.GetContentID(), rtx, values)
}

// sendContentFmt 发送邮件内容格式
var sendContentFmt = `
<html>
	<body>
		<h3>经典内容候选池导入任务%s已完成</h3>
		<p><b>任务概要:</b></p>
	<ul>
		<li>筛选到数据: %d条</li>
		<li>成功导入: %d条</li>
		<li>跳过已在池中: %d条</li>
		<li>导入失败: %d条</li>
	</ul>
	
	<p><b>详细ID列表:</b></p>
	<ol>
		<li>成功导入的内容ID:<br/>%s</li>
		<li>跳过已在池中的内容ID:<br/>%s</li>
		<li>导入失败的内容ID:<br/>%s</li>
		</ol>
	</body>
</html>`

// formatMsg 格式获得发送内容的数据消息
func (c *classics) formatMsg(ctx context.Context, msg *infoMsg) (title, content string) {
	titleFmt := "经典内容候选池导入任务%s已完成，成功导入内容%d条，跳过已在经典内容池中的内容%d条"
	title = fmt.Sprintf(titleFmt, msg.task.TaskID, len(msg.successIDs), len(msg.skipIDs))
	content = fmt.Sprintf(sendContentFmt,
		msg.task.TaskID,
		msg.total,
		len(msg.successIDs),
		len(msg.skipIDs),
		len(msg.errIDs),
		strings.Join(msg.successIDs, "\n"),
		strings.Join(msg.skipIDs, "\n"),
		strings.Join(msg.errIDs, "\n"))
	return
}

// senEmail 发送邮件
func senEmail(ctx context.Context, title, msgContent string, rtx string) error {
	mailConf, err := rainbow.GetGalaxyContentMailConf()
	if err != nil {
		return err
	}
	var maxRetryTimes = 3
	_, err = tools.SendWithRetry(ctx, maxRetryTimes, mailConf.URL, mailConf.PaasID, mailConf.Token,
		&tools.Tof4EmailInfo{
			Title:      title,
			Content:    msgContent,
			From:       mailConf.From,
			To:         rtx,
			BodyFormat: 1,
		})
	return err
}
