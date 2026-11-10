package group

import (
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责分组路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册分组路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/groups", middleware.Permit("group.read"), ctl.list)
	authed.POST("/groups", middleware.Permit("group.write"), ctl.create)
	authed.PUT("/groups/:id", middleware.Permit("group.write"), ctl.update)
	authed.DELETE("/groups/:id", middleware.Permit("group.write"), ctl.remove)
}

func (ctl *Controller) list(c *gin.Context) {
	result, err := ctl.resource.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) create(c *gin.Context) {
	var request GroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Create(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) update(c *gin.Context) {
	request, ok := ctl.idRequest(c)
	if !ok {
		return
	}
	if err := c.ShouldBindJSON(request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Update(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) remove(c *gin.Context) {
	request, ok := ctl.idRequest(c)
	if !ok {
		return
	}
	result, err := ctl.resource.Delete(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) idRequest(c *gin.Context) (*GroupRequest, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.HandleError(c, errors.BadRequest("无效的编号"))
		return nil, false
	}
	return &GroupRequest{ID: uint(n)}, true
}
