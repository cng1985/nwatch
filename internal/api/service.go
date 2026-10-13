package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/svcctl"
	"github.com/gin-gonic/gin"
)

func (s *Server) listServices(c *gin.Context) {
	state := strings.ToLower(strings.TrimSpace(c.Query("state")))
	switch state {
	case "", "all", "active", "inactive", "failed":
	default:
		fail(c, http.StatusBadRequest, "状态筛选不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	res, err := s.services.List(ctx, c.Query("q"), state)
	if err != nil {
		slog.Warn("读取系统服务失败", "err", err.Error())
		fail(c, serviceCode(err), err.Error())
		return
	}
	ok(c, res)
}

func (s *Server) startService(c *gin.Context)   { s.serviceAction(c, "start") }
func (s *Server) stopService(c *gin.Context)    { s.serviceAction(c, "stop") }
func (s *Server) restartService(c *gin.Context) { s.serviceAction(c, "restart") }

func (s *Server) serviceAction(c *gin.Context, action string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	unit, err := s.services.Action(ctx, c.Param("name"), action)
	if err != nil {
		slog.Warn("系统服务操作失败", "user", c.GetString("username"), "action", action, "unit", c.Param("name"), "err", err.Error())
		fail(c, serviceCode(err), err.Error())
		return
	}
	slog.Info("系统服务操作", "user", c.GetString("username"), "action", action, "unit", unit)
	ok(c, gin.H{"ok": true, "name": unit, "action": action})
}

func (s *Server) serviceLogs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	logs, err := s.services.Logs(ctx, c.Param("name"), atoi(c.Query("lines"), 200))
	if err != nil {
		slog.Warn("读取系统服务日志失败", "user", c.GetString("username"), "unit", c.Param("name"), "err", err.Error())
		fail(c, serviceCode(err), err.Error())
		return
	}
	ok(c, logs)
}

func serviceCode(err error) int {
	switch {
	case errors.Is(err, svcctl.ErrDisabled):
		return http.StatusForbidden
	case errors.Is(err, svcctl.ErrUnavailable), errors.Is(err, svcctl.ErrNoJournal):
		return http.StatusServiceUnavailable
	case errors.Is(err, svcctl.ErrBadName), errors.Is(err, svcctl.ErrProtected):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
