package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/cng1985/nwatch/internal/version"
	"github.com/gin-gonic/gin"
)

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	token, principal, err := s.tokens.Login(c.ClientIP(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		code := http.StatusUnauthorized
		if err == auth.ErrRateLimited {
			code = http.StatusTooManyRequests
		}
		fail(c, code, err.Error())
		return
	}
	ok(c, s.sessionBody(token, principal))
}

func (s *Server) me(c *gin.Context) {
	body := s.sessionBody("", principalOf(c))
	body["version"] = version.Version
	ok(c, body)
}

func (s *Server) sessionBody(token string, principal *rbac.Principal) gin.H {
	body := gin.H{
		"username":             principal.Username,
		"displayName":          principal.DisplayName,
		"roles":                principal.Roles,
		"permissions":          principal.PermissionList(),
		"usingDefaultPassword": s.usingDefaultPassword(principal.Username),
	}
	if token != "" {
		body["token"] = token
	}
	return body
}

func (s *Server) usingDefaultPassword(username string) bool {
	return username == rbac.ConfigUsername(s.cfg) && s.settings.UsingDefaultPassword()
}

func (s *Server) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	userID := c.GetUint("userID")
	if !s.tokens.Users().CheckPassword(userID, req.OldPassword) {
		fail(c, http.StatusBadRequest, "原密码不正确")
		return
	}
	if err := s.tokens.Users().SetPassword(userID, req.NewPassword); err != nil {
		if err == rbac.ErrBadPassword {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		fail(c, http.StatusInternalServerError, "保存密码失败")
		return
	}
	s.syncBootstrapPassword(c.GetString("username"), req.NewPassword)
	ok(c, gin.H{"ok": true})
}

func (s *Server) syncBootstrapPassword(username, plain string) {
	if username != rbac.ConfigUsername(s.cfg) {
		return
	}
	if err := s.settings.SetPassword(plain); err != nil {
		slog.Warn("同步管理员密码失败", "err", err.Error())
	}
}
