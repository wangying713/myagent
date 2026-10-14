package openobserve

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOTLPPartialRejectionIsNotSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"partialSuccess":{"rejectedSpans":"1","errorMessage":"Basic fake"}}`)
	}))
	defer server.Close()
	var warnings bytes.Buffer
	observer, err := New(context.Background(), Config{Enabled: true, ServiceName: "测试", MaxChars: 256, Endpoint: server.URL + "/v1/traces", Headers: map[string]string{"Authorization": "Basic fake"}}, nil, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	_, span := observer.StartCheck(context.Background())
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := observer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if observer.FailedSpans() != 1 {
		t.Errorf("服务端明确拒收，失败 span 实际 %d，预期 1", observer.FailedSpans())
	}
	if strings.Contains(warnings.String(), "Basic fake") {
		t.Error("拒收说明泄漏了鉴权值")
	}
}
