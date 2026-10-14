package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/dockerctl"
	"github.com/gin-gonic/gin"
)

func (s *Server) listDockerContainers(c *gin.Context) {
	state := strings.ToLower(strings.TrimSpace(c.Query("state")))
	switch state {
	case "", "all", "running", "exited", "paused", "created", "restarting", "dead", "removing":
	default:
		fail(c, http.StatusBadRequest, "状态筛选不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	res, err := s.docker.Containers(ctx, c.Query("q"), state)
	if err != nil {
		slog.Warn("读取容器失败", "err", err.Error())
		fail(c, dockerCode(err), err.Error())
		return
	}
	ok(c, res)
}

func (s *Server) listDockerImages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	res, err := s.docker.Images(ctx, c.Query("q"))
	if err != nil {
		slog.Warn("读取镜像失败", "err", err.Error())
		fail(c, dockerCode(err), err.Error())
		return
	}
	ok(c, res)
}

func (s *Server) startDockerContainer(c *gin.Context)   { s.dockerAction(c, "start") }
func (s *Server) stopDockerContainer(c *gin.Context)    { s.dockerAction(c, "stop") }
func (s *Server) restartDockerContainer(c *gin.Context) { s.dockerAction(c, "restart") }

func (s *Server) dockerAction(c *gin.Context, action string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	name, err := s.docker.Action(ctx, c.Param("name"), action)
	if err != nil {
		slog.Warn("容器操作失败", "user", c.GetString("username"), "action", action, "name", c.Param("name"), "err", err.Error())
		fail(c, dockerCode(err), err.Error())
		return
	}
	slog.Info("容器操作", "user", c.GetString("username"), "action", action, "name", name)
	ok(c, gin.H{"ok": true, "name": name, "action": action})
}

func (s *Server) dockerContainerLogs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	logs, err := s.docker.Logs(ctx, c.Param("name"), atoi(c.Query("lines"), 200))
	if err != nil {
		slog.Warn("读取容器日志失败", "user", c.GetString("username"), "name", c.Param("name"), "err", err.Error())
		fail(c, dockerCode(err), err.Error())
		return
	}
	ok(c, logs)
}

func dockerCode(err error) int {
	switch {
	case errors.Is(err, dockerctl.ErrDisabled):
		return http.StatusForbidden
	case errors.Is(err, dockerctl.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, dockerctl.ErrBadName):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
