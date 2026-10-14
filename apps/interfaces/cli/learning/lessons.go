package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"sakelei/ai/domain/agent/entity"
	"sakelei/ai/domain/agent/service"
)

// chatLesson 展示一次请求的消息、回答、结束原因和服务端用量。
func chatLesson(ctx context.Context, model service.ChatClient, out io.Writer, question string) (entity.RunResult, error) {
	messages := []entity.Message{{Role: "system", Content: "你是一个简洁的中文助手。"}, {Role: "user", Content: question}}
	encoded, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return entity.RunResult{}, err
	}
	fmt.Fprintln(out, "消息：", string(encoded))
	result := entity.RunResult{ModelCalls: 1, UsageComplete: true}
	response, err := model.Chat(ctx, entity.ChatRequest{Messages: messages})
	result.AddUsage(response.Usage)
	if err != nil {
		return result, err
	}
	if err := response.Validate(); err != nil {
		return result, err
	}
	fmt.Fprintln(out, "回答：", response.Message.Content)
	fmt.Fprintln(out, "结束原因：", response.FinishReason)
	printUsage(out, response.Usage)
	result.Answer = response.Message.Content
	return result, nil
}

// messagesLesson 显式保存第一轮消息，再将完整历史发给模型。
func messagesLesson(ctx context.Context, model service.ChatClient, out io.Writer) (entity.RunResult, error) {
	result := entity.RunResult{UsageComplete: true}
	messages := []entity.Message{{Role: "system", Content: "用简短中文回答。"}}
	for _, question := range []string{"我给一只虚构小猫取名叫豆包，请记住。", "我给小猫取了什么名字？"} {
		messages = append(messages, entity.Message{Role: "user", Content: question})
		result.ModelCalls++
		fmt.Fprintf(out, "第 %d 轮发送 %d 条消息\n", result.ModelCalls, len(messages))
		response, err := model.Chat(ctx, entity.ChatRequest{Messages: messages})
		result.AddUsage(response.Usage)
		if err != nil {
			return result, err
		}
		if err := response.Validate(); err != nil {
			return result, err
		}
		messages = append(messages, response.Message)
		result.Answer = response.Message.Content
		fmt.Fprintln(out, "用户：", question)
		fmt.Fprintln(out, "回答：", result.Answer)
		printUsage(out, response.Usage)
	}
	return result, nil
}

func printUsage(out io.Writer, usage entity.Usage) {
	if tokens, known := usage.Total(); known {
		fmt.Fprintln(out, "本轮 token：", tokens)
	} else {
		fmt.Fprintln(out, "本轮 token：服务端未提供完整用量")
	}
}
