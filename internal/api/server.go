package api

import (
	"context"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/logview"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/scheduler"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/version"
	"github.com/cng1985/nwatch/internal/web"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Server struct {
	cfg       *config.Config
	db        *gorm.DB
	settings  *settings.Store
	tokens    *auth.Service
	processor *scheduler.Processor
	pool      *scheduler.Pool
	sched     *scheduler.Scheduler
	alerts    *alert.Manager
	mail      *mailer.Service
	collector *host.Collector
	logs      *logview.Store
	engine    *gin.Engine
	http      *http.Server
	started   time.Time
}

func NewServer(
	cfg *config.Config,
	db *gorm.DB,
	store *settings.Store,
	tokens *auth.Service,
	processor *scheduler.Processor,
	pool *scheduler.Pool,
	sched *scheduler.Scheduler,
	alerts *alert.Manager,
	mail *mailer.Service,
	collector *host.Collector,
	logs *logview.Store,
	lc fx.Lifecycle,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		cfg: cfg, db: db, settings: store, tokens: tokens,
		processor: processor, pool: pool, sched: sched, alerts: alerts, mail: mail,
		collector: collector, logs: logs,
		engine: gin.New(), started: time.Now(),
	}
	s.engine.Use(gin.Recovery())
	s.routes()
	s.http = &http.Server{
		Addr:              cfg.Addr(),
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					slog.Info("HTTP 服务已启动", "addr", s.http.Addr)
					if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						slog.Error("HTTP 服务异常", "err", err.Error())
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				slog.Info("停止接收 HTTP 请求")
				return s.http.Shutdown(ctx)
			},
		})
	}
	return s
}

func (s *Server) Handler() http.Handler { return s.engine }

func (s *Server) routes() {
	s.engine.GET("/health", s.health)
	s.engine.GET("/version", s.version)
	api := s.engine.Group("/api")
	api.POST("/auth/login", s.login)
	authed := api.Group("")
	authed.Use(s.requireAuth)
	authed.GET("/auth/me", s.me)
	authed.PUT("/account/password", s.changePassword)
	authed.GET("/dashboard", s.dashboard)
	authed.GET("/monitors", s.listMonitors)
	authed.POST("/monitors", s.createMonitor)
	authed.GET("/monitors/:id", s.getMonitor)
	authed.PUT("/monitors/:id", s.updateMonitor)
	authed.DELETE("/monitors/:id", s.deleteMonitor)
	authed.POST("/monitors/:id/enable", s.enableMonitor)
	authed.POST("/monitors/:id/disable", s.disableMonitor)
	authed.POST("/monitors/:id/check", s.checkMonitor)
	authed.GET("/monitors/:id/checks", s.listChecks)
	authed.GET("/monitors/:id/metrics", s.monitorMetrics)
	authed.GET("/monitors/:id/events", s.monitorEvents)
	authed.GET("/groups", s.listGroups)
	authed.POST("/groups", s.createGroup)
	authed.PUT("/groups/:id", s.updateGroup)
	authed.DELETE("/groups/:id", s.deleteGroup)
	authed.GET("/events", s.listEvents)
	authed.GET("/certificates", s.listCertificates)
	authed.GET("/notifiers", s.listNotifiers)
	authed.POST("/notifiers", s.createNotifier)
	authed.PUT("/notifiers/:id", s.updateNotifier)
	authed.DELETE("/notifiers/:id", s.deleteNotifier)
	authed.POST("/notifiers/:id/test", s.testNotifier)
	authed.GET("/notification-logs", s.listNotifyLogs)
	authed.GET("/settings", s.getSettings)
	authed.PUT("/settings", s.updateSettings)
	authed.PUT("/settings/mail", s.updateMail)
	authed.POST("/settings/mail/test", s.testMail)
	authed.GET("/backup", s.downloadBackup)
	authed.POST("/backup", s.createBackup)
	authed.GET("/export", s.exportConfig)
	authed.POST("/import", s.importConfig)
	authed.GET("/host", s.hostStatus)
	authed.GET("/scripts/runs", s.listScriptRuns)
	authed.GET("/scripts/runs/:id", s.getScriptRun)
	authed.POST("/scripts/run", s.runScript)
	authed.GET("/logs", s.listLogs)
	s.engine.NoRoute(s.frontend)
}

func (s *Server) requireAuth(c *gin.Context) {
	header := c.GetHeader("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || token == header {
		fail(c, http.StatusUnauthorized, "未登录")
		c.Abort()
		return
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		fail(c, http.StatusUnauthorized, "登录已过期")
		c.Abort()
		return
	}
	c.Set("username", claims.Username)
	c.Next()
}

func (s *Server) frontend(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		fail(c, http.StatusNotFound, "接口不存在")
		return
	}
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		c.String(http.StatusInternalServerError, "前端资源缺失")
		return
	}
	rel := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	if _, err := fs.Stat(sub, rel); err != nil {
		rel = "index.html"
	}
	data, err := fs.ReadFile(sub, rel)
	if err != nil {
		c.String(http.StatusNotFound, "页面不存在")
		return
	}
	ctype := mime.TypeByExtension(path.Ext(rel))
	switch path.Ext(rel) {
	case ".js":
		ctype = "text/javascript; charset=utf-8"
	case ".css":
		ctype = "text/css; charset=utf-8"
	case ".html":
		ctype = "text/html; charset=utf-8"
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	if rel == "index.html" {
		c.Header("Cache-Control", "no-cache")
	} else {
		c.Header("Cache-Control", "public, max-age=86400")
	}
	c.Data(http.StatusOK, ctype, data)
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

func pageParams(c *gin.Context) (int, int) {
	page := atoi(c.Query("page"), 1)
	size := atoi(c.Query("pageSize"), 20)
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		fail(c, http.StatusBadRequest, "无效的编号")
		return 0, false
	}
	return uint(n), true
}

func (s *Server) version(c *gin.Context) {
	ok(c, gin.H{
		"version": version.Version,
		"uptime":  time.Since(s.started).Truncate(time.Second).String(),
	})
}
