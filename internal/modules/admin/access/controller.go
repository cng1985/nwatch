package access

import (
	"strconv"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/middleware"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Controller 负责访问控制路由注册与协议绑定。
type Controller struct {
	resource *Resource
}

func NewController(resource *Resource) *Controller {
	return &Controller{resource: resource}
}

// RegisterRoutes 注册访问控制路由。登录是公开路由。
func (ctl *Controller) RegisterRoutes(public *gin.RouterGroup, authed *gin.RouterGroup) {
	public.POST("/auth/login", ctl.login)
	authed.GET("/auth/me", ctl.me)
	authed.PUT("/account/password", ctl.changePassword)
	authed.GET("/access/permissions", middleware.Permit("access.read"), ctl.permissions)
	authed.GET("/access/roles", middleware.Permit("access.read"), ctl.roles)
	authed.POST("/access/roles", middleware.Permit("access.write"), ctl.createRole)
	authed.PUT("/access/roles/:id", middleware.Permit("access.write"), ctl.updateRole)
	authed.DELETE("/access/roles/:id", middleware.Permit("access.write"), ctl.deleteRole)
	authed.GET("/access/users", middleware.Permit("access.read"), ctl.users)
	authed.POST("/access/users", middleware.Permit("access.write"), ctl.createUser)
	authed.PUT("/access/users/:id", middleware.Permit("access.write"), ctl.updateUser)
	authed.DELETE("/access/users/:id", middleware.Permit("access.write"), ctl.deleteUser)
}

func (ctl *Controller) login(c *gin.Context) {
	var request LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	request.IP = c.ClientIP()
	result, err := ctl.resource.Login(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) me(c *gin.Context) {
	result, err := ctl.resource.Me(c.Request.Context(), middleware.PrincipalOf(c))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) changePassword(c *gin.Context) {
	var request ChangePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.ChangePassword(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) permissions(c *gin.Context) {
	result, err := ctl.resource.ListPermissions(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) roles(c *gin.Context) {
	result, err := ctl.resource.ListRoles(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) createRole(c *gin.Context) {
	var request RoleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.CreateRole(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) updateRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	request := &RoleRequest{ID: id}
	if err := c.ShouldBindJSON(request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.UpdateRole(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) deleteRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.DeleteRole(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) users(c *gin.Context) {
	result, err := ctl.resource.ListUsers(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) createUser(c *gin.Context) {
	var request UserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.CreateUser(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) updateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	request := &UserRequest{ID: id}
	if err := c.ShouldBindJSON(request); err != nil {
		response.HandleError(c, errors.BadRequest("请求格式不正确"))
		return
	}
	result, err := ctl.resource.UpdateUser(c.Request.Context(), request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func (ctl *Controller) deleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctl.resource.DeleteUser(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.SuccessResponse(c, result)
}

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		response.HandleError(c, errors.BadRequest("无效的编号"))
		return 0, false
	}
	return uint(n), true
}
