// Package task 定时任务
package task

import (
	"context"

	"git.code.oa.com/pcg-csd/trpc-ext/trpc-task/tasksdk"
)

// GetList 获取任务列表
func GetList() []tasksdk.TaskSet {
	return taskList
}

// taskList 任务列表
var taskList = []tasksdk.TaskSet{
	{
		Name:       "authority_move_cate",   // 权威资讯分类移动
		HandleFunc: NewAuthority().MoveCate, // 任务处理函数
	},
	{
		Name:       "classics_import_pool",         // 合格池导入经典内容候选池
		HandleFunc: NewClassics().HandleImportPool, // 任务处理函数
	},
	{
		Name: "collection", // 合集任务
		HandleFunc: func(ctx context.Context, name, params string, seq int32) error {
			return NewCollection().Handle(ctx, name, params, seq)
		},
	},
	{
		Name: "collection_insert", // 强插合集任务
		HandleFunc: func(ctx context.Context, name, params string, seq int32) error {
			return NewCollection().InsertHandle(ctx, name, params, seq)
		},
	},
	{
		Name:       "task_flow", // 重试状态流转
		HandleFunc: NewClient().Flow,
	},
	{
		Name: "content_export", // 导出任务
		HandleFunc: func(ctx context.Context, name, params string, seq int32) error {
			return NewContentExport().Handle(ctx, name, params, seq)
		},
	},
	{
		Name: "special_series_stats", // 计算专辑 阅读数量 && 评论数量
		HandleFunc: func(ctx context.Context, name, params string, seq int32) error {
			return NewSpecialSeries().RefreshSeriesStats(ctx, name, params, seq)
		},
	},
	{
		Name: "history_authority_yuanbao", // 权威资讯历史数据推送至元宝
		HandleFunc: func(ctx context.Context, name, params string, seq int32) error {
			return NewHistoryAuthorityYuanbao().Handle(ctx, name, params, seq)
		},
	},
}
