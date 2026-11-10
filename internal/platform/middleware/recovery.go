package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestID 为每个请求生成或透传请求标识，供日志与审计关联。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if id == "" {
			id = newRequestID()
		}
		c.Set("requestID", id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

// Recovery 恢复 panic，记录服务端日志并输出统一错误，不伪造成功。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("请求处理异常", "panic", recovered, "path", c.Request.URL.Path, "requestID", c.GetString("requestID"))
		if !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 500, "msg": "服务器内部错误"})
		}
		c.Abort()
	})
}
