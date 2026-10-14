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
	"github.com/cng1985/nwatch/internal/cronjob"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/logview"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/cng1985/nwatch/internal/scheduler"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/shell"
	"github.com/cng1985/nwatch/internal/svcctl"
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
	cron      *cronjob.Service
	shellHub  *shell.Hub
	services  *svcctl.Manager
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
	jobs *cronjob.Service,
	lc fx.Lifecycle,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		cfg: cfg, db: db, settings: store, tokens: tokens,
		processor: processor, pool: pool, sched: sched, alerts: alerts, mail: mail,
		collector: collector, logs: logs, cron: jobs,
		shellHub: shell.NewHub(shell.Options{
			Enabled:     cfg.Shell.Enabled,
			IdleTimeout: cfg.Shell.IdleTimeout,
			MaxLifetime: cfg.Shell.MaxLifetime,
			MaxSessions: cfg.Shell.MaxSessions,
		}),
		services: svcctl.New(cfg.Service.Enabled),
		engine:   gin.New(), started: time.Now(),
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
				s.shellHub.Close()
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
	authed.GET("/access/permissions", s.permit(rbac.PermAccessRead), s.listPermissions)
	authed.GET("/access/roles", s.permit(rbac.PermAccessRead), s.listRoles)
	authed.POST("/access/roles", s.permit(rbac.PermAccessWrite), s.createRole)
	authed.PUT("/access/roles/:id", s.permit(rbac.PermAccessWrite), s.updateRole)
	authed.DELETE("/access/roles/:id", s.permit(rbac.PermAccessWrite), s.deleteRole)
	authed.GET("/access/users", s.permit(rbac.PermAccessRead), s.listUsers)
	authed.POST("/access/users", s.permit(rbac.PermAccessWrite), s.createUser)
	authed.PUT("/access/users/:id", s.permit(rbac.PermAccessWrite), s.updateUser)
	authed.DELETE("/access/users/:id", s.permit(rbac.PermAccessWrite), s.deleteUser)
	authed.GET("/dashboard", s.permit(rbac.PermDashboardRead), s.dashboard)
	authed.GET("/monitors", s.permit(rbac.PermMonitorRead), s.listMonitors)
	authed.POST("/monitors", s.permit(rbac.PermMonitorWrite), s.createMonitor)
	authed.GET("/monitors/:id", s.permit(rbac.PermMonitorRead), s.getMonitor)
	authed.PUT("/monitors/:id", s.permit(rbac.PermMonitorWrite), s.updateMonitor)
	authed.DELETE("/monitors/:id", s.permit(rbac.PermMonitorWrite), s.deleteMonitor)
	authed.POST("/monitors/:id/enable", s.permit(rbac.PermMonitorWrite), s.enableMonitor)
	authed.POST("/monitors/:id/disable", s.permit(rbac.PermMonitorWrite), s.disableMonitor)
	authed.POST("/monitors/:id/check", s.permit(rbac.PermMonitorWrite), s.checkMonitor)
	authed.GET("/monitors/:id/checks", s.permit(rbac.PermMonitorRead), s.listChecks)
	authed.GET("/monitors/:id/metrics", s.permit(rbac.PermMonitorRead), s.monitorMetrics)
	authed.GET("/monitors/:id/events", s.permit(rbac.PermMonitorRead), s.monitorEvents)
	authed.GET("/groups", s.permit(rbac.PermGroupRead), s.listGroups)
	authed.POST("/groups", s.permit(rbac.PermGroupWrite), s.createGroup)
	authed.PUT("/groups/:id", s.permit(rbac.PermGroupWrite), s.updateGroup)
	authed.DELETE("/groups/:id", s.permit(rbac.PermGroupWrite), s.deleteGroup)
	authed.GET("/events", s.permit(rbac.PermEventRead), s.listEvents)
	authed.GET("/certificates", s.permit(rbac.PermCertRead), s.listCertificates)
	authed.GET("/notifiers", s.permit(rbac.PermNotifierRead), s.listNotifiers)
	authed.POST("/notifiers", s.permit(rbac.PermNotifierWrite), s.createNotifier)
	authed.PUT("/notifiers/:id", s.permit(rbac.PermNotifierWrite), s.updateNotifier)
	authed.DELETE("/notifiers/:id", s.permit(rbac.PermNotifierWrite), s.deleteNotifier)
	authed.POST("/notifiers/:id/test", s.permit(rbac.PermNotifierWrite), s.testNotifier)
	authed.GET("/notification-logs", s.permit(rbac.PermNotifierRead), s.listNotifyLogs)
	authed.GET("/settings", s.permit(rbac.PermSettingsRead), s.getSettings)
	authed.PUT("/settings", s.permit(rbac.PermSettingsWrite), s.updateSettings)
	authed.PUT("/settings/mail", s.permit(rbac.PermSettingsWrite), s.updateMail)
	authed.POST("/settings/mail/test", s.permit(rbac.PermSettingsWrite), s.testMail)
	authed.GET("/backup", s.permit(rbac.PermSettingsWrite), s.downloadBackup)
	authed.POST("/backup", s.permit(rbac.PermSettingsWrite), s.createBackup)
	authed.GET("/export", s.permit(rbac.PermSettingsWrite), s.exportConfig)
	authed.POST("/import", s.permit(rbac.PermSettingsWrite), s.importConfig)
	authed.GET("/host", s.permit(rbac.PermHostRead), s.hostStatus)
	authed.GET("/host/processes", s.permit(rbac.PermHostRead), s.hostProcesses)
	authed.GET("/scripts/runs", s.permit(rbac.PermScriptRead), s.listScriptRuns)
	authed.GET("/scripts/runs/:id", s.permit(rbac.PermScriptRead), s.getScriptRun)
	authed.POST("/scripts/run", s.permit(rbac.PermScriptWrite), s.runScript)
	authed.GET("/cron/preview", s.permit(rbac.PermCronRead), s.previewCron)
	authed.GET("/cron-jobs", s.permit(rbac.PermCronRead), s.listCronJobs)
	authed.POST("/cron-jobs", s.permit(rbac.PermCronWrite), s.createCronJob)
	authed.GET("/cron-jobs/:id", s.permit(rbac.PermCronRead), s.getCronJob)
	authed.PUT("/cron-jobs/:id", s.permit(rbac.PermCronWrite), s.updateCronJob)
	authed.DELETE("/cron-jobs/:id", s.permit(rbac.PermCronWrite), s.deleteCronJob)
	authed.POST("/cron-jobs/:id/enable", s.permit(rbac.PermCronWrite), s.enableCronJob)
	authed.POST("/cron-jobs/:id/disable", s.permit(rbac.PermCronWrite), s.disableCronJob)
	authed.POST("/cron-jobs/:id/run", s.permit(rbac.PermCronWrite), s.runCronJob)
	authed.GET("/cron-jobs/:id/runs", s.permit(rbac.PermCronRead), s.listCronRuns)
	authed.GET("/cron-runs", s.permit(rbac.PermCronRead), s.listCronRunsQuery)
	authed.GET("/logs", s.permit(rbac.PermLogRead), s.listLogs)
	authed.GET("/services", s.permit(rbac.PermServiceRead), s.listServices)
	authed.POST("/services/:name/start", s.permit(rbac.PermServiceWrite), s.startService)
	authed.POST("/services/:name/stop", s.permit(rbac.PermServiceWrite), s.stopService)
	authed.POST("/services/:name/restart", s.permit(rbac.PermServiceWrite), s.restartService)
	authed.GET("/services/:name/logs", s.permit(rbac.PermServiceRead), s.serviceLogs)
	authed.GET("/shell", s.permit(rbac.PermShellUse), s.shellInfo)
	authed.POST("/shell/ticket", s.permit(rbac.PermShellUse), s.shellTicket)
	s.engine.GET("/api/shell/ws", s.shellConnect)
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
	principal, err := s.tokens.Principal(claims)
	if err != nil {
		msg := "登录已过期"
		if err == rbac.ErrDisabled {
			msg = err.Error()
		}
		fail(c, http.StatusUnauthorized, msg)
		c.Abort()
		return
	}
	c.Set("username", principal.Username)
	c.Set("userID", principal.ID)
	c.Set(ctxPrincipal, principal)
	c.Next()
}

const ctxPrincipal = "principal"

func principalOf(c *gin.Context) *rbac.Principal {
	value, ok := c.Get(ctxPrincipal)
	if !ok {
		return nil
	}
	principal, _ := value.(*rbac.Principal)
	return principal
}

func (s *Server) permit(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !principalOf(c).Can(perm) {
			fail(c, http.StatusForbidden, "没有权限")
			c.Abort()
			return
		}
		c.Next()
	}
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
