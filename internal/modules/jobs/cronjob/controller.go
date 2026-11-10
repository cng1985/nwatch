package cronjob

import (
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责定时任务路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册定时任务路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/cron/preview", middleware.Permit("cron.read"), ctl.preview)
	authed.GET("/cron-jobs", middleware.Permit("cron.read"), ctl.list)
	authed.POST("/cron-jobs", middleware.Permit("cron.write"), ctl.create)
	authed.GET("/cron-jobs/:id", middleware.Permit("cron.read"), ctl.one)
	authed.PUT("/cron-jobs/:id", middleware.Permit("cron.write"), ctl.update)
	authed.DELETE("/cron-jobs/:id", middleware.Permit("cron.write"), ctl.remove)
	authed.POST("/cron-jobs/:id/enable", middleware.Permit("cron.write"), ctl.enable)
	authed.POST("/cron-jobs/:id/disable", middleware.Permit("cron.write"), ctl.disable)
	authed.POST("/cron-jobs/:id/run", middleware.Permit("cron.write"), ctl.run)
	authed.GET("/cron-jobs/:id/runs", middleware.Permit("cron.read"), ctl.runs)
	authed.GET("/cron-runs", middleware.Permit("cron.read"), ctl.allRuns)
}

func (ctl *Controller) preview(c *gin.Context) {
	var request CronPreviewRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Preview(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) list(c *gin.Context) {
	var search CronJobSearch
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
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.One(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) create(c *gin.Context) {
	var request CronJobRequest
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
	var request CronJobRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.Update(c.Request.Context(), id, &request)
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
	result, err := ctl.resource.Delete(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) enable(c *gin.Context)  { ctl.setEnabled(c, true) }
func (ctl *Controller) disable(c *gin.Context) { ctl.setEnabled(c, false) }

func (ctl *Controller) setEnabled(c *gin.Context, enabled bool) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var (
		result interface{}
		err    error
	)
	if enabled {
		r, e := ctl.resource.Enable(c.Request.Context(), id)
		result, err = r, e
	} else {
		r, e := ctl.resource.Disable(c.Request.Context(), id)
		result, err = r, e
	}
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(200, result)
}

func (ctl *Controller) run(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.Run(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) runs(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var search CronRunSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	search.JobID = strconv.FormatUint(uint64(id), 10)
	page, err := ctl.resource.SearchRuns(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func (ctl *Controller) allRuns(c *gin.Context) {
	var search CronRunSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.SearchRuns(c.Request.Context(), &search)
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
