package backup

import (
	"os"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责备份与导入导出路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册备份与导入导出路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/backup", middleware.Permit("settings.write"), ctl.download)
	authed.POST("/backup", middleware.Permit("settings.write"), ctl.create)
	authed.GET("/export", middleware.Permit("settings.write"), ctl.export)
	authed.POST("/import", middleware.Permit("settings.write"), ctl.importConfig)
}

func (ctl *Controller) download(c *gin.Context) {
	path, name, err := ctl.resource.DownloadBackup(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	defer os.Remove(path)
	c.FileAttachment(path, name)
}

func (ctl *Controller) create(c *gin.Context) {
	result, err := ctl.resource.CreateBackup(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) export(c *gin.Context) {
	var request ExportRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Export(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) importConfig(c *gin.Context) {
	var request ImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("导入内容不正确"))
		return
	}
	result, err := ctl.resource.Import(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
