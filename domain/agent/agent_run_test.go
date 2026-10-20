package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPrepareRunRejectsInvalidQuestions(t *testing.T) {
	for _, tc := range []struct {
		name      string   // 问题列表不符合要求的情况。
		questions []string // 未经调用方校验的原始问题。
	}{
		{
			name: "未提供问题",
		},
		{
			name:      "空问题列表",
			questions: []string{},
		},
		{
			name:      "空字符串",
			questions: []string{""},
		},
		{
			name:      "只有空白",
			questions: []string{" \t\n"},
		},
		{
			name:      "超过字符上限",
			questions: []string{strings.Repeat("字", MaxQuestionChars+1)},
		},
		{
			name: "第二轮问题无效",
			questions: []string{
				"第一轮有效",
				" ",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatCli := modelFunc(func(context.Context, Request) (Response, error) {
				t.Fatal("问题无效时不应调用模型")
				return Response{}, nil
			})
			agentCli := newTestClient(t, chatCli, DefaultLimits())
			result := NewChatRsp(OperationChat)
			run, err := agentCli.prepareRun(&result, RunInput{
				Questions: tc.questions,
			})
			if !errors.Is(err, ErrInput) || run != nil {
				t.Fatalf("实际运行 %v、错误 %v，预期没有运行且返回 ErrInput", run, err)
			}
			if result.ModelCalls != 0 {
				t.Fatalf("无效问题启动了运行记录：%+v", result)
			}
		})
	}
}
