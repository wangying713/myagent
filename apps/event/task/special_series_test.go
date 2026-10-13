package task

import (
	"context"
	"testing"

	"git.code.oa.com/sport/content-pool/content_backend/domain/series/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/series/repo"
	mockRepo "git.code.oa.com/sport/content-pool/content_backend/domain/series/repo/mock"
	"git.code.oa.com/sport/content-pool/content_backend/domain/series/service/mock"
	commentMock "git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/comment/mock"
	indexBuilderMock "git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/indexBuilder/mock"
	"git.code.oa.com/trpc-go/trpc-go"
	hermesCommentPB "git.code.oa.com/trpcprotocol/hermes/comment_hermes_comment"
	"git.woa.com/sport/common/content"
	"github.com/golang/mock/gomock"
)

func TestUnit_specialSeries_getReplyNum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	commentCli := commentMock.NewMockClient(ctrl)
	commentCli.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int32(10), nil).AnyTimes()
	type args struct {
		ctx   context.Context
		sid   string
		infos map[string]*content.Info
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				ctx: trpc.BackgroundContext(),
				sid: "S00025",
				infos: map[string]*content.Info{
					"1": content.NewInfo(map[string]string{
						"pageView": "10",
						"replyNum": "10",
					}),
					"2": content.NewInfo(map[string]string{
						"platForm": content.PlatformContentCreator,
						"pageView": "30",
						"replyNum": "30",
					}),
				},
			},
			want:    50,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &specialSeries{
				commentCli: commentCli,
			}
			got, err := s.getReplyNum(tt.args.ctx, tt.args.sid, tt.args.infos)
			if (err != nil) != tt.wantErr {
				t.Errorf("getReplyNum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getReplyNum() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnit_specialSeries_getReadNum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	seriesHisRepo := mockRepo.NewMockSpecialSeriesHistory(ctrl)
	seriesHisRepo.EXPECT().GetReadNumber(gomock.Any(), gomock.Any()).Return([]*repo.HistoryReadNumber{
		{
			ContentID:  "1_1",
			ReadNumber: 1000,
		},
		{
			ContentID:  "1_3",
			ReadNumber: 10000,
		},
	}, nil).AnyTimes()
	seriesHisRepo.EXPECT().SaveReadNumber(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	type args struct {
		ctx   context.Context
		sid   string
		infos map[string]*content.Info
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				ctx: trpc.BackgroundContext(),
				sid: "S00025",
				infos: map[string]*content.Info{
					"1_1": content.NewInfo(map[string]string{
						"src":      "1",
						"id":       "1",
						"pageView": "10",
						"replyNum": "10",
					}),
					"1_2": content.NewInfo(map[string]string{
						"src":      "1",
						"id":       "2",
						"platForm": content.PlatformContentCreator,
						"pageView": "30",
						"replyNum": "30",
					}),
				},
			},
			want:    10163,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &specialSeries{
				seriesHisRepo: seriesHisRepo,
			}
			got, err := s.getReadNum(tt.args.ctx, tt.args.sid, tt.args.infos)
			if (err != nil) != tt.wantErr {
				t.Errorf("getReadNum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getReadNum() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnit_specialSeries_updateComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	commentCli := commentMock.NewMockClient(ctrl)
	commentCli.EXPECT().List(gomock.Any(), gomock.Any()).Return(&hermesCommentPB.HListReply{
		Cards: []*hermesCommentPB.HCard{
			{
				Comment: &hermesCommentPB.HComment{
					Id:         "123456",
					ExtendInfo: nil,
				},
			},
		},
	}, nil).AnyTimes()

	indexBuilder := indexBuilderMock.NewMockClient(ctrl)
	indexBuilder.EXPECT().IndexUpdateAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	type args struct {
		ctx        context.Context
		sid        string
		contentIDs []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				ctx:        trpc.BackgroundContext(),
				sid:        "S00054",
				contentIDs: []string{"1_1"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &specialSeries{
				commentCli:   commentCli,
				indexBuilder: indexBuilder,
			}
			if err := s.updateComment(tt.args.ctx, tt.args.sid, tt.args.contentIDs); (err != nil) != tt.wantErr {
				t.Errorf("updateComment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnit_specialSeries_updateCoverPic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	seriesSer := mock.NewMockSpecialSeries(ctrl)
	seriesSer.EXPECT().Save(gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)

	type args struct {
		ctx             context.Context
		info            *entity.SpecialSeries
		specialContents []*entity.Content
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				ctx: trpc.BackgroundContext(),
				info: &entity.SpecialSeries{
					LockField: []entity.LockFieldType{entity.LockFieldCoverPicture},
				},
				specialContents: []*entity.Content{
					{
						ContentID: "3_1834719122481479845",
						Pic:       "image",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "normal",
			args: args{
				ctx:             trpc.BackgroundContext(),
				info:            &entity.SpecialSeries{},
				specialContents: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &specialSeries{
				seriesSer: seriesSer,
			}
			if err := s.updateCoverPic(tt.args.ctx, tt.args.info, tt.args.specialContents); (err != nil) != tt.wantErr {
				t.Errorf("updateCoverPic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
