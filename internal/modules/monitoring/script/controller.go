package script

import (
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责脚本执行路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册脚本执行路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/scripts/runs", middleware.Permit("script.read"), ctl.list)
	authed.GET("/scripts/runs/:id", middleware.Permit("script.read"), ctl.one)
	authed.POST("/scripts/run", middleware.Permit("script.write"), ctl.run)
}

func (ctl *Controller) run(c *gin.Context) {
	var request ScriptRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Run(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) list(c *gin.Context) {
	var search ScriptRunSearch
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

func (ctl *Controller) one(c *gin.Context) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.HandleError(c, errors.BadRequest("无效的编号"))
		return
	}
	result, err := ctl.resource.One(c.Request.Context(), &ScriptRunOneRequest{ID: uint(n)})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
