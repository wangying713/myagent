package openobserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var streamPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// QueryTrace 按 trace ID 查询入库记录，不生成新的 span。
func QueryTrace(ctx context.Context, config Config, traceID string, client *http.Client) ([]map[string]any, error) {
	if !traceIDPattern.MatchString(traceID) || traceID == strings.Repeat("0", 32) {
		return nil, errors.New("trace_id 必须是有效的 32 位小写十六进制 ID")
	}
	if !strings.HasSuffix(config.Endpoint, "/v1/traces") {
		return nil, errors.New("观测查询需要以 /v1/traces 结尾的端点")
	}
	stream := "default"
	for key, value := range config.Headers {
		if strings.EqualFold(key, "stream-name") {
			stream = value
		}
	}
	if !streamPattern.MatchString(stream) {
		return nil, errors.New("stream 名只能包含字母、数字、下划线和短横线")
	}
	now := time.Now().UnixMicro()
	payload := map[string]any{"query": map[string]any{
		"sql":        fmt.Sprintf(`SELECT * FROM "%s" WHERE trace_id = '%s' ORDER BY start_time`, stream, traceID),
		"start_time": now - int64((24*time.Hour)/time.Microsecond), "end_time": now + 1_000_000, "from": 0, "size": 100,
	}}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSuffix(config.Endpoint, "/v1/traces") + "/_search?type=traces"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("无法创建观测查询请求")
	}
	for key, value := range config.Headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Content-Type", "application/json")
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client = &http.Client{Transport: transport, Timeout: 5 * time.Second}
		defer client.CloseIdleConnections()
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := copyClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("查询 OpenObserve: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenObserve 查询失败：HTTP %d", response.StatusCode)
	}
	const limit = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(raw) > limit {
		return nil, errors.New("OpenObserve 查询响应读取失败或超过 8 MiB")
	}
	var result struct {
		Hits  []map[string]any `json:"hits"`
		Error json.RawMessage  `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("OpenObserve 查询响应不是有效 JSON")
	}
	if len(result.Error) != 0 && string(result.Error) != "null" && string(result.Error) != `""` && string(result.Error) != "false" {
		return nil, errors.New("OpenObserve 查询返回服务端错误")
	}
	return result.Hits, nil
}

// WaitTrace 有界轮询，只有实际查回足够的记录才成功；鉴权或协议错误立即返回。
func WaitTrace(ctx context.Context, config Config, traceID string, expected int, client *http.Client) ([]map[string]any, error) {
	if expected <= 0 || expected > 100 {
		return nil, errors.New("预期 span 数量必须在 1 到 100 之间")
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	var hits []map[string]any
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("未查到足够记录，实际 %d，预期至少 %d: %w", len(hits), expected, ctx.Err())
		case <-timer.C:
		}
		var err error
		hits, err = QueryTrace(ctx, config, traceID, client)
		if err != nil {
			return nil, err
		}
		if len(hits) >= expected {
			if ValidateTrace(hits, traceID) == nil {
				return hits, nil
			}
		}
		timer.Reset(time.Second)
	}
}

// ValidateTrace 验证实际返回的 ID 和父子关系，不能只凭 HTTP 200 或记录数成功。
func ValidateTrace(hits []map[string]any, traceID string) error {
	ids := make(map[string]bool)
	root := ""
	for _, hit := range hits {
		id, _ := hit["span_id"].(string)
		idTrace, _ := hit["trace_id"].(string)
		if idTrace != traceID || len(id) != 16 || ids[id] {
			return errors.New("观测记录的 trace ID 或 span ID 无效")
		}
		ids[id] = true
		parent, _ := hit["reference_parent_span_id"].(string)
		if parent == "" || parent == strings.Repeat("0", 16) {
			if root != "" {
				return errors.New("同一个 trace 中存在多个根节点")
			}
			root = id
		}
	}
	if root == "" {
		return errors.New("根 span 尚未入库")
	}
	for _, hit := range hits {
		id, _ := hit["span_id"].(string)
		parent, _ := hit["reference_parent_span_id"].(string)
		if id != root && parent != root {
			return errors.New("模型或工具 span 没有归属于运行根节点")
		}
	}
	return nil
}
