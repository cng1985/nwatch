package dashboard

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责总览路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册总览路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/dashboard", middleware.Permit("dashboard.read"), ctl.summary)
}

func (ctl *Controller) summary(c *gin.Context) {
	var request DashboardRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Summary(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
