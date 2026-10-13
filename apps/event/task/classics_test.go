package task

import (
	"context"
	"fmt"
	"testing"

	"git.code.oa.com/sport/content-pool/content_backend/application/interfaces"
	"git.code.oa.com/sport/content-pool/content_backend/domain/content/base/repo"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/entity"
	"git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service"
	taskMock "git.code.oa.com/sport/content-pool/content_backend/domain/taskq/service/mock"
	"git.code.oa.com/trpc-go/trpc-go/errs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestUnit_classics_formatMsg(t *testing.T) {
	type fields struct {
		taskCli        service.Client
		contentRepoCli repo.Content
	}
	type args struct {
		ctx context.Context
		msg *infoMsg
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantTitle   string
		wantContent string
	}{
		{
			name: "normal",
			args: args{
				ctx: context.Background(),
				msg: &infoMsg{
					total:      1,
					successIDs: []string{"1"},
					errIDs:     []string{"2"},
					skipIDs:    []string{"3"},
					task:       &entity.Info{},
				},
			},
			wantTitle: "经典内容候选池导入任务已完成，成功导入内容1条，跳过已在经典内容池中的内容1条",
			wantContent: `
<html>
	<body>
		<h3>经典内容候选池导入任务已完成</h3>
		<p><b>任务概要:</b></p>
	<ul>
		<li>筛选到数据: 1条</li>
		<li>成功导入: 1条</li>
		<li>跳过已在池中: 1条</li>
		<li>导入失败: 1条</li>
	</ul>
	
	<p><b>详细ID列表:</b></p>
	<ol>
		<li>成功导入的内容ID:<br/>1</li>
		<li>跳过已在池中的内容ID:<br/>3</li>
		<li>导入失败的内容ID:<br/>2</li>
		</ol>
	</body>
</html>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &classics{
				taskCli:        tt.fields.taskCli,
				contentRepoCli: tt.fields.contentRepoCli,
			}
			gotTitle, gotContent := c.formatMsg(tt.args.ctx, tt.args.msg)
			if gotTitle != tt.wantTitle {
				t.Errorf("formatMsg() gotTitle = %v, want %v", gotTitle, tt.wantTitle)
			}
			if gotContent != tt.wantContent {
				t.Errorf("formatMsg() gotContent = %v, want %v", gotContent, tt.wantContent)
			}
		})
	}
}

func TestUnit_classics_HandleImportPool(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	taskCli := taskMock.NewMockClient(ctrl)
	taskCli.EXPECT().Pop(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Info{}, errs.New(123, "no data"))
	type fields struct {
		taskCli        service.Client
		contentRepoCli repo.Content
		modifyCli      interfaces.ContentModify
	}
	type args struct {
		ctx context.Context
		in1 string
		in2 string
		in3 int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "normal",
			fields: fields{
				taskCli: taskCli,
			},
			args: args{
				ctx: context.Background(),
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &classics{
				taskCli:        tt.fields.taskCli,
				contentRepoCli: tt.fields.contentRepoCli,
				modifyCli:      tt.fields.modifyCli,
			}
			tt.wantErr(t, c.HandleImportPool(tt.args.ctx, tt.args.in1, tt.args.in2, tt.args.in3), fmt.Sprintf("HandleImportPool(%v, %v, %v, %v)", tt.args.ctx, tt.args.in1, tt.args.in2, tt.args.in3))
		})
	}
}
