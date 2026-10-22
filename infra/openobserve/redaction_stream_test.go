package openobserve

import (
	"strings"
	"testing"
)

func TestTextRedactorAcrossChunkBoundaries(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    4096,
		Secrets: []string{
			"test-api-key",
			"中文密钥",
			"abc",
			"cdef",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"你好İ，普通流式回答。",
		"前缀 test-api-key，后缀。",
		"中文密钥与 test-api-key。",
		"Bearer token-value 以及 Basic auth-value。",
		"Bearer! ordinary Basic",
		"Bearer \t\n token-value 结束",
		"fooBearer token-value 与 xsk-1234567890。",
		"sk- sk-1234567 sk-12345678!",
		"sk-1234567890 结束",
		`{"password":"含 空格的值","api_key":"another-value"}`,
		`{"refresh-token":123456,"secret":"未闭合的 敏感值`,
		`{"password":"未闭合的 转义\`,
		`{"password":"escaped \\" quote","quantity":0}`,
		`{"secret":null,"password":0}`,
		"abcdef",
	} {
		t.Run(text, func(t *testing.T) {
			want := observeCli.Clean(text)
			chars := []rune(text)
			for split := 0; split <= len(chars); split++ {
				redactor := observeCli.NewTextRedactor()
				got := redactor.Write(string(chars[:split]))
				if !strings.HasPrefix(want, got) {
					t.Fatalf("切分 %d 时提前输出 %q，最终应为 %q", split, got, want)
				}
				got += redactor.Write(string(chars[split:])) + redactor.Flush()
				if got != want {
					t.Fatalf("切分 %d 的实际输出 %q，预期 %q", split, got, want)
				}
			}
			redactor := observeCli.NewTextRedactor()
			var output strings.Builder
			for _, char := range chars {
				output.WriteString(redactor.Write(string(char)))
				if !strings.HasPrefix(want, output.String()) {
					t.Fatalf("逐字符处理提前输出敏感值：%q", output.String())
				}
			}
			output.WriteString(redactor.Flush())
			if output.String() != want || redactor.Flush() != "" {
				t.Fatalf("实际输出 %q，预期 %q 且尾部只刷新一次", output.String(), want)
			}
		})
	}
}

func TestTextRedactorEmitsOrdinaryTextImmediately(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	redactor := observeCli.NewTextRedactor()
	if got := redactor.Write("你好，世界。"); got != "你好，世界。" {
		t.Fatalf("实际输出 %q，预期普通内容实时输出", got)
	}
}

func TestTextRedactorBoundsPendingStateForLongValues(t *testing.T) {
	observeCli, err := New(t.Context(), Config{
		ServiceName: "test",
		TracerName:  "test",
		MaxChars:    256,
		Secrets:     []string{"configured-key"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	word := strings.Repeat("a", 64<<10)
	for _, text := range []string{
		word,
		"Bearer " + word,
		"sk-" + word,
		`{"password":"` + word + `"}`,
		"Basic " + strings.Repeat(" ", 64<<10) + "token",
	} {
		redactor := observeCli.NewTextRedactor()
		var output strings.Builder
		for offset := 0; offset < len(text); offset += 64 {
			output.WriteString(redactor.Write(text[offset:min(offset+64, len(text))]))
			if len(redactor.auth.pending) >= maxAuthPrefixBytes || len(redactor.json.pending) > len(`"refresh_token"`) {
				t.Fatal("增量脱敏暂存随正文增长")
			}
			for _, stage := range redactor.literals {
				if len(stage.pending) >= len(stage.secret) {
					t.Fatal("固定密钥阶段暂存了前缀之外的正文")
				}
			}
		}
		if text == word && output.Len() != len(word) {
			t.Fatalf("普通连续文本结束前仅输出 %d 字节，预期全部实时输出", output.Len())
		}
		output.WriteString(redactor.Flush())
		if got, want := output.String(), observeCli.Clean(text); got != want {
			t.Fatalf("长文本脱敏结果不同：实际 %d 字节，预期 %d 字节", len(got), len(want))
		}
	}
}
