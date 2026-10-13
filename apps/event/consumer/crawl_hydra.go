package consumer

import (
	"context"
	"encoding/json"
	"strings"

	"git.code.oa.com/sport/content-pool/content_backend/domain/content/monitor/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/monitor/service"
	"git.code.oa.com/sport/content-pool/content_backend/tools/defs"
	"git.code.oa.com/trpc-go/trpc-database/kafka"
	"git.code.oa.com/trpc-go/trpc-go/log"
	"github.com/IBM/sarama"
)

// CrawlHydra 内容日志
type CrawlHydra struct {
	cli service.Client
}

// NewCrawlHydraConsumer 内容日志实例化
func NewCrawlHydraConsumer() kafka.KafkaConsumer {
	return &CrawlHydra{
		cli: service.NewClient(),
	}
}

// Handle 处理
func (s *CrawlHydra) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	log.InfoContextf(ctx, "CrawlHydra.Handle key: %s value: %s", msg.Key, string(msg.Value))
	value := &crawlHydraMsg{}
	if err := json.Unmarshal(msg.Value, value); err != nil {
		// 忽略错误记录日志，脏数据丢掉
		log.ErrorContextf(ctx, "CrawlHydra.Handle unmarshal failed skip err: %v", err)
		return nil
	}
	createdAt, _ := defs.StrToTime(value.PubTime)
	updatedAt, _ := defs.StrToTime(value.UpdatedAt)
	err := s.cli.HydraNotify(ctx, &entity.RawReq{
		RawID:   value.RawId,
		Title:   value.StaticExt.Title,
		Content: value.StaticExt.Content,
		URL:     value.StaticExt.Url,
		Images:  value.GetImageIDs(),
		Account: &entity.Account{
			Name:   value.StaticExt.Author,
			Avatar: value.StaticExt.CpAvatar,
			URL:    value.StaticExt.CpUrl,
			ID:     value.StaticExt.CpEname,
		},
		Source:     entity.Source(value.Source),
		CreatedAt:  createdAt,
		UpdateTime: updatedAt,
	})
	if err != nil {
		log.ErrorContextf(ctx, "CrawlHydra.Handle HydraNotify err: %v", err)
		return err
	}
	return nil
}

type crawlHydraMsg struct {
	RawId     string `json:"raw_id"`     // 原始数据ID（示例值：32位哈希值）
	PubTime   string `json:"pub_time"`   // 内容发布时间（示例值："2025-07-30 11:57:27"）
	Mid       string `json:"mid"`        // 平台内容ID（示例值："2772543750"）
	Source    string `json:"source"`     // 内容来源平台（固定值："twitter"）
	CreatedAt string `json:"created_at"` // 记录创建时间（示例值："2025-07-30 11:58:24"）
	UpdatedAt string `json:"updated_at"` // 记录更新时间（示例值："2025-07-31 17:32:56"）
	StaticExt struct {
		TmsBizId       string             `json:"tms_biz_id"`             // 业务 ID
		RawTraceId     string             `json:"raw_trace_id"`           // 原始追踪ID（示例值："e0745aacd988781f45c873740e2ba4e7"）
		OriRawID       string             `json:"ori_raw_id"`             // 转发 ID
		QuoteRawID     string             `json:"quote_raw_id"`           // 引用 ID
		Url            string             `json:"url"`                    // 内容来源URL（示例值：推特链接）
		Title          string             `json:"title"`                  // 内容标题
		Content        string             `json:"content"`                // 内容文字
		Author         string             `json:"author"`                 // 作者名称（示例值："StatMuse"）
		CpUrl          string             `json:"cp_url"`                 // 内容发布者主页URL（示例值：推特主页）
		CpAvatar       string             `json:"cp_avatar"`              // 发布者头像URL（示例值：图片地址）
		CpIntroduction string             `json:"cp_introduction"`        // 发布者简介（示例值：统计类媒体简介）
		CpEname        string             `json:"cp_ename"`               // 发布者英文名称（示例值："statmuse"）
		IsVerified     int                `json:"is_verified"`            // 认证状态（0未认证 1已认证）
		VerifiedType   string             `json:"verified_type"`          // 认证类型（示例值："2"）
		Imgs           []string           `json:"imgs"`                   // 内容图片URL列表（示例值：推文配图地址）
		Abstract       string             `json:"abstract"`               // 内容摘要（示例值：同标题的摘要信息）
		BizExt         string             `json:"biz_ext"`                // 业务扩展图片URL（示例值：带签名的COS存储图片）
		ArticleType    string             `json:"article_type"`           // 内容类型（示例值："news"）
		Trace          string             `json:"trace"`                  // 追踪标识（示例值："caller-hydra"）
		Images         []*crawlHydraImage `json:"download_img_callbacks"` // 下载图片回调数据
	} `json:"static_ext"` // 静态扩展信息（爬取时固定的元数据）
}

func (crawlHydraMsg *crawlHydraMsg) GetImageIDs() []string {
	var ids []string
	for _, img := range crawlHydraMsg.StaticExt.Images {
		if img.IsScreenshot() {
			continue
		}
		ids = append(ids, img.ImageId)
	}
	return ids
}

// DownloadImage 下载的图片信息
type crawlHydraImage struct {
	ImageId           string `json:"image_id"`            // 图片唯一ID（32位哈希值）
	Url               string `json:"url"`                 // 图片源地址
	DownloadStatusStr string `json:"download_status_str"` // 下载状态（"0"表示成功）
	Width             int    `json:"width"`               // 图片宽度（像素）
	Height            int    `json:"height"`              // 图片高度（像素）
	FileSize          int    `json:"file_size"`           // 文件大小（字节）
	ImageType         string `json:"image_type"`          // 图片格式（jpeg/png等）
}

// IsScreenshot 是否是截图
func (c *crawlHydraImage) IsScreenshot() bool {
	return strings.Contains(c.Url, "beijing.myqcloud.com")
}
