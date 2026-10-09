package api

import (
	"net/http"
	"strings"

	"github.com/cng1985/nwatch/internal/auth"
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
	token, err := s.tokens.Login(c.ClientIP(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		code := http.StatusUnauthorized
		if err == auth.ErrRateLimited {
			code = http.StatusTooManyRequests
		}
		fail(c, code, err.Error())
		return
	}
	ok(c, gin.H{
		"token":                token,
		"username":             s.cfg.Security.Username,
		"usingDefaultPassword": s.settings.UsingDefaultPassword(),
	})
}

func (s *Server) me(c *gin.Context) {
	ok(c, gin.H{
		"username":             c.GetString("username"),
		"version":              version.Version,
		"usingDefaultPassword": s.settings.UsingDefaultPassword(),
	})
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
	if !s.settings.CheckPassword(req.OldPassword) {
		fail(c, http.StatusBadRequest, "原密码不正确")
		return
	}
	if len(req.NewPassword) < 6 {
		fail(c, http.StatusBadRequest, "新密码至少 6 位")
		return
	}
	if err := s.settings.SetPassword(req.NewPassword); err != nil {
		fail(c, http.StatusInternalServerError, "保存密码失败")
		return
	}
	ok(c, gin.H{"ok": true})
}
