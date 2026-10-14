package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/k8sctl"
	"github.com/gin-gonic/gin"
)

func (s *Server) listKubeNamespaces(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	res, err := s.kube.Namespaces(ctx)
	if err != nil {
		slog.Warn("读取命名空间失败", "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	ok(c, res)
}

func (s *Server) listKubeResources(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	if kind == "" {
		kind = "pods"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	res, err := s.kube.List(ctx, kind, c.Query("namespace"), c.Query("q"))
	if err != nil {
		slog.Warn("读取 Kubernetes 资源失败", "kind", kind, "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	ok(c, res)
}

func (s *Server) restartKubeWorkload(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	ns, name, err := s.kube.Restart(ctx, c.Param("kind"), c.Param("namespace"), c.Param("name"))
	if err != nil {
		slog.Warn("重启工作负载失败", "user", c.GetString("username"), "kind", c.Param("kind"), "namespace", c.Param("namespace"), "name", c.Param("name"), "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	slog.Info("重启工作负载", "user", c.GetString("username"), "kind", c.Param("kind"), "namespace", ns, "name", name)
	ok(c, gin.H{"ok": true, "namespace": ns, "name": name, "action": "restart"})
}

func (s *Server) scaleKubeWorkload(c *gin.Context) {
	var body struct {
		Replicas *int `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Replicas == nil {
		fail(c, http.StatusBadRequest, "副本数不合法")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	ns, name, err := s.kube.Scale(ctx, c.Param("kind"), c.Param("namespace"), c.Param("name"), *body.Replicas)
	if err != nil {
		slog.Warn("调整副本失败", "user", c.GetString("username"), "kind", c.Param("kind"), "namespace", c.Param("namespace"), "name", c.Param("name"), "replicas", *body.Replicas, "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	slog.Info("调整副本", "user", c.GetString("username"), "kind", c.Param("kind"), "namespace", ns, "name", name, "replicas", *body.Replicas)
	ok(c, gin.H{"ok": true, "namespace": ns, "name": name, "replicas": *body.Replicas})
}

func (s *Server) deleteKubePod(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	ns, name, err := s.kube.DeletePod(ctx, c.Param("namespace"), c.Param("name"))
	if err != nil {
		slog.Warn("删除 Pod 失败", "user", c.GetString("username"), "namespace", c.Param("namespace"), "name", c.Param("name"), "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	slog.Info("删除 Pod", "user", c.GetString("username"), "namespace", ns, "name", name)
	ok(c, gin.H{"ok": true, "namespace": ns, "name": name, "action": "delete"})
}

func (s *Server) kubePodLogs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	logs, err := s.kube.Logs(ctx, c.Param("namespace"), c.Param("name"), c.Query("container"), atoi(c.Query("lines"), 200))
	if err != nil {
		slog.Warn("读取 Pod 日志失败", "user", c.GetString("username"), "namespace", c.Param("namespace"), "name", c.Param("name"), "err", err.Error())
		fail(c, kubeCode(err), err.Error())
		return
	}
	ok(c, logs)
}

func kubeCode(err error) int {
	switch {
	case errors.Is(err, k8sctl.ErrDisabled):
		return http.StatusForbidden
	case errors.Is(err, k8sctl.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, k8sctl.ErrBadName), errors.Is(err, k8sctl.ErrBadKind), errors.Is(err, k8sctl.ErrBadScale), errors.Is(err, k8sctl.ErrBadConfig):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}
