package docker

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责 Docker 路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册 Docker 路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/docker/containers", middleware.Permit("docker.read"), ctl.containers)
	authed.GET("/docker/images", middleware.Permit("docker.read"), ctl.images)
	authed.POST("/docker/containers/:name/start", middleware.Permit("docker.write"), ctl.start)
	authed.POST("/docker/containers/:name/stop", middleware.Permit("docker.write"), ctl.stop)
	authed.POST("/docker/containers/:name/restart", middleware.Permit("docker.write"), ctl.restart)
	authed.GET("/docker/containers/:name/logs", middleware.Permit("docker.read"), ctl.logs)
}

func (ctl *Controller) containers(c *gin.Context) {
	var search DockerContainerSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Containers(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) images(c *gin.Context) {
	var search DockerImageSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Images(c.Request.Context(), &search)
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
	result, err := ctl.resource.Action(c.Request.Context(), &DockerActionRequest{Name: c.Param("name"), Action: action})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) logs(c *gin.Context) {
	var request DockerLogsRequest
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
