// 学习入口负责配置和依赖组装，具体协议与 Agent 循环位于领域层。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sakelei/ai/apps/interfaces/config"
	"sakelei/ai/domain/agent/entity"
	"sakelei/ai/domain/agent/service"
	"sakelei/ai/infra/deepseek"
	"sakelei/ai/infra/openobserve"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败：", err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, args []string, out, warnings io.Writer) error {
	flags := flag.NewFlagSet("learning", flag.ContinueOnError)
	flags.SetOutput(warnings)
	lesson := flags.String("lesson", "chat", "课程：chat、messages、agent、observe、verify")
	envFile := flags.String("env-file", "", "显式配置文件路径；环境变量优先")
	question := flags.String("question", "", "替换 chat 或 agent 课程的问题")
	traceID := flags.String("trace-id", "", "verify 模式查询的 trace ID")
	expected := flags.Int("expect", 1, "verify 模式至少应查到的 span 数量")
	checkTrace := flags.Bool("verify", false, "课程完成后按实际调用数验证业务与观测入库")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errors.New("命令参数无效，请使用 -h 查看帮助")
	}
	if flags.NArg() != 0 {
		return errors.New("不支持位置参数，请使用命令选项")
	}
	switch *lesson {
	case "chat", "messages", "agent", "observe", "verify":
	default:
		return errors.New("lesson 应为 chat、messages、agent、observe 或 verify")
	}
	settings, err := config.Load(*envFile, os.LookupEnv)
	if err != nil {
		return err
	}
	if (*checkTrace || *lesson == "observe" || *lesson == "verify") && !settings.Observe.Enabled {
		return errors.New("观测检查与验收需要启用 TELEMETRY_ENABLED")
	}
	if *lesson == "verify" {
		if !settings.Observe.Enabled {
			return errors.New("verify 需要启用观测")
		}
		return verify(ctx, settings.Observe, *traceID, *expected, out)
	}
	telemetry, err := openobserve.New(ctx, settings.Observe, nil, warnings)
	if err != nil {
		return err
	}
	// 即便业务上下文已取消，仍给已有观测记录独立、有限的刷新时间。
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		telemetry.Close(flushCtx)
	}()
	if *lesson == "observe" {
		if !settings.Observe.Enabled {
			return errors.New("observe 需要启用观测")
		}
		_, span := telemetry.StartCheck(ctx)
		id := span.SpanContext().TraceID().String()
		span.End()
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := telemetry.Close(flushCtx)
		cancel()
		if err != nil || telemetry.FailedSpans() != 0 {
			return errors.New("观测上报未成功，请检查服务、端点和鉴权")
		}
		return verify(ctx, settings.Observe, id, 1, out)
	}
	client, err := deepseek.New(settings.Model, telemetry, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	ctx, run := telemetry.StartRun(ctx, *lesson)
	fmt.Fprintln(out, "trace_id：", run.TraceID)
	result, runErr := runLesson(ctx, *lesson, *question, settings.Limits, client, telemetry, out)
	run.End(runErr)
	if runErr != nil {
		// 错误保留分类和上下文；终端不输出可能包含凭据的底层文本。
		return errors.New(telemetry.CleanText(runErr.Error()))
	}
	fmt.Fprintf(out, "完成：模型请求 %d 次，工具分发 %d 次，已知累计 token %d\n", result.ModelCalls, result.ToolCalls, result.KnownTokens)
	if !result.UsageComplete {
		fmt.Fprintln(out, "部分调用缺少用量，累计值仅包含已知部分；token 预算不能证明未知用量未超限。")
	}
	if *checkTrace {
		if !settings.Observe.Enabled {
			return errors.New("课程观测验收需要启用 TELEMETRY_ENABLED")
		}
		if *question == "" {
			if err := validateLesson(*lesson, result); err != nil {
				return err
			}
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := telemetry.Close(flushCtx)
		cancel()
		if err != nil || telemetry.FailedSpans() != 0 {
			return errors.New("业务已完成，但观测上报未确认成功")
		}
		return verify(ctx, settings.Observe, run.TraceID, 1+result.ModelCalls+result.ToolCalls, out)
	}
	return nil
}

func validateLesson(lesson string, result entity.RunResult) error {
	switch lesson {
	case "messages":
		if !strings.Contains(result.Answer, "豆包") {
			return errors.New("多轮课程验收失败：回答没有保留第一轮的小猫名字")
		}
	case "agent":
		found := false
		for _, execution := range result.ToolExecutions {
			if execution.Call.Function.Name == "multiply" && execution.Result.OK && execution.Result.Result == float64(391) {
				found = true
			}
		}
		if !found || !strings.Contains(result.Answer, "391") {
			return errors.New("工具课程验收失败：需要实际计算出 391 并在最终回答中体现结果")
		}
	}
	return nil
}

func runLesson(ctx context.Context, lesson, question string, limits service.Limits, model service.ChatClient, telemetry *openobserve.Telemetry, out io.Writer) (entity.RunResult, error) {
	switch lesson {
	case "chat":
		if question == "" {
			question = "用一句话解释什么是递归。"
		}
		return chatLesson(ctx, model, out, question)
	case "messages":
		return messagesLesson(ctx, model, out)
	case "agent":
		if question == "" {
			question = "每盒有 17 支笔，买 23 盒，一共多少支？"
		}
		registry, err := service.NewRegistry([]service.Tool{service.MultiplyTool()})
		if err != nil {
			return entity.RunResult{}, err
		}
		agent, err := service.New(model, &openobserve.Tools{Registry: registry, Telemetry: telemetry})
		if err != nil {
			return entity.RunResult{}, err
		}
		result, err := agent.Run(ctx, question, "你是计算助手。涉及乘法必须使用 multiply；工具出错时不能捏造结果。用中文简短回答。", limits)
		for _, execution := range result.ToolExecutions {
			fmt.Fprintf(out, "实际工具：%s，结果：%v\n", execution.Call.Function.Name, execution.Result)
		}
		if err == nil {
			fmt.Fprintln(out, "回答：", result.Answer)
		}
		return result, err
	}
	return entity.RunResult{}, errors.New("未知课程")
}

func verify(ctx context.Context, settings openobserve.Config, traceID string, expected int, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	hits, err := openobserve.WaitTrace(ctx, settings, traceID, expected, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "观测验收成功：trace_id=%s，查回 %d 个 span\n", traceID, len(hits))
	for _, hit := range hits {
		fmt.Fprintf(out, "步骤：%v，span_id=%v，父节点=%v\n", hit["operation_name"], hit["span_id"], hit["reference_parent_span_id"])
	}
	return nil
}
