// Package routing 定义公共路由注册契约。
// server 不导入业务 Controller，通过组合根汇集注册器。
package routing

import "github.com/gin-gonic/gin"

// Registrar 由业务模块的 Controller 实现。
// public 是无需登录即可访问的路由组；authed 已装配认证中间件，
// 路由级权限由 Controller 通过 middleware.Permit 附加。
type Registrar interface {
	RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup)
}
