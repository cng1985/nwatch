package monitor

import (
	"io"
	"net/http"
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责路由注册、协议绑定与统一输出，不承担数据库操作与业务编排。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// maxUploadExcel 是单次导入文件的上限。
const maxUploadExcel = 8 << 20

// maxExportRows 是单次导出上限。
const maxExportRows = 10000

// RegisterRoutes 注册监控项相关路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/monitors", middleware.Permit("monitor.read"), ctl.list)
	authed.POST("/monitors", middleware.Permit("monitor.write"), ctl.create)
	authed.GET("/monitors/export", middleware.Permit("monitor.read"), ctl.export)
	authed.POST("/monitors/import", middleware.Permit("monitor.write"), ctl.importMonitors)
	authed.GET("/monitors/requests/export", middleware.Permit("monitor.read"), ctl.exportRequests)
	authed.POST("/monitors/requests/import", middleware.Permit("monitor.write"), ctl.importRequests)
	authed.GET("/monitors/:id", middleware.Permit("monitor.read"), ctl.one)
	authed.PUT("/monitors/:id", middleware.Permit("monitor.write"), ctl.update)
	authed.DELETE("/monitors/:id", middleware.Permit("monitor.write"), ctl.remove)
	authed.POST("/monitors/:id/enable", middleware.Permit("monitor.write"), ctl.enable)
	authed.POST("/monitors/:id/disable", middleware.Permit("monitor.write"), ctl.disable)
	authed.POST("/monitors/:id/check", middleware.Permit("monitor.write"), ctl.check)
	authed.GET("/monitors/:id/checks", middleware.Permit("monitor.read"), ctl.checks)
	authed.GET("/monitors/:id/metrics", middleware.Permit("monitor.read"), ctl.metrics)
	authed.GET("/monitors/:id/events", middleware.Permit("monitor.read"), ctl.events)
	authed.GET("/events", middleware.Permit("event.read"), ctl.allEvents)
	authed.GET("/certificates", middleware.Permit("cert.read"), ctl.certificates)
}

func (ctl *Controller) list(c *gin.Context) {
	var search MonitorSearch
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

func (ctl *Controller) create(c *gin.Context) {
	var request MonitorRequest
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

func (ctl *Controller) one(c *gin.Context) {
	request, ok := ctl.idRequest(c)
	if !ok {
		return
	}
	result, err := ctl.resource.One(c.Request.Context(), request)
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

func (ctl *Controller) enable(c *gin.Context)  { ctl.setEnabled(c, true) }
func (ctl *Controller) disable(c *gin.Context) { ctl.setEnabled(c, false) }

func (ctl *Controller) setEnabled(c *gin.Context, enabled bool) {
	request, ok := ctl.idRequest(c)
	if !ok {
		return
	}
	if enabled {
		result, err := ctl.resource.Enable(c.Request.Context(), request)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.SuccessResponse(c, result)
		return
	}
	result, err := ctl.resource.Disable(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) check(c *gin.Context) {
	request, ok := ctl.idRequest(c)
	if !ok {
		return
	}
	result, err := ctl.resource.Check(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) checks(c *gin.Context) {
	search := &MonitorCheckSearch{}
	if err := c.ShouldBindQuery(search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	search.ID = id
	page, err := ctl.resource.SearchChecks(c.Request.Context(), search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func (ctl *Controller) metrics(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	search := &MonitorMetricSearch{ID: id}
	if err := c.ShouldBindQuery(search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.MetricSummary(c.Request.Context(), search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) events(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	search := &MonitorEventSearch{MonitorID: strconv.FormatUint(uint64(id), 10)}
	if err := c.ShouldBindQuery(search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.SearchEvents(c.Request.Context(), search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func (ctl *Controller) allEvents(c *gin.Context) {
	var search MonitorEventSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	page, err := ctl.resource.SearchEvents(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessPage(c, page)
}

func (ctl *Controller) certificates(c *gin.Context) {
	result, err := ctl.resource.ListCertificates(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) export(c *gin.Context) {
	var request MonitorExportRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.ExportMonitors(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	streamWorkbook(c, result.Data.Filename, result.Data.Data)
}

func (ctl *Controller) exportRequests(c *gin.Context) {
	var request RequestExportRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.ExportRequests(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	streamWorkbook(c, result.Data.Filename, result.Data.Data)
}

func (ctl *Controller) importMonitors(c *gin.Context) {
	data, ok := readExcelUpload(c)
	if !ok {
		return
	}
	request := &MonitorImportRequest{Data: data}
	result, err := ctl.resource.ImportMonitors(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) importRequests(c *gin.Context) {
	data, ok := readExcelUpload(c)
	if !ok {
		return
	}
	request := &MonitorImportRequest{Data: data}
	result, err := ctl.resource.ImportRequests(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) idRequest(c *gin.Context) (*MonitorRequest, bool) {
	id, ok := parseID(c)
	if !ok {
		return nil, false
	}
	return &MonitorRequest{ID: id}, true
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.HandleError(c, errors.BadRequest("无效的编号"))
		return 0, false
	}
	return uint(n), true
}

func streamWorkbook(c *gin.Context, filename string, data []byte) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// readExcelUpload 读取上传的 xlsx 文件并校验大小。
func readExcelUpload(c *gin.Context) ([]byte, bool) {
	file, err := c.FormFile("file")
	if err != nil {
		response.HandleError(c, errors.BadRequest("请选择 xlsx 文件"))
		return nil, false
	}
	if file.Size > maxUploadExcel {
		response.HandleError(c, errors.BadRequest("文件不能超过 8MB"))
		return nil, false
	}
	src, err := file.Open()
	if err != nil {
		response.HandleError(c, errors.BadRequest("请选择 xlsx 文件"))
		return nil, false
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxUploadExcel+1))
	if err != nil {
		response.HandleError(c, errors.BadRequest("无法读取文件"))
		return nil, false
	}
	if len(data) > maxUploadExcel {
		response.HandleError(c, errors.BadRequest("文件不能超过 8MB"))
		return nil, false
	}
	return data, true
}
