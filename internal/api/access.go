package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/cng1985/nwatch/internal/rbac"
	"github.com/gin-gonic/gin"
)

func (s *Server) listPermissions(c *gin.Context) {
	ok(c, rbac.Catalog())
}

func (s *Server) listRoles(c *gin.Context) {
	roles, err := s.tokens.Users().ListRoles()
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询角色失败")
		return
	}
	ok(c, roles)
}

func (s *Server) createRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	role, err := s.tokens.Users().CreateRole(req.input())
	if err != nil {
		accessFail(c, err)
		return
	}
	slog.Info("已创建角色", "role", role.Name, "operator", c.GetString("username"))
	ok(c, role)
}

func (s *Server) updateRole(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	role, err := s.tokens.Users().UpdateRole(id, req.input())
	if err != nil {
		accessFail(c, err)
		return
	}
	slog.Info("已更新角色", "role", role.Name, "operator", c.GetString("username"))
	ok(c, role)
}

func (s *Server) deleteRole(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if err := s.tokens.Users().DeleteRole(id); err != nil {
		accessFail(c, err)
		return
	}
	slog.Info("已删除角色", "id", id, "operator", c.GetString("username"))
	ok(c, gin.H{"ok": true})
}

func (s *Server) listUsers(c *gin.Context) {
	users, err := s.tokens.Users().ListUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	ok(c, users)
}

func (s *Server) createUser(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	user, err := s.tokens.Users().CreateUser(req.input())
	if err != nil {
		accessFail(c, err)
		return
	}
	slog.Info("已创建用户", "username", user.Username, "operator", c.GetString("username"))
	ok(c, user)
}

func (s *Server) updateUser(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	user, err := s.tokens.Users().UpdateUser(c.GetUint("userID"), id, req.input())
	if err != nil {
		accessFail(c, err)
		return
	}
	if req.Password != "" {
		s.syncBootstrapPassword(user.Username, req.Password)
	}
	slog.Info("已更新用户", "username", user.Username, "operator", c.GetString("username"))
	ok(c, user)
}

func (s *Server) deleteUser(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if err := s.tokens.Users().DeleteUser(c.GetUint("userID"), id); err != nil {
		accessFail(c, err)
		return
	}
	slog.Info("已删除用户", "id", id, "operator", c.GetString("username"))
	ok(c, gin.H{"ok": true})
}

type roleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r roleRequest) input() rbac.RoleInput {
	return rbac.RoleInput{Name: r.Name, Description: r.Description, Permissions: r.Permissions}
}

type userRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Enabled     *bool  `json:"enabled"`
	RoleIDs     []uint `json:"roleIds"`
}

func (r userRequest) input() rbac.UserInput {
	return rbac.UserInput{
		Username: r.Username, DisplayName: r.DisplayName, Password: r.Password,
		Enabled: r.Enabled, RoleIDs: r.RoleIDs,
	}
}

func accessFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, rbac.ErrNotFound), errors.Is(err, rbac.ErrRoleMissing):
		fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, rbac.ErrUsernameTaken),
		errors.Is(err, rbac.ErrRoleNameTaken),
		errors.Is(err, rbac.ErrBuiltinDelete),
		errors.Is(err, rbac.ErrBuiltinRename),
		errors.Is(err, rbac.ErrRoleInUse),
		errors.Is(err, rbac.ErrLastAdmin),
		errors.Is(err, rbac.ErrBadUsername),
		errors.Is(err, rbac.ErrBadPassword),
		errors.Is(err, rbac.ErrNoRole),
		errors.Is(err, rbac.ErrBadPerm),
		errors.Is(err, rbac.ErrNoPerm),
		errors.Is(err, rbac.ErrBadName),
		errors.Is(err, rbac.ErrDisableSelf),
		errors.Is(err, rbac.ErrDeleteSelf):
		fail(c, http.StatusBadRequest, err.Error())
	default:
		slog.Error("访问控制操作失败", "err", err.Error())
		fail(c, http.StatusInternalServerError, "保存失败")
	}
}
