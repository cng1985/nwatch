package settings

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责系统设置路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册系统设置路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/settings", middleware.Permit("settings.read"), ctl.get)
	authed.PUT("/settings", middleware.Permit("settings.write"), ctl.update)
	authed.PUT("/settings/mail", middleware.Permit("settings.write"), ctl.updateMail)
	authed.POST("/settings/mail/test", middleware.Permit("settings.write"), ctl.testMail)
}

func (ctl *Controller) get(c *gin.Context) {
	result, err := ctl.resource.Get(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) update(c *gin.Context) {
	var request SettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Update(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) updateMail(c *gin.Context) {
	var request MailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.UpdateMail(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) testMail(c *gin.Context) {
	var request MailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.TestMail(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
