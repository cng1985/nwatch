package kube

import (
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责 Kubernetes 路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册 Kubernetes 路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	authed.GET("/kubernetes/namespaces", middleware.Permit("kube.read"), ctl.namespaces)
	authed.GET("/kubernetes/resources", middleware.Permit("kube.read"), ctl.list)
	authed.POST("/kubernetes/pods/:namespace/:name/delete", middleware.Permit("kube.write"), ctl.deletePod)
	authed.POST("/kubernetes/workloads/:kind/:namespace/:name/restart", middleware.Permit("kube.write"), ctl.restart)
	authed.POST("/kubernetes/workloads/:kind/:namespace/:name/scale", middleware.Permit("kube.write"), ctl.scale)
	authed.GET("/kubernetes/pods/:namespace/:name/logs", middleware.Permit("kube.read"), ctl.podLogs)
}

func (ctl *Controller) namespaces(c *gin.Context) {
	result, err := ctl.resource.Namespaces(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) list(c *gin.Context) {
	var search KubeResourceSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.List(c.Request.Context(), &search)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) restart(c *gin.Context) {
	request := &KubeWorkloadRequest{Kind: c.Param("kind"), Namespace: c.Param("namespace"), Name: c.Param("name")}
	result, err := ctl.resource.Restart(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) scale(c *gin.Context) {
	request := &KubeScaleRequest{Kind: c.Param("kind"), Namespace: c.Param("namespace"), Name: c.Param("name")}
	if err := c.ShouldBindJSON(request); err != nil || request.Replicas == nil {
		response.HandleError(c, errors.BadRequest("副本数不合法"))
		return
	}
	result, err := ctl.resource.Scale(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) deletePod(c *gin.Context) {
	request := &KubePodRequest{Namespace: c.Param("namespace"), Name: c.Param("name")}
	result, err := ctl.resource.DeletePod(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) podLogs(c *gin.Context) {
	var request KubePodLogsRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	request.Namespace = c.Param("namespace")
	request.Name = c.Param("name")
	result, err := ctl.resource.PodLogs(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}
