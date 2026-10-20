package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"saklay/ai/infra/httpx"

	"github.com/gin-gonic/gin"
)

// integerCase 保存输入表示和精确整数输出，want 为空时表示拒绝输入。
type integerCase struct {
	name  string // 场景名称。
	input string // value 字段的原始 JSON。
	want  string // 成功时的十进制整数；空字符串表示预期 HTTP 400。
}

func TestDecodeJSONIntegers(t *testing.T) {
	router := newIntegerRouter(t)
	cases := append(validIntegerCases(), invalidIntegerCases()...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/integers", strings.NewReader(`{"value":`+tc.input+`}`))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if tc.want == "" {
				if out.Code != http.StatusBadRequest {
					t.Errorf("整数输入 %s 实际 HTTP %d、正文 %s，预期 HTTP 400", tc.input, out.Code, out.Body.String())
				}
				return
			}
			if out.Code != http.StatusOK || out.Body.String() != `{"value":`+tc.want+`}` {
				t.Errorf("整数输入 %s 实际 HTTP %d、正文 %s，预期精确整数 %s", tc.input, out.Code, out.Body.String(), tc.want)
			}
		})
	}
}

func newIntegerRouter(t *testing.T) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.POST("/integers", func(c *gin.Context) {
		var req struct {
			Value *int64 `json:"value"` // 可选整数，nil 表示省略，零与大整数均需精确保留。
		}
		if err := httpx.DecodeJSON(c, &req); err != nil {
			writeTestReqError(c, err, nil)
			return
		}
		if err := httpx.Write(c, http.StatusOK, req); err != nil {
			t.Error(err)
		}
	})
	return router
}

func validIntegerCases() []integerCase {
	return []integerCase{
		{
			name:  "零",
			input: `0`,
			want:  "0",
		},
		{
			name:  "数字字符串",
			input: `"2"`,
			want:  "2",
		},
		{
			name:  "整数值小数",
			input: `2.00`,
			want:  "2",
		},
		{
			name:  "指数",
			input: `2e1`,
			want:  "20",
		},
		{
			name:  "负指数消去尾零",
			input: `2000e-3`,
			want:  "2",
		},
		{
			name:  "字符串指数",
			input: `"0.002E+3"`,
			want:  "2",
		},
		{
			name:  "超过浮点安全整数",
			input: `9007199254740993`,
			want:  "9007199254740993",
		},
		{
			name:  "最大整数",
			input: `9223372036854775807`,
			want:  "9223372036854775807",
		},
		{
			name:  "最小整数",
			input: `"-9223372036854775808"`,
			want:  "-9223372036854775808",
		},
		{
			name:  "极大指数的零",
			input: `0e999999999999`,
			want:  "0",
		},
	}
}

func invalidIntegerCases() []integerCase {
	return []integerCase{
		{
			name:  "null",
			input: `null`,
		},
		{
			name:  "小数",
			input: `1.5`,
		},
		{
			name:  "负指数产生小数",
			input: `1e-2`,
		},
		{
			name:  "大整数小数不能舍入",
			input: `9007199254740993.1`,
		},
		{
			name:  "正溢出",
			input: `9223372036854775808`,
		},
		{
			name:  "负溢出",
			input: `-9223372036854775809`,
		},
		{
			name:  "极大指数",
			input: `1e999999999999`,
		},
		{
			name:  "极小指数",
			input: `1e-999999999999`,
		},
		{
			name:  "空字符串",
			input: `""`,
		},
		{
			name:  "布尔值",
			input: `true`,
		},
		{
			name:  "数组",
			input: `[]`,
		},
		{
			name:  "非法数字字符串",
			input: `"NaN"`,
		},
	}
}
