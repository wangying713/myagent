package agent

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrInput 表示问题、消息数量、工具策略或其他业务输入不符合要求。
var ErrInput = errors.New("Agent 输入无效")

const (
	OperationChat     = "chat"     // 聊天操作，默认用于普通问答。
	OperationMessages = "messages" // 固定两轮的消息历史调用。
	OperationRun      = "agent"    // 工具循环，保留既有协议值 agent。
)

// MaxQuestionChars 是单个问题允许的最大 Unicode 字符数。
const MaxQuestionChars = 4096

// Client 的依赖在构造后保持不变，每次调用单独保存历史和执行记录，可并发使用。
type Client struct {
	chatCli  ChatClient // 支持取消和并发调用的模型客户端。
	observer Observer   // 支持并发使用的观测与脱敏接口。
	limits   Limits     // 构造时校验并固定的单次运行预算。
}

// NewClient 校验非 nil 依赖及正数预算，创建可并发调用的 Agent 客户端。
// chatCli 和 observer 由调用方注入并管理生命周期，须支持并发调用。
func NewClient(chatCli ChatClient, observer Observer, limits Limits) (*Client, error) {
	if chatCli == nil || observer == nil || limits.ModelCalls <= 0 || limits.ToolCalls <= 0 || limits.Tokens <= 0 {
		return nil, errors.New("Agent 依赖或运行预算无效")
	}
	return &Client{
		chatCli:  chatCli,
		observer: observer,
		limits:   limits,
	}, nil
}

// validateQuestion 拒绝纯空白或超过 MaxQuestionChars 个 Unicode 码点的问题。
// 校验通过返回 nil，否则返回 ErrInput；原问题内容保持不变。
func validateQuestion(question string) error {
	// TrimSpace 只用于判断是否为空；按码点计数，使中文等多字节字符按一个字符计算。
	if strings.TrimSpace(question) == "" || utf8.RuneCountInString(question) > MaxQuestionChars {
		return ErrInput
	}
	return nil
}
