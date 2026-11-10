package logview

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责运行日志路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册运行日志路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/logs", middleware.Permit("log.read"), ctl.list)
}

func (ctl *Controller) list(c *gin.Context) {
	var search LogSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.Search(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}
