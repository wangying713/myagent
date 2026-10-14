package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"sakelei/ai/domain/agent/entity"
)

type lessonModelFunc func(context.Context, entity.ChatRequest) (entity.ChatResponse, error)

func (f lessonModelFunc) Chat(ctx context.Context, request entity.ChatRequest) (entity.ChatResponse, error) {
	return f(ctx, request)
}

type lessonModel struct {
	requests []entity.ChatRequest
}

func (m *lessonModel) Chat(ctx context.Context, request entity.ChatRequest) (entity.ChatResponse, error) {
	request.Messages = append([]entity.Message(nil), request.Messages...)
	m.requests = append(m.requests, request)
	text := "记住了豆包"
	if len(m.requests) == 2 {
		text = "豆包"
	}
	return entity.ChatResponse{Message: entity.Message{Role: "assistant", Content: text}, FinishReason: "stop"}, nil
}

func TestMessagesLessonSendsActualHistory(t *testing.T) {
	model := &lessonModel{}
	result, err := messagesLesson(context.Background(), model, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if result.ModelCalls != 2 || len(model.requests) != 2 {
		t.Fatalf("模型请求实际 %d，预期 2", result.ModelCalls)
	}
	second := model.requests[1].Messages
	if len(second) != 4 || second[2].Role != "assistant" || second[2].Content != "记住了豆包" || second[3].Role != "user" {
		t.Errorf("第二轮历史实际 %+v，预期携带第一轮回答", second)
	}
}

func TestCLIHelpAndUnknownLesson(t *testing.T) {
	if err := execute(context.Background(), []string{"-h"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := execute(context.Background(), []string{"-lesson", "unknown"}, io.Discard, io.Discard); err == nil {
		t.Error("未知课程应在加载密钥和创建连接前被拒绝")
	}
}

func TestLessonsKeepUsageOnFailure(t *testing.T) {
	tokens, firstTokens, negative := 37, 10, -1
	for _, lesson := range []string{"chat", "messages"} {
		for _, tc := range []struct {
			name     string
			usage    entity.Usage
			callErr  error
			want     int
			complete bool
		}{
			{"调用失败", entity.Usage{TotalTokens: &tokens}, context.DeadlineExceeded, 37, true},
			{"校验失败", entity.Usage{TotalTokens: &tokens}, nil, 37, true},
			{"用量未知", entity.Usage{}, context.DeadlineExceeded, 0, false},
			{"用量非法", entity.Usage{TotalTokens: &negative}, nil, 0, false},
		} {
			t.Run(lesson+"/"+tc.name, func(t *testing.T) {
				calls := 0
				model := lessonModelFunc(func(context.Context, entity.ChatRequest) (entity.ChatResponse, error) {
					calls++
					if lesson == "messages" && calls == 1 {
						return entity.ChatResponse{Message: entity.Message{Role: "assistant", Content: "记住了豆包"}, FinishReason: "stop", Usage: entity.Usage{TotalTokens: &firstTokens}}, nil
					}
					return entity.ChatResponse{Message: entity.Message{Role: "assistant", Content: "半句"}, FinishReason: "length", Usage: tc.usage}, tc.callErr
				})
				var result entity.RunResult
				var err error
				wantTokens, wantCalls := tc.want, 1
				if lesson == "chat" {
					result, err = chatLesson(context.Background(), model, io.Discard, "问题")
				} else {
					result, err = messagesLesson(context.Background(), model, io.Discard)
					wantTokens += firstTokens
					wantCalls = 2
				}
				if err == nil || (tc.callErr != nil && !errors.Is(err, tc.callErr)) {
					t.Errorf("应保留调用错误或校验错误，实际 %v", err)
				}
				if result.KnownTokens != wantTokens || result.UsageComplete != tc.complete || result.ModelCalls != wantCalls {
					t.Errorf("失败课程统计实际 %+v，预期 %d token、完整=%v、%d 次请求", result, wantTokens, tc.complete, wantCalls)
				}
			})
		}
	}
}
