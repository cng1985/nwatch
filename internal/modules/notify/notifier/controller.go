package notifier

import (
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责通知渠道路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册通知渠道路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/notifiers", middleware.Permit("notifier.read"), ctl.list)
	authed.POST("/notifiers", middleware.Permit("notifier.write"), ctl.create)
	authed.PUT("/notifiers/:id", middleware.Permit("notifier.write"), ctl.update)
	authed.DELETE("/notifiers/:id", middleware.Permit("notifier.write"), ctl.remove)
	authed.POST("/notifiers/:id/test", middleware.Permit("notifier.write"), ctl.test)
	authed.GET("/notification-logs", middleware.Permit("notifier.read"), ctl.logs)
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
	var request NotifierRequest
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
	id, ok := parseID(c)
	if !ok {
		return
	}
	request := &NotifierRequest{ID: id}
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
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.Delete(c.Request.Context(), &NotifierTestRequest{ID: id})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) test(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.Test(c.Request.Context(), &NotifierTestRequest{ID: id})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) logs(c *gin.Context) {
	var search NotifyLogSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.SearchLogs(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.HandleError(c, errors.BadRequest("无效的编号"))
		return 0, false
	}
	return uint(n), true
}
