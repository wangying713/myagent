package task

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/defs"
	"git.code.oa.com/sport/content-pool/content_backend/common/util"
	baseEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/entity"
	baseService "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/base/value"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	errcode "git.code.oa.com/sport/content-pool/content_backend/errors"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/cos"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rainbow"
	"git.code.oa.com/sport/content-pool/content_backend/tools"
	"git.code.oa.com/sport/sports-match/go-helper/helper"
	"git.code.oa.com/sport/trpc-go/errors"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/samber/lo"
	cosv5 "github.com/tencentyun/cos-go-sdk-v5"
)

// ContentExport 内容导出
type ContentExport interface {
	Handle(ctx context.Context, name, params string, seq int32) error
}

type contentExport struct {
	taskCli   service.Client
	exportSer baseService.Export
	cosCli    cos.UploadPart
}

// NewContentExport 创建内容导出任务 client
func NewContentExport() ContentExport {
	return &contentExport{
		taskCli:   service.NewClient(),
		exportSer: baseService.NewExport(),
		cosCli:    cos.NewUploadPart(cos.NewRainbow().GetSecretCos()),
	}
}

// Handle 数据导出
func (s *contentExport) Handle(ctx context.Context, _, _ string, _ int32) error {
	info, err := s.taskCli.Pop(ctx, defs.ContentExportName, util.GetContainerName())
	if errors.Code(err) == errcode.NoData {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "contentExport task pop err")
	}
	stepParam, rsp, err := s.exportHandle(ctx, info)
	if err != nil {
		return err
	}
	ob, err := s.uploadPart(ctx, stepParam, rsp)
	if err != nil {
		return err
	}
	// 将上传的分块存入 StepParam
	stepParam.Objects = append(stepParam.Objects, ob)
	if info.Step >= info.StepMax { // 导出完成
		if err = s.commit(ctx, info.Operator, stepParam); err != nil {
			return err
		}
	}
	return s.taskCli.Commit(ctx, defs.ContentExportName, info.TaskID, stepParam.String())
}

// senEmail 发送邮件
func (s *contentExport) sendMail(ctx context.Context, title, msgContent string, rtx string) error {
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

func (s *contentExport) commit(ctx context.Context, operator string, stepParam *value.StepParam) error {
	shareURL, err := s.cosCli.Commit(ctx, stepParam.Filename, stepParam.UploadID, stepParam.Objects)
	if err != nil {
		return err
	}
	// 发送邮件
	log.InfoContextf(ctx, "operator = %s ,shareURL = %s", operator, shareURL)
	msgContent := fmt.Sprintf("请点击下列url（8小时后失效）进行导出数据的下载：\n%s", shareURL)
	return s.sendMail(ctx, fmt.Sprintf("%s导出成功", stepParam.Filename), msgContent, operator)
}

func (s *contentExport) uploadPart(ctx context.Context, stepParam *value.StepParam, rsp *baseEntity.ExportRsp) (cosv5.Object, error) {
	if len(rsp.Infos) == 0 {
		return cosv5.Object{}, nil
	}
	// 查询当前存储的最大 part number
	obMax := lo.MaxBy(stepParam.Objects, func(a cosv5.Object, b cosv5.Object) bool {
		return a.PartNumber > b.PartNumber
	})
	// 创建 buffer
	buffer := &bytes.Buffer{}
	csvCli := csv.NewWriter(buffer)
	if obMax.PartNumber == 0 {
		buffer.WriteString(util.BOM)
		_ = csvCli.Write(rsp.Handle.Head())
	}
	_ = csvCli.WriteAll(rsp.Infos)
	csvCli.Flush()
	// 上传
	return s.cosCli.Part(ctx, &cos.PartReq{
		Name:     stepParam.Filename,
		UploadID: stepParam.UploadID,
		Number:   obMax.PartNumber + 1,
		Reader:   buffer,
	})
}

func (s *contentExport) exportHandle(ctx context.Context,
	pop *entity.Info) (*value.StepParam, *baseEntity.ExportRsp, error) {
	log.InfoContextf(ctx, "contentExport start ,info = %s", helper.JSONStr(pop))
	stepParam, err := value.UnmarshalStepParam(pop.StepParam)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "contentExport.stepParam")
	}
	req, err := baseEntity.UnmarshalOfflineExport(pop.Param)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "contentExport")
	}
	req.Param.Param.Limit = baseService.IndexScrollMax
	req.Param.Param.ScrollData = stepParam.ScrollData
	// 查询需要导出数据
	rsp, err := s.exportSer.Export(ctx, req.Param)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "contentExport")
	}
	// 组装｜更新  StepParam
	handle := rsp.Handle
	stepParam.Filename = lo.Ternary(stepParam.Filename != "", stepParam.Filename, handle.FileName())
	stepParam.ScrollData = rsp.ScrollData
	// 首次上传初始化分块
	if stepParam.UploadID == "" {
		stepParam.UploadID, err = s.cosCli.Init(ctx, stepParam.Filename)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "contentExport.Init")
		}
	}
	return stepParam, rsp, nil
}
