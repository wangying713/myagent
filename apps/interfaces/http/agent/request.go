package agentapi

import "saklay/ai/domain/agent"

func questionInput(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	// 省略使用业务默认问题，显式空字符串仍应拒绝。
	if *value == "" {
		return "", agent.ErrInput
	}
	return *value, nil
}
