package service

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责系统服务路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册系统服务路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/services", middleware.Permit("service.read"), ctl.list)
	authed.POST("/services/:name/start", middleware.Permit("service.write"), ctl.start)
	authed.POST("/services/:name/stop", middleware.Permit("service.write"), ctl.stop)
	authed.POST("/services/:name/restart", middleware.Permit("service.write"), ctl.restart)
	authed.GET("/services/:name/logs", middleware.Permit("service.read"), ctl.logs)
}

func (ctl *Controller) list(c *gin.Context) {
	var request ServiceListRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.List(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) start(c *gin.Context)   { ctl.action(c, "start") }
func (ctl *Controller) stop(c *gin.Context)    { ctl.action(c, "stop") }
func (ctl *Controller) restart(c *gin.Context) { ctl.action(c, "restart") }

func (ctl *Controller) action(c *gin.Context, action string) {
	name := c.Param("name")
	result, err := ctl.resource.Action(c.Request.Context(), &ServiceActionRequest{Name: name, Action: action})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) logs(c *gin.Context) {
	var request ServiceLogsRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	request.Name = c.Param("name")
	result, err := ctl.resource.Logs(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
