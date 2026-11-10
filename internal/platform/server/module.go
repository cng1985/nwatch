package server

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Module 提供唯一 Gin 引擎与 HTTP 生命周期。
// 引擎同时以 *gin.Engine 暴露，供静态资源等装配方注册处理器。
var Module = fx.Module("server",
	fx.Provide(NewServer),
	fx.Provide(func(s *Server) *gin.Engine { return s.Engine() }),
	fx.Invoke(func(*Server) {}),
)
