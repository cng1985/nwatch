// Package middleware 提供 Gin 认证、安全拦截与可信上下文。
// middleware 可调用 auth 和 response，不导入 server 与具体业务模块。
package middleware

import (
	"net/http"
	"strings"

	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/domain"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

const ctxPrincipal = "principal"

// RequireAuth 校验 Bearer 令牌，写入可信 Principal 与标准 Scope。
// 认证失败输出后必须 Abort 并 return。
func RequireAuth(tokens *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" || token == header {
			response.HandleError(c, errors.Unauthorized("未登录"))
			c.Abort()
			return
		}
		claims, err := tokens.Parse(token)
		if err != nil {
			response.HandleError(c, errors.Unauthorized("登录已过期"))
			c.Abort()
			return
		}
		principal, err := tokens.Principal(claims)
		if err != nil {
			msg := "登录已过期"
			if err == auth.ErrDisabled {
				msg = err.Error()
			}
			response.HandleError(c, errors.Unauthorized(msg))
			c.Abort()
			return
		}
		c.Set("username", principal.Username)
		c.Set("userID", principal.ID)
		c.Set(ctxPrincipal, principal)
		requestID, _ := c.Get("requestID")
		scope := &domain.Scope{
			ActorID:   principal.ID,
			Username:  principal.Username,
			RequestID: toString(requestID),
		}
		c.Request = c.Request.WithContext(domain.WithScope(c.Request.Context(), scope))
		c.Next()
	}
}

// Permit 做路由级授权；数据级授权由 Resource 执行。
func Permit(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !PrincipalOf(c).Can(perm) {
			response.HandleError(c, errors.Forbidden("没有权限"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// PrincipalOf 读取当前请求的登录用户，未认证时返回 nil。
func PrincipalOf(c *gin.Context) *auth.Principal {
	value, ok := c.Get(ctxPrincipal)
	if !ok {
		return nil
	}
	principal, _ := value.(*auth.Principal)
	return principal
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

// MaxBody 限制请求体大小，覆盖真实读取过程，不只是 Content-Length。
func MaxBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
