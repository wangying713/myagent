package task

import (
	"context"
	"reflect"
	"testing"

	"git.code.oa.com/sport/content-pool/content_backend/common/content/model"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	mockRepo "git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo/mock"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/base/service"
	classicsEntity "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/entity"
	classicsService "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service"
	serviceMock "git.code.oa.com/sport/content-pool/content_backend/domain/content/classics/service/mock"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/database/mysql"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rainbow"
	"git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/mario"
	mockMario "git.code.oa.com/sport/content-pool/content_backend/infrastructure/rpc/mario/mock"
	contentInsert "git.code.oa.com/sport/content-pool/content_backend/service/content_insert"
	areaModel "git.code.oa.com/sport/feed/area/public/model"
	"git.code.oa.com/trpc-go/trpc-go"
	"github.com/golang/mock/gomock"
)

func TestUnit_collection_collectionDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	itemsCli := serviceMock.NewMockCollectionItems(ctrl)
	itemsCli.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*classicsEntity.CollectionItems{
		{
			ContentID: "1",
		},
	}, nil).AnyTimes()
	repoCli := mockRepo.NewMockContent(ctrl)
	repoCli.EXPECT().BatchGet(gomock.Any(), gomock.Any()).Return(map[string]*model.Content{
		"1": {
			Video: &model.Video{
				Collection: &model.Collection{
					MainID: 1,
					SubIDs: []uint32{1, 2, 3},
				},
			},
		},
	}, nil).AnyTimes()
	repoCli.EXPECT().Modify(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	type fields struct {
		collectionItemsCli classicsService.CollectionItems
		contentRepoCli     repo.Content
	}
	type args struct {
		ctx   context.Context
		param *classicsEntity.CollectionTask
		rtx   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			fields: fields{
				contentRepoCli:     repoCli,
				collectionItemsCli: itemsCli,
			},
			args: args{
				ctx: context.Background(),
				param: &classicsEntity.CollectionTask{
					Type:          classicsEntity.CollectionOpTypeDelete,
					CollectionIDs: []uint32{1},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &collection{
				contentRepoCli:     tt.fields.contentRepoCli,
				collectionItemsCli: tt.fields.collectionItemsCli,
			}
			if err := c.collectionDelete(tt.args.ctx, tt.args.param, tt.args.rtx); (err != nil) != tt.wantErr {
				t.Errorf("collectionDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnit_collection_mergeListsPriorityNew(t *testing.T) {
	type fields struct {
	}
	type args struct {
		newList []*areaModel.CollectionItem
		oldList []*areaModel.CollectionItem
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*areaModel.CollectionItem
	}{
		{
			name:   "normal",
			fields: fields{},
			args: args{
				newList: []*areaModel.CollectionItem{
					{
						ID:     201,
						Filter: areaModel.Filter{},
					},
					{
						ID:     101,
						Filter: areaModel.Filter{},
					},
					{
						ID:     202,
						Filter: areaModel.Filter{},
					},
				},
				oldList: []*areaModel.CollectionItem{
					{
						ID:     101,
						Filter: areaModel.Filter{},
					},
					{
						ID:     103,
						Filter: areaModel.Filter{},
					},
				},
			},
			want: []*areaModel.CollectionItem{
				{
					ID:     101,
					Filter: areaModel.Filter{},
				},
				{
					ID:     201,
					Filter: areaModel.Filter{},
				},
				{
					ID:     202,
					Filter: areaModel.Filter{},
				},
			},
		},
		{
			name:   "normal_eq",
			fields: fields{},
			args: args{
				newList: []*areaModel.CollectionItem{
					{
						ID:     52,
						Filter: areaModel.Filter{},
					},
					{
						ID:     47,
						Filter: areaModel.Filter{},
					},
				},
				oldList: []*areaModel.CollectionItem{
					{
						ID:     47,
						Filter: areaModel.Filter{},
					},
					{
						ID:     52,
						Filter: areaModel.Filter{},
					},
				},
			},
			want: []*areaModel.CollectionItem{
				{
					ID:     47,
					Filter: areaModel.Filter{},
				},
				{
					ID:     52,
					Filter: areaModel.Filter{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &collection{}
			if got := c.mergeList(tt.args.newList, tt.args.oldList); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeListsPriorityNew() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnit_collection_publishCollection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mockMario.NewMockMario(ctrl)
	cli.EXPECT().PublishCollection(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	type fields struct {
		rainbow            rainbow.Client
		marioCli           mario.Mario
		contentRepoCli     repo.Content
		indexCli           service.Index
		collectionCli      classicsService.Collection
		collectionItemsCli classicsService.CollectionItems
		insertCli          contentInsert.Client
		feedCli            mysql.OperationFeedClient
	}
	type args struct {
		ctx   context.Context
		param *classicsEntity.CollectionTask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			fields: fields{
				rainbow:            nil,
				marioCli:           cli,
				contentRepoCli:     nil,
				indexCli:           nil,
				collectionCli:      nil,
				collectionItemsCli: nil,
				insertCli:          nil,
				feedCli:            nil,
			},
			args: args{
				ctx: trpc.BackgroundContext(),
				param: &classicsEntity.CollectionTask{
					CollectionIDs: []uint32{1, 2, 3},
					Param:         "{}",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &collection{
				rainbow:            tt.fields.rainbow,
				marioCli:           tt.fields.marioCli,
				contentRepoCli:     tt.fields.contentRepoCli,
				indexCli:           tt.fields.indexCli,
				collectionCli:      tt.fields.collectionCli,
				collectionItemsCli: tt.fields.collectionItemsCli,
				insertCli:          tt.fields.insertCli,
				feedCli:            tt.fields.feedCli,
			}
			if err := c.publishCollection(tt.args.ctx, tt.args.param); (err != nil) != tt.wantErr {
				t.Errorf("publishCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
