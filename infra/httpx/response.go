package httpx

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// Write 先完成编码再写响应头，失败时返回错误，交由调用方生成其统一错误响应。
// 编码失败不会写出任何响应头或正文，也不重试业务请求。
func Write(c *gin.Context, status int, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.Header("Cache-Control", "no-store")
	c.Data(status, "application/json; charset=utf-8", body)
	return nil
}
