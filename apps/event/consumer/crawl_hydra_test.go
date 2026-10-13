package consumer

import (
	"reflect"
	"testing"
)

func TestUnit_crawlHydraImage_IsScreenshot(t *testing.T) {
	type fields struct {
		ImageId           string
		Url               string
		DownloadStatusStr string
		Width             int
		Height            int
		FileSize          int
		ImageType         string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "命中截图域名返回true",
			fields: fields{
				Url: "https://foo.beijing.myqcloud.com/bar.jpg",
			},
			want: true,
		},
		{
			name: "普通图片地址返回false",
			fields: fields{
				Url: "https://pbs.twimg.com/media/test.jpg",
			},
			want: false,
		},
		{
			name:   "空地址返回false",
			fields: fields{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &crawlHydraImage{
				ImageId:           tt.fields.ImageId,
				Url:               tt.fields.Url,
				DownloadStatusStr: tt.fields.DownloadStatusStr,
				Width:             tt.fields.Width,
				Height:            tt.fields.Height,
				FileSize:          tt.fields.FileSize,
				ImageType:         tt.fields.ImageType,
			}
			if got := c.IsScreenshot(); got != tt.want {
				t.Errorf("IsScreenshot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnit_crawlHydraMsg_GetImageIDs(t *testing.T) {
	type fields struct {
		RawId     string
		PubTime   string
		Mid       string
		Source    string
		CreatedAt string
		UpdatedAt string
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
		}
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "没有图片返回空",
			want: nil,
		},
		{
			name: "过滤截图并保留非截图顺序",
			fields: fields{
				StaticExt: struct {
					TmsBizId       string             `json:"tms_biz_id"`
					RawTraceId     string             `json:"raw_trace_id"`
					OriRawID       string             `json:"ori_raw_id"`
					QuoteRawID     string             `json:"quote_raw_id"`
					Url            string             `json:"url"`
					Title          string             `json:"title"`
					Content        string             `json:"content"`
					Author         string             `json:"author"`
					CpUrl          string             `json:"cp_url"`
					CpAvatar       string             `json:"cp_avatar"`
					CpIntroduction string             `json:"cp_introduction"`
					CpEname        string             `json:"cp_ename"`
					IsVerified     int                `json:"is_verified"`
					VerifiedType   string             `json:"verified_type"`
					Imgs           []string           `json:"imgs"`
					Abstract       string             `json:"abstract"`
					BizExt         string             `json:"biz_ext"`
					ArticleType    string             `json:"article_type"`
					Trace          string             `json:"trace"`
					Images         []*crawlHydraImage `json:"download_img_callbacks"`
				}{
					Images: []*crawlHydraImage{
						{ImageId: "img-1", Url: "https://pbs.twimg.com/media/1.jpg"},
						{ImageId: "img-screenshot", Url: "https://bucket.beijing.myqcloud.com/screenshot.png"},
						{ImageId: "img-2", Url: "https://pbs.twimg.com/media/2.jpg"},
					},
				},
			},
			want: []string{"img-1", "img-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawlHydraMsg := &crawlHydraMsg{
				RawId:     tt.fields.RawId,
				PubTime:   tt.fields.PubTime,
				Mid:       tt.fields.Mid,
				Source:    tt.fields.Source,
				CreatedAt: tt.fields.CreatedAt,
				UpdatedAt: tt.fields.UpdatedAt,
				StaticExt: tt.fields.StaticExt,
			}
			if got := crawlHydraMsg.GetImageIDs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetImageIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
