// Package server 提供唯一 Gin 引擎、中间件装配与 HTTP 生命周期。
// server 不导入业务 Controller，通过组合根汇集路由注册器。
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// HealthCheck 是参与就绪检查的组件报告。
type HealthCheck struct {
	Name string
	Up   bool
}

// Reporter 由后台组件实现，向健康端点报告自身状态。
type Reporter interface {
	HealthReport() HealthCheck
}

type Params struct {
	fx.In
	Cfg        *config.Config
	DB         *gorm.DB
	Tokens     *auth.Service
	Registrars []routing.Registrar `group:"registrars"`
	Reporters  []Reporter          `group:"health"`
}

// Server 装配引擎并在生命周期内监听、排空与关闭。
type Server struct {
	cfg        *config.Config
	db         *gorm.DB
	tokens     *auth.Service
	registrars []routing.Registrar
	reporters  []Reporter
	engine     *gin.Engine
	http       *http.Server
	started    time.Time
}

// NewServer 创建引擎、装配中间件并注册健康端点与业务路由。
func NewServer(p Params, lc fx.Lifecycle) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		cfg:        p.Cfg,
		db:         p.DB,
		tokens:     p.Tokens,
		registrars: p.Registrars,
		reporters:  p.Reporters,
		engine:     gin.New(),
		started:    time.Now(),
	}
	// 信任代理为空时不信任任何代理，ClientIP 使用真实连接地址。
	_ = s.engine.SetTrustedProxies(p.Cfg.Server.TrustedProxies)
	s.engine.Use(middleware.Recovery(), middleware.RequestID(), middleware.MaxBody(maxBodyBytes()))
	s.routes()
	s.http = &http.Server{
		Addr:              p.Cfg.Addr(),
		Handler:           s.engine,
		ReadHeaderTimeout: p.Cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       p.Cfg.Server.IdleTimeout,
		MaxHeaderBytes:    p.Cfg.Server.MaxHeaderBytes,
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				return s.start()
			},
			OnStop: func(ctx context.Context) error {
				return s.shutdown(ctx)
			},
		})
	}
	return s
}

// maxBodyBytes 是全局请求体上限。项目含 Excel 导入上传（单文件另限 8MB），
// 因此默认比规范普通 JSON 上限更宽松（见 docs/decisions.md）。
func maxBodyBytes() int64 {
	return 1 << 24 // 16MB
}

// Engine 暴露引擎给静态资源注册等装配方。
func (s *Server) Engine() *gin.Engine { return s.engine }

func (s *Server) routes() {
	s.engine.GET("/health/live", s.healthLive)
	s.engine.GET("/health/ready", s.healthReady)
	s.engine.GET("/health", s.healthDetail)
	s.engine.GET("/version", s.version)
	api := s.engine.Group("/api")
	public := api.Group("")
	authed := api.Group("")
	authed.Use(middleware.RequireAuth(s.tokens))
	for _, registrar := range s.registrars {
		registrar.RegisterRoutes(public, authed)
	}
}

func (s *Server) start() error {
	// 先创建监听器，端口占用同步报错，再启动 Serve。
	listener, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", s.http.Addr, err)
	}
	slog.Info("HTTP 服务已启动", "addr", listener.Addr().String())
	go func() {
		if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常", "err", err.Error())
		}
	}()
	return nil
}

func (s *Server) shutdown(ctx context.Context) error {
	slog.Info("停止接收 HTTP 请求")
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.Server.ShutdownTimeout)
	defer cancel()
	return s.http.Shutdown(shutdownCtx)
}
